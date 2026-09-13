# Contract: Guest Boot Sequence

**Phase**: 1 | **Normative for**: `guestinit/guest-entry.sh`, `guestinit/tmux-attach.sh` (both embedded via //go:embed in Go binary)

## Init process

Kernel cmdline `init=/opt/guest-init/guest-entry.sh` (embedded from host bootstrap).

Environment defaults:

| Var | Default |
|-----|---------|
| `PROJECT_DISK` | `/dev/vdb` |
| `AGENT_USER` | `agent` |
| `TMUX_SESSION` | `agent` |

Paths (constants in `guest-entry.sh`):

| Name | Path |
|------|------|
| `MOUNT` | `/project` (project disk mount) |
| `REPO_DIR` | `/project` (staged repo at disk root; same as mount) |

## Ordered steps

Each step logs `[guest-init] {message}` to serial console (stdout).

1. **wait_disk** — poll up to 60×0.2s for block device `PROJECT_DISK`
2. **mount_project** — mount `/dev/vdb` → `/project`; on failure mkfs.ext4 + retry; verify `/project/.filter-staged` exists else ERROR exit
3. **chown_repo** — `chown -R $AGENT_USER:$AGENT_USER /project` (repo tree only; do NOT chown `/project/.filter/`)
4. **setup_runtime_dirs** — mount proc, sys, run tmpfs, /tmp tmpfs (mode 1777), devpts
5. **setup_dns** — bind `/project/.filter/resolv.conf` if exists; else write fallback to `/run/filter-resolv.conf` and bind
6. **setup_agent_home** — mkdir `.filter/agent-home/{.cache,.config,.local/share}`; write `.zshenv`; bind-mount RW → `/home/agent`; chown agent (no vendor-specific agent config files)
7. **setup_ssh** — require `/project/.filter/authorized_keys`; start `/usr/sbin/sshd`
8. **start_tmux_session** — detached tmux session in `REPO_DIR` with idle `zsh -l` (agent CLI MUST NOT start at boot)
9. **readiness log** — if tmux ok: `[guest-init] runtime ready`; else degraded message

## `.zshenv` content

```sh
export TMPDIR=/tmp
export BUN_TMPDIR=/tmp
export npm_config_cache=/tmp/npm-cache
export PATH=/usr/local/bin:/usr/bin:/bin
mkdir -p /tmp/npm-cache 2>/dev/null || true
```

## tmux session at boot

Working directory: `/project` (`REPO_DIR`). Session name: `$TMUX_SESSION` (default `agent`).

Command chain at boot:
```text
exec zsh -l
```

If tmux already exists for session name, skip create (idempotent).

## SSH attach script

Path: `/opt/guest-init/tmux-attach.sh` (embedded in Go binary at compile time, mode 755)

Run as `agent` via `ssh -t`. Environment from host:

| Var | Default (host config) |
|-----|------------------------|
| `TMUX_SESSION` | `agent` |
| `REPO_DIR` | `/project` |
| `AGENT_BIN` | from `CELL_AGENT_BIN` (default `opencode`) |
| `AGENT_CMD` | from `CELL_AGENT_CMD` (default `opencode --auto`) |

Script MUST use `REPO_DIR` directly (no alias to nested `workspace/` paths).

Behavior:

1. If tmux session missing → `cd $REPO_DIR` then `tmux new-session` running `$AGENT_CMD; exec zsh -l` (or plain session if `AGENT_CMD` empty)
2. If `$AGENT_BIN` process running for agent user → `tmux attach-session`
3. Else if `AGENT_CMD` set → attach and `send-keys "$AGENT_CMD" Enter`
4. Else → attach only

## PID 1 loop

After init steps, stay alive without busy-waiting (MUST NOT spin on bare `wait`):

```sh
while true; do
  sleep 86400 &
  wait $! 2>/dev/null || true
done
```

## Out of scope / MUST NOT

- Copy agent binary to project disk (lives in rootfs under `/opt/agent` + `/usr/local/bin`)
- Start the coding agent at boot (only on SSH attach)
