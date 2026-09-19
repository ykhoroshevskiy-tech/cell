#!/bin/sh
set -eu

PROJECT_DISK="${PROJECT_DISK:-/dev/vdb}"
MOUNT="/project"
REPO_DIR="${MOUNT}"
AGENT_USER="${AGENT_USER:-agent}"
TMUX_SESSION="${TMUX_SESSION:-agent}"
AGENT_KIND="${AGENT_KIND:-opencode}"

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
   AGENT_KIND="opencode"
   KIND_FILE="${REPO_DIR}/.filter/agent.kind"
   if [ -s "${KIND_FILE}" ]; then
     AGENT_KIND="$(cat "${KIND_FILE}")"
   fi
   case "${AGENT_KIND}" in
     opencode|claude|none) ;;
     *)
       AGENT_KIND="opencode"
       log "unknown agent kind in ${KIND_FILE}; defaulting to opencode"
       ;;
   esac
   log "agent kind: ${AGENT_KIND}"
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

ensure_filter_gitignore() {
  GI="${REPO_DIR}/.gitignore"
  if [ -f "${GI}" ] && grep -qxE '\.filter(/)?' "${GI}"; then
    return 0
  fi
  if [ ! -f "${GI}" ]; then
    printf '%s\n' '.filter/' > "${GI}"
  else
    printf '\n%s\n' '.filter/' >> "${GI}"
  fi
  chown "${AGENT_USER}:${AGENT_USER}" "${GI}" 2>/dev/null || true
}

setup_agent_home() {
  AGENT_HOME="/home/${AGENT_USER}"
  RW="${MOUNT}/.filter/agent-home"
  if [ "${AGENT_KIND}" = "opencode" ]; then
    mkdir -p "${RW}/.cache" "${RW}/.config/opencode" "${RW}/.local/share"
    CFG="${RW}/.config/opencode/opencode.json"
    if [ -f "${MOUNT}/.filter/opencode.json" ]; then
      if cp "${MOUNT}/.filter/opencode.json" "${CFG}"; then
        log "agent config from host .filter/opencode.json"
      else
        log "WARN: failed to copy host opencode.json; using default"
      fi
    fi
    if [ ! -f "${CFG}" ]; then
      if [ -d /opt/opencode-plugins/node_modules/superpowers ]; then
        cat > "${CFG}" <<'EOF'
{
  "$schema": "https://opencode.ai/config.json",
  "permission": "allow",
  "plugin": ["/opt/opencode-plugins/node_modules/superpowers"]
}
EOF
      else
        cat > "${CFG}" <<'EOF'
{
  "$schema": "https://opencode.ai/config.json",
  "permission": "allow"
}
EOF
      fi
    fi
  else
    mkdir -p "${RW}/.cache" "${RW}/.local/share"
  fi
  if [ "${AGENT_KIND}" = "claude" ]; then
    mkdir -p "${RW}/.claude"
    cat > "${RW}/.claude/settings.json" <<'EOF'
{
  "permissions": {
    "defaultMode": "bypassPermissions"
  }
}
EOF
    log "claude settings written to ${RW}/.claude/settings.json"
  fi
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

# Rootfs is Firecracker RO; /usr/local lives there. Seed a project-disk copy and bind it
# so the agent can install to /usr/local/bin (npm -g, etc.). Ceiling: copy is ~Node size;
# upgrade: overlayfs if re-copy after rootfs Node bumps becomes painful.
setup_usr_local_rw() {
  RW="${MOUNT}/.filter/usr-local"
  mkdir -p "${RW}"
  if [ ! -f "${RW}/.seeded" ]; then
    cp -a /usr/local/. "${RW}/"
    touch "${RW}/.seeded"
  fi
  if ! mountpoint -q /usr/local; then
    mount --bind "${RW}" /usr/local
  fi
  chown -R "${AGENT_USER}:${AGENT_USER}" "${RW}"
  log "usr/local rw at ${RW}"
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
  if [ "${AGENT_KIND}" = "none" ]; then
    log "agent disabled (none); ssh-only runtime, no tmux boot"
    return 0
  fi
  ERR="/run/tmux-start.err"
  agent_tmux() {
    su - "${AGENT_USER}" -c "$*"
  }
  if [ "${AGENT_KIND}" = "claude" ]; then
    TMUX_SPEC="
    cd '${REPO_DIR}' &&
    tmux new-session -d -s '${TMUX_SESSION}' -c '${REPO_DIR}' \
      'export HOME=${AGENT_HOME}; export TMPDIR=/tmp; \
       export PATH=/usr/local/bin:/usr/bin:/bin; \
       cd ${REPO_DIR}; \
       exec claude --dangerously-skip-permissions'
  "
  else
    PASS_FILE="${REPO_DIR}/.filter/opencode-server.pass"
    PORT_FILE="${REPO_DIR}/.filter/opencode-serve.port"
    SERVE_PORT=4096
    if [ -s "${PORT_FILE}" ]; then
      SERVE_PORT=$(cat "${PORT_FILE}")
    fi
    if [ ! -s "${PASS_FILE}" ]; then
      log "ERROR: missing ${PASS_FILE}"
      return 1
    fi
    cat > /run/opencode.env <<EOF
OPENCODE_SERVER_PASSWORD=$(cat "${PASS_FILE}")
EOF
    chmod 600 /run/opencode.env
    chown "${AGENT_USER}:${AGENT_USER}" /run/opencode.env
    TMUX_SPEC="
    cd '${REPO_DIR}' &&
    tmux new-session -d -s '${TMUX_SESSION}' -c '${REPO_DIR}' \
      'export HOME=${AGENT_HOME}; export TMPDIR=/tmp; export BUN_TMPDIR=/tmp; \
       export npm_config_cache=/tmp/npm-cache; \
       export XDG_CACHE_HOME=${AGENT_HOME}/.cache; export XDG_CONFIG_HOME=${AGENT_HOME}/.config; \
       export PATH=/usr/local/bin:/usr/bin:/bin; \
       set -a; . /run/opencode.env; set +a; \
       exec opencode serve --hostname 127.0.0.1 --port ${SERVE_PORT}'
  "
  fi
  : > "${ERR}"
  if agent_tmux "tmux has-session -t '${TMUX_SESSION}'" 2>/dev/null; then
    log "tmux session ${TMUX_SESSION} already exists"
    return 0
  fi
  if agent_tmux "${TMUX_SPEC}" 2>>"${ERR}"; then
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
ensure_filter_gitignore
chown_repo
setup_runtime_dirs
setup_dns
setup_agent_home
setup_usr_local_rw
setup_ssh
if start_tmux_session; then
  log "runtime ready"
else
  log "runtime degraded: ssh only (tmux failed)"
fi
# PID 1: stay alive (sshd/tmux children daemonize; block on long-lived sleeper, no wait spin)
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
