# OpenCode host TUI + guest server

Date: 2026-08-16

## Problem

`cell launch` SSHes into the guest and runs OpenCode TUI inside tmux. Clipboard, mouse, and paste belong to the SSH/tmux/TUI stack in a headless VM. OpenCode already splits: `opencode serve` (tools, repo, sessions) and `opencode attach` (TUI).

## Goal

- Server runs in the Firecracker guest (not a container).
- TUI runs on the host and talks to that server through an SSH local forward.
- Exiting the TUI does not stop the VM.
- After host reboot, `cell start <id>` boots the same `project.ext4` so OpenCode chat/auth on disk come back.

## Non-goals

- IDE plugins, ACP, web UI.
- Exposing guest `:4096` on the TAP (no iptables change).
- Docker/container runtime.
- Merging host-repo edits with a stopped guest disk.
- Background auto-pull while nobody is attached.
- Copying the guest OpenCode binary onto the host.

## Architecture

```
host: cell launch | start | attach
        ssh -N -L {hostPort}:127.0.0.1:4096
        opencode attach http://127.0.0.1:{hostPort} --dir /project --continue -p …
              │
guest: sshd
       tmux session (process babysitter only)
         opencode serve --hostname 127.0.0.1 --port 4096
```

OpenCode session data lives in guest `~` which is bind-mounted from `/project/.filter/agent-home` on `project.ext4`. File edits sync with existing `cell pull` / auto-pull **while attach is in the foreground**. After TUI exit, pull with `cell pull` or attach again.

## Commands

| Command | Behavior |
|---------|----------|
| `cell launch --repo PATH` | New session id, stage, mkfs `project.ext4`, boot, wait server health, generate password once, attach TUI. **Do not `Stop` on TUI exit.** `--no-attach` skips TUI, VM stays up. |
| `cell attach --session ID` | VM must be running. SSH tunnel + host `opencode attach --continue`. Auto-pull for the duration of this process. |
| `cell start --session ID` | VM must be down (or pid stale after reboot). Reuse **existing** `project.ext4` (no restage, no mkfs). Recreate TAP from saved network config, boot, wait health, then attach unless `--no-attach`. If VM is already running: exit 0 and print `cell attach` hint. |
| `cell stop --session ID` | Kill Firecracker + TAP. Disk and password file stay. |
| `cell ssh --session ID` | SSH into guest tmux (serve logs / debug shell). No host TUI, no tunnel to 4096. |

`cell status` / `verify` / `ps`: report `server=` (HTTP health) instead of `tmux=` as the readiness bit. tmux may still be up; it is not the signal.

## Guest

`guest-entry.sh` starts sshd, then a detached tmux session whose command is serve, not an interactive TUI:

- Bind `127.0.0.1:4096` only.
- Read password from `/project/.filter/opencode-server.pass` (written at launch onto the project disk).
- Auto-approve: set on the **server** process (env / OpenCode permission config). `opencode attach` has no `--auto`.
- Log a clear ready line after serve is intended to be up. Host still waits on health, not the log line alone.

`tmux-attach.sh`: attach to that session for `cell ssh` only. Do not `send-keys` a TUI command.

Bootstrap: ensure `curl` exists in the guest rootfs (jammy deb extract, same pattern as `rsync`). Needed for `ssh … curl -sf http://127.0.0.1:4096/global/health`. Rebuild rootfs required once (`cell bootstrap --rebuild-rootfs`).

Default `CELL_AGENT_CMD`: `opencode serve --hostname 127.0.0.1 --port 4096`.

## Host attach / tunnel

1. Require host `opencode` (`CELL_HOST_AGENT_BIN` or `PATH`). Fail before opening SSH if missing. Same major version as the guest binary; cell does not install the host client.
2. Pick a free localhost TCP port (do not hardcode 4096 — two sessions collide). Store last used port on the session record (informational; each attach may pick a new one).
3. `ssh -N -o ExitOnForwardFailure=yes -L {hostPort}:127.0.0.1:4096` using existing session key + guest IP. Keep the tunnel until the TUI process exits, then kill it. Do not kill the VM.
4. `opencode attach http://127.0.0.1:{hostPort} --dir /project --continue -p <password>`. `--dir /project` is the path **inside the guest**.
5. Password: file `{sessionDir}/opencode-server.pass` mode `0600`, generated once at launch, copied onto the project disk as `.filter/opencode-server.pass`. `start` reuses it (do not rotate; that would require rewriting a stopped ext4).

## Persistence

| Data | Survives TUI exit | Survives `cell stop` | Survives host reboot + `cell start` |
|------|-------------------|----------------------|-------------------------------------|
| In-flight model turn | yes (serve still running) | no | no |
| OpenCode chat / auth on disk | yes | yes (on ext4) | yes |
| Repo files on guest disk | yes | yes | yes |
| Host working tree | only if pull ran | only if pull ran | only if pull ran |

If the host edited the repo while the VM was stopped, `cell pull` after start overwrites the host with the guest disk. No merge in this spec.

Stale `session.json` `running` + dead pid (host reboot): treat as not running and allow `start`.

## Config

| Key | Default | Role |
|-----|---------|------|
| `CELL_AGENT_CMD` | `opencode serve --hostname 127.0.0.1 --port 4096` | Guest serve command |
| `CELL_AGENT_SERVE_PORT` | `4096` | Port **inside** the guest (forward target) |
| `CELL_HOST_AGENT_BIN` | `opencode` | Host TUI binary |
| `CELL_AGENT_BIN` | `opencode` | Guest binary name (unchanged) |

TAP/firewall unchanged: guest may still reach host `:8080`; host reaches guest only via SSH.

## Errors

- Missing host `opencode`: fail fast, VM left as-is if already booted.
- Health timeout: existing verify path (serial tail). Do not auto-stop.
- Bind/forward port busy: choose another localhost port.
- `start` with no `project.ext4`: error; `rescue` remains for extracting files without boot.
- Tunnel or TUI crash: serve stays; user runs `cell attach` again.

## Tests

No Firecracker in unit tests.

- Default config: `AgentCmd` is serve; serve port 4096.
- `ServerReady` true only when the health probe command succeeds (replace `TmuxReady` helper).
- Attach builds `ssh -N -L` with `ExitOnForwardFailure` and does not invoke `tmux-attach.sh`.
- `start` with missing disk fails; `start` when `VMRunning` does not restage/mkfs.
- Password file written `0600`; launch copies it under `.filter/` in the staged tree.

One runnable check is enough per non-trivial helper (probe, forward args, start guards).

## Docs

README: flow is serve in guest + TUI on host; `launch` / `attach` / `start` / `stop`; host needs OpenCode; clipboard is local TUI; `cell ssh` is debug only.
