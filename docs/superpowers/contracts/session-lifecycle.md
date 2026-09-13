# Contract: Session lifecycle

**Phase**: 1 | **Normative for**: `internal/session/`

## Overview

`SessionManager` owns prepare → stage → disk → start → waitReady → (attach|idle) → stop/pull/rescue/status/list.

**Dependency rule (FR-026):** session code MUST depend only on `Hypervisor` and `NetworkProvider` interfaces, not concrete Firecracker/TAP types.

## Launch

```text
Launch(ctx, repoPath, attach) (*SessionRecord, error)
```

1. `prepareSession` — require kernel/rootfs/firecracker present; allocate session id; create session dir; generate SSH keypair; allocate network config; state=`created`; persist `session.json`
2. `stageRepo` — copy/stage host repo into `StagedRepoDir` (exclude patterns from config); state=`staged`
3. `buildDisk` — create project ext4 from staged repo + `.filter/` (authorized_keys, resolv, …); state=`disk_ready`
4. `startVM` — see below
5. `waitReady` — see below
6. If `attach`:
   - optionally start `sync.StartAutoPull` (when `AutoPull`)
   - `attachSSH` (blocking)
   - on attach return/error: cancel auto-pull; `Stop` session
7. If `!attach`: leave VM running; return session

## `startVM(session)`

1. Reject if already running with live pid
2. Allow restart from `stopped` if pid dead
3. `network.Setup(networkConfig)` — create TAP, iptables rules (idempotent; FR-032)
4. Write `ssh-config`, `vm-config.json`
5. `hypervisor.Start(…)` — spawn Firecracker with `stdin=DEVNULL`; serial log → `session.SerialLogPath` (NOT a temp dir). Process lifetime MUST NOT be tied to a cancelled start context (plain `exec.Command`, not `CommandContext` that dies when start returns).
6. Configure via Firecracker API; `StartInstance`
7. state=`running`, save `FCPid`, persist session

## `stop(session)` / `Stop` / `StopAll`

1. If `FCPid > 0`: send SIGTERM; wait up to 10 seconds for process exit
2. If still alive after timeout: send SIGKILL; wait to reap (bounded)
3. MUST NOT block indefinitely on `Wait()` without timeout (FR-028)
4. `network.Teardown(networkConfig)` — delete TAP, iptables rules
5. state=`stopped`, clear pid, persist

`StopAll` iterates running sessions; collects per-session errors.

## `waitReady(session)`

Poll until `runtime_ready && ssh_reachable && tmux_ready` or timeout (`SSHReadyTimeoutSec`).

**Fatal pattern detection** (return immediately with serial tail, do not wait for full timeout) — FR-033:

| Pattern | Label |
|---------|-------|
| `(?i)Kernel panic` | kernel panic |
| `(?i)\[guest-init\] ERROR:` | guest-init error |
| `(?i)Attempted to kill init` | init exited |

On timeout or fatal match: return error with serial log tail (last 20 lines).

## Status / list

- `Status(sessionID)` → `SessionStatus` with `vm_running`, `ssh_reachable`, `tmux_ready`, `runtime_ready`, guest IP, tap.
- **`vm_running` (FR-031):** `FCPid > 0` AND `Signal(0)` succeeds — NOT inferred from `State == running`.
- `List(runningOnly)` → session list entries; when `runningOnly`, filter by live VM.

## Pull / Attach / Rescue / SerialLog

- `Pull(sessionID, opts)` — rsync guest workspace → host (`Dest` default `RepoSource`); reject unsafe dest paths (FR-034); SSH MUST use `IdentitiesOnly=yes` (FR-030).
- `Attach(sessionID)` — SSH+tmux attach script (see `contracts/cli.md`).
- `Rescue(sessionID, dest)` — mount/extract project disk offline into `dest`.
- `SerialLog(sessionID)` — read `serial.log` bytes.

## Persistence

Per session under `{SessionDataDir}/{session_id}/`:

| Path | Content |
|------|---------|
| `session.json` | `SessionRecord` |
| `repo/` | Staged source |
| `project.ext4` | Project disk |
| `firecracker.socket` | API socket |
| `serial.log` | Guest serial |
| `vm-config.json` | Firecracker config snapshot |
| `id_ed25519` / `.pub` | Session SSH key |
