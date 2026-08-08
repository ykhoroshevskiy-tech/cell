#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CELL="${ROOT}/cell"
REPO="$(cd "${ROOT}/../../.." && pwd)"
AB="/tmp/cell-abtest"
IMAGES="${AB}/images"

export CELL_DATA_DIR="${AB}"
export CELL_IMAGES_DIR="${IMAGES}"

log() { echo "==> $*"; }

serial_head() {
  local sid="$1"
  local f="${AB}/session-data/${sid}/serial.log"
  if [[ ! -f "$f" ]]; then
    echo "(no serial.log)"
    return
  fi
  head -30 "$f"
  if grep -q 'Linux version' "$f"; then
    echo "--- KERNEL SERIAL: YES ---"
  else
    echo "--- KERNEL SERIAL: NO ---"
  fi
}

launch_once() {
  local label="$1"
  log "${label}: launch"
  if ! sudo env CELL_DATA_DIR CELL_IMAGES_DIR "$CELL" -v launch --repo "${REPO}" --no-attach 2>&1 | tee "/tmp/cell-abtest-${label}.log"; then
    true
  fi
  local sid
  sid="$(ls -t "${AB}/session-data" 2>/dev/null | head -1 || true)"
  log "session=${sid}"
  if [[ -n "$sid" && -f "${AB}/session-data/${sid}/vm-config.json" ]]; then
    jq '{boot_source,drives,machine_config:.machine_config}' "${AB}/session-data/${sid}/vm-config.json"
  fi
  serial_head "${sid}"
}

log "build cell"
export PATH="${PATH}:/tmp/go/bin"
(cd "$ROOT" && go build -o cell ./cmd/cell)

rm -rf "$AB"
mkdir -p "$IMAGES/bin" "${AB}/session-data"

log "Step 0: baseline bootstrap (auto-resolved artifacts)"
sudo env CELL_DATA_DIR CELL_IMAGES_DIR "$CELL" bootstrap
ls -la "$IMAGES" "$IMAGES/bin"
readlink -f "$IMAGES/vmlinux" "$IMAGES/bin/firecracker" 2>/dev/null || true
"$IMAGES/bin/firecracker" --version 2>&1 | head -1 || true
launch_once "baseline"

log "Step 1: known-good Firecracker v1.16.0 only"
FC_BIN="$(readlink -f "$IMAGES/bin/firecracker")"
curl -fsSL -o /tmp/fc-v1.16.0.tgz \
  https://github.com/firecracker-microvm/firecracker/releases/download/v1.16.0/firecracker-v1.16.0-x86_64.tgz
tar -xzf /tmp/fc-v1.16.0.tgz -C /tmp
install -m755 /tmp/release-v1.16.0-x86_64/firecracker-v1.16.0-x86_64 "$FC_BIN"
launch_once "fc-v1.16.0"

log "Step 2: known-good kernel 6.1.174 (Python success stack prefix)"
KVER="$(basename "$(readlink -f "$IMAGES/vmlinux")")"
curl -fsSL -o "${IMAGES}/${KVER}.new" \
  https://s3.amazonaws.com/spec.ccfc.min/firecracker-ci/20260624-ce269725504a-0/x86_64/vmlinux-6.1.174
mv "${IMAGES}/${KVER}.new" "${IMAGES}/${KVER}"
launch_once "kernel-6.1.174"

log "Step 3: known-good rootfs from Python launcher"
export CELL_ROOTFS_PATH="${REPO}/runtime/images/rootfs.ext4"
launch_once "rootfs-python"

log "Done. Compare KERNEL SERIAL: YES/NO above."
