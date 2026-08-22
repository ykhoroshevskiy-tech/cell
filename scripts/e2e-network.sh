#!/usr/bin/env bash
# Network lifecycle smoke test for the built cell binary.
# Usage: sudo scripts/e2e-network.sh [path/to/cell]
# Requires: prior `sudo cell bootstrap`, root, /dev/kvm.
set -euo pipefail

CELL_BIN="${1:-./cell}"
TMP=""
REPO_A=""
REPO_B=""
REPO_C=""
LAST_SESSION_ID=""
declare -a E2E_SESSIONS=()

log() { printf 'e2e: %s\n' "$*"; }
die() { printf 'e2e: ERROR: %s\n' "$*" >&2; exit 1; }

cell() {
	"$CELL_BIN" "$@"
}

preflight() {
	[[ "$(id -u)" -eq 0 ]] || die "must run as root (sudo)"
	[[ -e /dev/kvm ]] || die "/dev/kvm missing"
	[[ -x "$CELL_BIN" ]] || die "cell binary not executable: $CELL_BIN"

	local k="${CELL_KERNEL_PATH:-/var/lib/cell/images/vmlinux}"
	local r="${CELL_ROOTFS_PATH:-/var/lib/cell/images/rootfs.ext4}"
	local f="${CELL_FIRECRACKER_BIN:-/var/lib/cell/images/bin/firecracker}"
	[[ -f "$k" && -f "$r" && -x "$f" ]] || die "bootstrap artifacts missing; run: sudo cell bootstrap"

	for cmd in python3 ip bridge ssh; do
		command -v "$cmd" >/dev/null || die "missing host tool: $cmd"
	done
}

setup_env() {
	TMP="$(mktemp -d /tmp/cell-e2e-XXXXXX)"
	export CELL_DATA_DIR="$TMP/data"
	export CELL_IMAGES_DIR="${CELL_IMAGES_DIR:-/var/lib/cell/images}"
	export CELL_KERNEL_PATH="${CELL_KERNEL_PATH:-/var/lib/cell/images/vmlinux}"
	export CELL_ROOTFS_PATH="${CELL_ROOTFS_PATH:-/var/lib/cell/images/rootfs.ext4}"
	export CELL_FIRECRACKER_BIN="${CELL_FIRECRACKER_BIN:-/var/lib/cell/images/bin/firecracker}"
	export CELL_VCPU_COUNT=2
	export CELL_MEM_SIZE_MIB=4096
	export CELL_PROJECT_DISK_SIZE_MB=512
	export CELL_AUTO_PULL=false
	mkdir -p "$CELL_DATA_DIR"

	REPO_A="$TMP/repo-a"
	REPO_B="$TMP/repo-b"
	REPO_C="$TMP/repo-c"
	for d in "$REPO_A" "$REPO_B" "$REPO_C"; do
		mkdir -p "$d"
		echo "e2e" >"$d/README"
	done
	log "CELL_DATA_DIR=$CELL_DATA_DIR"
	log "CELL_IMAGES_DIR=$CELL_IMAGES_DIR"
	log "kernel=$CELL_KERNEL_PATH"
	log "vcpu=$CELL_VCPU_COUNT mem=${CELL_MEM_SIZE_MIB}MiB disk=${CELL_PROJECT_DISK_SIZE_MB}MB"
}

session_field() {
	local id="$1" field="$2"
	cell ps --json --all | python3 -c "
import json, sys
want, field = sys.argv[1], sys.argv[2]
for s in json.load(sys.stdin):
    if s.get('session_id') == want:
        print(s.get(field, ''))
        break
" "$id" "$field"
}

launch_session() {
	local repo="$1"
	local id
	LAST_SESSION_ID=""
	log "---- cell -v launch --no-attach --repo $repo ----"
	# Do not wrap in $() or script: this function used to run under command
	# substitution, which hid all cell output until exit.
	if ! "$CELL_BIN" -v launch --no-attach --repo "$repo"; then
		die "launch failed for $repo"
	fi
	id="$(cell ps --json --all | python3 -c "
import json, sys
rows = json.load(sys.stdin)
rows.sort(key=lambda s: s.get('created_at', ''))
print(rows[-1]['session_id'] if rows else '')
")"
	[[ -n "$id" ]] || die "could not parse session id after launch"
	log "launched session $id"
	E2E_SESSIONS+=("$id")
	LAST_SESSION_ID="$id"
}

stop_rm_session() {
	local id="$1"
	log "stop --session $id"
	cell stop --session "$id" || true
	log "rm --session $id"
	cell rm --session "$id" || true
	local kept=() s
	for s in ${E2E_SESSIONS[@]+"${E2E_SESSIONS[@]}"}; do
		[[ "$s" == "$id" ]] || kept+=("$s")
	done
	E2E_SESSIONS=()
	for s in ${kept[@]+"${kept[@]}"}; do
		E2E_SESSIONS+=("$s")
	done
}

ssh_guest() {
	local id="$1"
	shift
	local key="$CELL_DATA_DIR/session-data/$id/id_ed25519"
	local ip
	ip="$(session_field "$id" guest_ip)"
	[[ -n "$ip" ]] || die "no guest_ip for session $id"
	ssh -F /dev/null -i "$key" \
		-o BatchMode=yes \
		-o ConnectTimeout=5 \
		-o StrictHostKeyChecking=no \
		-o UserKnownHostsFile=/dev/null \
		-o IdentitiesOnly=yes \
		"agent@$ip" "$@"
}

guest_tcp() {
	local id="$1" host="$2" port="$3"
	ssh_guest "$id" curl --connect-only --connect-timeout 2 --max-time 3 -sS -o /dev/null "http://${host}:${port}/"
}

assert_bridge() {
	ip link show cell0 >/dev/null 2>&1 || die "cell0 missing"
	ip -4 addr show dev cell0 | grep -q '172.16.107.1/24' || die "cell0 missing 172.16.107.1/24"
	local routes
	routes="$(ip route show 172.16.107.0/24 dev cell0 2>/dev/null || true)"
	[[ -n "$routes" ]] || die "no 172.16.107.0/24 route via cell0"
}

assert_tap() {
	local tap="$1"
	ip link show "$tap" >/dev/null 2>&1 || die "tap missing: $tap"
	if ip -4 addr show dev "$tap" 2>/dev/null | grep -q 'inet '; then
		die "tap $tap has an IPv4 address (should be none)"
	fi
	ip link show "$tap" | grep -q 'master cell0' || die "tap $tap not enslaved to cell0"
	bridge -d link show dev "$tap" 2>/dev/null | grep -q 'isolated on' || die "tap $tap not isolated"
}

assert_two_vms() {
	local id_a="$1" id_b="$2"
	local ip_a ip_b tap_a tap_b
	ip_a="$(session_field "$id_a" guest_ip)"
	ip_b="$(session_field "$id_b" guest_ip)"
	tap_a="$(session_field "$id_a" tap_name)"
	tap_b="$(session_field "$id_b" tap_name)"
	[[ -n "$ip_a" && -n "$ip_b" ]] || die "missing guest IPs"
	[[ "$ip_a" != "$ip_b" ]] || die "guest IPs must differ"
	guest_ip_ok() {
		python3 -c "
import ipaddress, sys
ip = ipaddress.ip_address(sys.argv[1])
ok = ip in ipaddress.ip_network('172.16.107.0/24') and str(ip) not in ('172.16.107.0', '172.16.107.1', '172.16.107.255')
sys.exit(0 if ok else 1)
" "$1"
	}
	guest_ip_ok "$ip_a" || die "bad guest IP: $ip_a"
	guest_ip_ok "$ip_b" || die "bad guest IP: $ip_b"
	[[ -n "$tap_a" && -n "$tap_b" ]] || die "missing tap names"
	assert_bridge
	assert_tap "$tap_a"
	assert_tap "$tap_b"
	log "two VMs ok: A=$id_a ip=$ip_a tap=$tap_a  B=$id_b ip=$ip_b tap=$tap_b"
}

assert_isolation() {
	local id_a="$1" id_b="$2"
	local ip_b
	ip_b="$(session_field "$id_b" guest_ip)"
	if guest_tcp "$id_a" "$ip_b" 22; then
		die "guest A TCP to B:22 should fail (isolation broken)"
	fi
	log "guest-to-guest TCP :22 blocked"
	guest_tcp "$id_a" 1.1.1.1 443 || die "guest A cannot TCP 1.1.1.1:443"
	log "guest internet egress ok"
}

maybe_teardown_host_net() {
	local running ctaps
	running="$(CELL_DATA_DIR=/var/lib/cell "$CELL_BIN" ps --json --all 2>/dev/null | python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
except Exception:
    sys.exit(1)
sys.exit(0 if not any(s.get('vm_running') for s in data) else 1)
" && echo no || echo yes)"
	ctaps="$(ip -o link show 2>/dev/null | awk -F': ' '{print $2}' | grep '^ctap-' || true)"
	if [[ "$running" == yes || -n "$ctaps" ]]; then
		log "keeping cell0/CELL_* (other cell VMs or TAPs present)"
		return
	fi
	log "tearing down cell0 and CELL_* chains"
	ip link del cell0 2>/dev/null || true
	iptables -D INPUT -j CELL_INPUT 2>/dev/null || true
	iptables -D FORWARD -j CELL_FORWARD 2>/dev/null || true
	iptables -t nat -D POSTROUTING -j CELL_NAT 2>/dev/null || true
	iptables -F CELL_INPUT 2>/dev/null || true
	iptables -X CELL_INPUT 2>/dev/null || true
	iptables -F CELL_FORWARD 2>/dev/null || true
	iptables -X CELL_FORWARD 2>/dev/null || true
	iptables -t nat -F CELL_NAT 2>/dev/null || true
	iptables -t nat -X CELL_NAT 2>/dev/null || true
}

cleanup() {
	local id
	for id in ${E2E_SESSIONS[@]+"${E2E_SESSIONS[@]}"}; do
		stop_rm_session "$id" || true
	done
	[[ -n "$TMP" && -d "$TMP" ]] && rm -rf "$TMP"
	maybe_teardown_host_net || true
}

main() {
	trap cleanup EXIT
	preflight
	setup_env

	log "launching VM A (live cell -v output follows)"
	local id_a id_b id_c ip_b tap_a
	launch_session "$REPO_A"
	id_a="$LAST_SESSION_ID"
	log "launching VM B (live cell -v output follows)"
	launch_session "$REPO_B"
	id_b="$LAST_SESSION_ID"

	assert_two_vms "$id_a" "$id_b"
	assert_isolation "$id_a" "$id_b"

	ip_b="$(session_field "$id_b" guest_ip)"
	tap_a="$(session_field "$id_a" tap_name)"

	log "injecting orphan tap ctap-deadbeef"
	ip tuntap add dev ctap-deadbeef mode tap 2>/dev/null || true
	ip link set ctap-deadbeef up 2>/dev/null || true

	log "deleting live TAP for A ($tap_a) to simulate stale state"
	ip link del "$tap_a"

	log "stop+rm B to trigger reconcile"
	stop_rm_session "$id_b"

	ip link show "$tap_a" >/dev/null 2>&1 || die "TAP $tap_a not recreated after reconcile"
	if ip link show ctap-deadbeef >/dev/null 2>&1; then
		die "orphan ctap-deadbeef not removed"
	fi
	log "stale TAP cleanup ok"

	log "launching VM C (should reuse B's IP $ip_b)"
	launch_session "$REPO_C"
	id_c="$LAST_SESSION_ID"
	local ip_c
	ip_c="$(session_field "$id_c" guest_ip)"
	[[ "$ip_c" == "$ip_b" ]] || die "lease reuse failed: C=$ip_c want $ip_b"
	log "lease reuse ok: C got $ip_c"

	log "stopping remaining sessions"
	stop_rm_session "$id_a"
	stop_rm_session "$id_c"

	# Clear trap list so cleanup doesn't double-stop; cleanup still runs rm -rf + maybe teardown
	E2E_SESSIONS=()
	log "PASS"
}

main "$@"
