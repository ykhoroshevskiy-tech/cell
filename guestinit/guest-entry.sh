#!/bin/sh
set -eu

PROJECT_DISK="${PROJECT_DISK:-/dev/vdb}"
MOUNT="/project"
REPO_DIR="${MOUNT}"
AGENT_USER="${AGENT_USER:-agent}"
TMUX_SESSION="${TMUX_SESSION:-agent}"

log() { echo "[guest-init] $*"; }

# region agent log
debug_log() { echo "[guest-init-debug] $*"; }
# endregion

wait_disk() {
  for _ in $(seq 1 60); do
    if [ -b "${PROJECT_DISK}" ]; then
      return 0
    fi
    sleep 0.2
  done
  log "ERROR: project disk ${PROJECT_DISK} not found"
  exit 1
}

mount_project() {
  wait_disk
  if ! mountpoint -q "${MOUNT}"; then
    mount "${PROJECT_DISK}" "${MOUNT}" || {
      log "mount failed, trying mkfs"
      mkfs.ext4 -F "${PROJECT_DISK}"
      mount "${PROJECT_DISK}" "${MOUNT}"
    }
  fi
  log "project mounted at ${MOUNT}"
  if [ ! -f "${REPO_DIR}/.filter-staged" ]; then
    log "ERROR: project marker missing in ${REPO_DIR}"
    exit 1
  fi
  log "project marker ok"
}

chown_repo() {
  chown -R "${AGENT_USER}:${AGENT_USER}" "${REPO_DIR}"
  log "repo owned by ${AGENT_USER}"
}

setup_runtime_dirs() {
  if ! mountpoint -q /proc; then
    mount -t proc proc /proc
  fi
  if ! mountpoint -q /sys; then
    mount -t sysfs sysfs /sys
  fi
  if ! mountpoint -q /run; then
    mount -t tmpfs tmpfs /run
  fi
  mkdir -p /run/sshd
  if ! mountpoint -q /tmp; then
    mount -t tmpfs tmpfs /tmp
  fi
  chmod 1777 /tmp
  mkdir -p /dev/pts
  if ! mountpoint -q /dev/pts; then
    mount -t devpts devpts /dev/pts -o gid=5,mode=620,ptmxmode=666
  fi
  log "runtime dirs ready (/proc, /sys, /run, /tmp, /dev/pts)"
}

setup_dns() {
  DNS_SRC="${MOUNT}/.filter/resolv.conf"
  if [ ! -f "${DNS_SRC}" ]; then
    DNS_SRC="/run/filter-resolv.conf"
    cat > "${DNS_SRC}" <<EOF
nameserver 1.1.1.1
nameserver 8.8.8.8
options timeout:1 attempts:2
EOF
  fi
  if [ -e /etc/resolv.conf ]; then
    mount --bind "${DNS_SRC}" /etc/resolv.conf
    log "dns configured from ${DNS_SRC}"
  else
    log "WARN: /etc/resolv.conf missing; dns not configured"
  fi
}

setup_agent_home() {
  AGENT_HOME="/home/${AGENT_USER}"
  RW="${MOUNT}/.filter/agent-home"
  mkdir -p "${RW}/.cache" "${RW}/.config" "${RW}/.local/share"
  for dot in .zshrc .profile .bashrc; do
    if [ ! -e "${RW}/${dot}" ] && [ -e "${AGENT_HOME}/${dot}" ]; then
      cp -a "${AGENT_HOME}/${dot}" "${RW}/"
    fi
  done
  cat > "${RW}/.zshenv" <<'EOF'
export TMPDIR=/tmp
export BUN_TMPDIR=/tmp
export npm_config_cache=/tmp/npm-cache
export PATH=/usr/local/bin:/usr/bin:/bin
mkdir -p /tmp/npm-cache 2>/dev/null || true
EOF
  if ! mountpoint -q "${AGENT_HOME}"; then
    mount --bind "${RW}" "${AGENT_HOME}"
  fi
  chown -R "${AGENT_USER}:${AGENT_USER}" "${RW}"
  log "agent home rw at ${AGENT_HOME} (${RW})"
}

setup_ssh() {
  AUTH_SRC="${MOUNT}/.filter/authorized_keys"
  if [ ! -f "${AUTH_SRC}" ]; then
    log "ERROR: missing ${AUTH_SRC}"
    exit 1
  fi
  /usr/sbin/sshd
  log "sshd started (AuthorizedKeysFile ${AUTH_SRC})"
}

start_tmux_session() {
  AGENT_HOME="/home/${AGENT_USER}"
  PASS_FILE="${REPO_DIR}/.filter/opencode-server.pass"
  if [ ! -s "${PASS_FILE}" ]; then
    log "ERROR: missing ${PASS_FILE}"
    return 1
  fi
  cat > /run/opencode.env <<EOF
OPENCODE_SERVER_PASSWORD=$(cat "${PASS_FILE}")
OPENCODE_PERMISSION={"*":"allow"}
EOF
  chmod 600 /run/opencode.env
  chown "${AGENT_USER}:${AGENT_USER}" /run/opencode.env
  ERR="/run/tmux-start.err"
  agent_tmux() {
    su - "${AGENT_USER}" -c "$*"
  }
  : > "${ERR}"
  if agent_tmux "tmux has-session -t '${TMUX_SESSION}'" 2>/dev/null; then
    log "tmux session ${TMUX_SESSION} already exists"
    return 0
  fi
  if agent_tmux "
    cd '${REPO_DIR}' &&
    tmux new-session -d -s '${TMUX_SESSION}' -c '${REPO_DIR}' \
      'export HOME=${AGENT_HOME}; export TMPDIR=/tmp; export BUN_TMPDIR=/tmp; \
       export npm_config_cache=/tmp/npm-cache; \
       export XDG_CACHE_HOME=${AGENT_HOME}/.cache; export XDG_CONFIG_HOME=${AGENT_HOME}/.config; \
       export PATH=/usr/local/bin:/usr/bin:/bin; \
       set -a; . /run/opencode.env; set +a; \
       exec opencode serve --hostname 127.0.0.1 --port 4096'
  " 2>"${ERR}"; then
    if agent_tmux "tmux has-session -t '${TMUX_SESSION}'" 2>/dev/null; then
      log "tmux session ${TMUX_SESSION} started"
      return 0
    fi
    log "WARN: tmux command succeeded but session ${TMUX_SESSION} missing"
  else
    log "WARN: tmux session ${TMUX_SESSION} failed to start"
  fi
  if [ -s "${ERR}" ]; then
    log "tmux stderr:"
    while IFS= read -r line; do
      log "  ${line}"
    done < "${ERR}"
  fi
  return 1
}

mount_project
chown_repo
setup_runtime_dirs
setup_dns
setup_agent_home
setup_ssh
if start_tmux_session; then
  log "runtime ready"
else
  log "runtime degraded: ssh only (tmux failed)"
fi
# PID 1: stay alive (sshd/tmux children daemonize; block on long-lived sleeper, no wait spin)
# ponytail: single blocking wait on sleep child; upgrade path: SIGCHLD trap for reaping
# region agent log
debug_log "idle-block entering"
# endregion
while true; do
  sleep 86400 &
  sleeper_pid=$!
  # region agent log
  debug_log "idle-block waiting pid=${sleeper_pid}"
  # endregion
  wait "${sleeper_pid}" 2>/dev/null || true
  # region agent log
  debug_log "idle-block sleeper exited pid=${sleeper_pid}"
  # endregion
done
