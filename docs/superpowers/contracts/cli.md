# Contract: CLI

**Phase**: 1 | **Normative for**: `internal/cli/`, `cmd/cell/`

## Platform

- Linux + KVM required (`runtime.GOOS != linux` → exit).
- Runtime commands require root (`geteuid() == 0`), except `version`, `help`, `--help`, `-h`.

## Global flags

| Flag | Effect |
|------|--------|
| `--quiet` | Suppress non-error output |
| `--verbose` / `-v` | Verbose host-side tracing |

Config loaded via `config.Load()` (`CELL_*` env, see `data-model.md` / `contracts/bootstrap.md`).

## Commands

### `cell version`

Print `cell 0.1.0`.

### `cell bootstrap`

Download/build kernel, rootfs, firecracker (`bootstrap.Ensure`).

| Flag | Effect |
|------|--------|
| `--force` | Re-download pinned artifacts; rebuild rootfs |
| `--rebuild-rootfs` | Rebuild rootfs only |

On success print paths for firecracker, kernel, rootfs.

### `cell launch`

| Flag | Required | Effect |
|------|----------|--------|
| `--repo` | yes | Source repository directory |
| `--no-attach` | no | Wait until ready; do not exec SSH |

Flow: `Ensure` → `SessionManager.Launch(repo, attach=!noAttach)`. With attach: auto-pull while attached (if `AutoPull`), then stop session after SSH exits. With `--no-attach`: print `Session {id} ready at {guest_ip}`.

### `cell stop`

Exactly one of:

| Flag | Effect |
|------|--------|
| `--session {id}` | Stop one session |
| `--all` | Stop all running sessions |

### `cell ssh`

| Flag | Required |
|------|----------|
| `--session` | yes |

Reattach SSH+tmux via `SessionManager.Attach`.

### `cell status`

| Flag | Required |
|------|----------|
| `--session` | yes |
| `--json` | no |

Probe `vm_running` / `ssh` / `tmux` / `runtime` (+ guest IP, tap).

### `cell verify`

| Flag | Required |
|------|----------|
| `--session` | yes |

OK if `runtime && ssh && tmux`; else print serial tail (20 lines) to stderr and fail.

### `cell logs`

| Flag | Required |
|------|----------|
| `--session` | yes |

Print `serial.log` bytes to stdout.

### `cell ps`

| Flag | Effect |
|------|--------|
| `--all` | List all valid sessions (default: running-only filter) |
| `--json` | JSON list |

Table columns: SESSION, STATE, GUEST_IP, REPO, CREATED.

### `cell pull`

| Flag | Required | Effect |
|------|----------|--------|
| `--session` | yes | Session ID |
| `--dest` | no | Destination (default: session `repo_source`) |
| `--dry-run` | no | rsync dry-run |
| `--delete` | no | Delete host files absent in guest |

### `cell rescue`

| Flag | Required |
|------|----------|
| `--session` | yes |
| `--dest` | yes |

Extract workspace from project disk (offline; VM need not be running).

## SSH exec shape (attach)

```text
ssh -t -i {key} -o IdentitiesOnly=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
  {ssh_user}@{guest_ip} \
  'TMUX_SESSION={tmux_session_name} REPO_DIR={guest_repo_dir} \
   AGENT_BIN={agent_bin} AGENT_CMD={agent_cmd} \
   /opt/guest-init/tmux-attach.sh'
```

`-o IdentitiesOnly=yes` is REQUIRED on attach/pull/status probes that use SSH (FR-030).
