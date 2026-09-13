# E2E TCP Probes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace guest ICMP in `scripts/e2e-network.sh` with curl `--connect-only` TCP checks and ignore host ssh_config in `ssh_guest`.

**Architecture:** One bash script. SSH as `agent`, then curl TCP-connect-only. Isolation = A cannot connect to B:22. Egress = A can connect to 1.1.1.1:443.

**Tech Stack:** bash, OpenSSH client, guest curl (already in rootfs).

## Global Constraints

- `ssh_guest` must pass `-F /dev/null` and `-o BatchMode=yes` and `-o ConnectTimeout=5`.
- Keep existing `-i`, `IdentitiesOnly=yes`, `StrictHostKeyChecking=no`, `UserKnownHostsFile=/dev/null`.
- Guest probe command: `curl --connect-only --connect-timeout 2 --max-time 3 -sS -o /dev/null "http://HOST:PORT/"`.
- Isolation: A → B port 22 must fail; success is `die`.
- Egress: A → `1.1.1.1` port 443 must succeed.
- No `ping` calls in the script; remove `ping` from preflight `command -v` list.
- Do not change Go, firewall, rootfs, or `README.md`.
- Do not run `sudo scripts/e2e-network.sh` in this task (needs live KVM + operator).
- `docs/` is globally gitignored — do not `git add -f` spec/plan.
- Commit only `scripts/e2e-network.sh` (include the file’s existing uncommitted launch/mem/`set -u` fixes; they are already in the working tree).

---

### Task 1: TCP probes and ssh_guest flags

**Files:**
- Modify: `scripts/e2e-network.sh`

**Interfaces:**
- Consumes: `ssh_guest id [remote command...]`, `session_field id guest_ip`
- Produces: `guest_tcp id host port` — runs curl `--connect-only` on that guest via `ssh_guest`; returns curl’s exit status.

- [ ] **Step 1: Confirm current ping-based assertions exist**

```bash
grep -n ping scripts/e2e-network.sh
```

Expected: `preflight` lists `ping`; `assert_isolation` calls `ping -c1`.

- [ ] **Step 2: There is no Go test. Treat `bash -n` plus the absence of `ping` as the check.**

No new test file. Do not add a Go test for a bash script.

- [ ] **Step 3: Implement**

In `preflight`, change:

```bash
	for cmd in python3 ip bridge ssh ping; do
```

to:

```bash
	for cmd in python3 ip bridge ssh; do
```

Replace `ssh_guest` with:

```bash
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
```

Add after `ssh_guest`:

```bash
guest_tcp() {
	local id="$1" host="$2" port="$3"
	ssh_guest "$id" curl --connect-only --connect-timeout 2 --max-time 3 -sS -o /dev/null "http://${host}:${port}/"
}
```

Replace `assert_isolation` body with:

```bash
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
```

Do not revert the existing uncommitted launch/vcpu/`LAST_SESSION_ID`/`set -u` array fixes in this file.

- [ ] **Step 4: Verify**

```bash
bash -n scripts/e2e-network.sh
grep -n ping scripts/e2e-network.sh && echo 'FAIL: ping still present' && exit 1 || true
grep -n 'BatchMode=yes' scripts/e2e-network.sh
grep -n 'connect-only' scripts/e2e-network.sh
```

Expected: `bash -n` exit 0; `grep ping` finds nothing (exit 1 from grep is success here); BatchMode and connect-only present.

- [ ] **Step 5: Commit**

```bash
git add scripts/e2e-network.sh
git commit -m "$(cat <<'EOF'
Probe e2e isolation and egress with guest TCP, not ping.

EOF
)"
```

Do not `git add README.md` or any other file.
