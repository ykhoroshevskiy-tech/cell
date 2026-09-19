# Spec: Cell Runtime

**Status:** reconciled to code at `feature/*` branches (privilege model c53c609,
version scheme caeeb17, session completion 893979e, omz plugin 4e1e07d,
claude agent support — this branch). This is the single
source of truth; feature specs live in `specs/<date>-<slug>-design.md` under the
AGENTS.md spec-first rule.

**Product intent:** Run a coding agent inside a Firecracker microVM for host
isolation; sync useful workspace changes back. Validated with OpenCode in-guest.

Normative details live in `contracts/*` and `data-model.md`.

## Privilege model

- Read-only commands run without root: `ps`, `logs`, `version`, `help`,
  `completion`, `__complete` (+ `__completeNoDesc`). They only read
  world-readable state (`session.json` 0644, `/proc/<pid>/cmdline`).
- Every other command requires root. The gate lives in `cli/root.go`
  `PersistentPreRunE`; non-root invocation fails with
  `cell: <cmd> requires root — run: sudo cell <cmd>` and exit 1.
- No file capabilities, no `cell` group, no auto-elevation (dropped in c53c609).

## Functional requirements

- **FR-026**: Session lifecycle code depends only on `Hypervisor` and
  `NetworkProvider` interfaces, not concrete implementations.
- **FR-027**: Bootstrap resolves download URLs from pinned versions
  (`ci_prefix`, `kernel_version`, `firecracker_version` in config / `CELL_*`
  env). No "latest" resolution in the hot path. Downloads retry with
  exponential backoff (3 attempts: 1s/2s/4s), write atomically (`*.tmp` →
  rename), and surface a clear error naming artifact, URL, and pins.
- **FR-027a**: Custom `CELL_KERNEL_PATH` / `CELL_FIRECRACKER_BIN` skip download
  only when the path differs from the managed default and the file is a
  non-empty non-directory. Managed defaults refresh from pins.
- **FR-027b**: Rootfs records a stamp file; changing pins/custom paths
  invalidates and rebuilds the rootfs (unless `--force`/`--rebuild-rootfs`).
- **FR-028**: `stop` sends SIGTERM, waits up to 10s, then SIGKILL. If the first
  SIGTERM returns EPERM (root-owned process), `stop` fails with
  `cannot stop VM pid <n> (started as another user?): run: sudo cell stop`
  instead of reporting false success.
- **FR-029**: Guest PID 1 stays alive by waiting on a long-lived sleeper child,
  not busy-waiting.
- **FR-030**: All SSH and rsync invocations include `-o IdentitiesOnly=yes`.
- **FR-031**: VM liveness is computed from `/proc/<pid>/cmdline` matched against
  the session API socket (world-readable, truthful without root); NOT from
  `State == running` and NOT from `Signal(0)` (EPERM false-negatives for
  root-owned processes).
- **FR-032**: Network setup is idempotent: chains are created only when missing
  (`iptables -nL` probe), firewall rules are flushed and re-added, stale TAPs
  are deleted under the global lock.
- **FR-033**: `WaitRuntimeReady` detects fatal serial-log patterns
  (`Kernel panic`, `[guest-init] ERROR:`, `Attempted to kill init`) and returns
  immediately with the serial tail.
- **FR-034**: `pull` rejects unsafe destination paths (`/`, `/usr`, `/bin`,
  `/etc`, `/var`, `/sbin`) before invoking rsync.
- **FR-035**: `cell ps` lists ALL sessions (no running-only default);
  `--json` emits machine-readable output.
- **FR-036**: `cell completion [bash|zsh|fish|powershell]` emits shell
  completion scripts (read-only, no root).
- **FR-037**: Every `--session` flag completes dynamically via `__complete` as
  `id\t<repo> (<running|stopped>)`, powered by `SessionManager.ListSummaries`
  (JSON + `/proc` reads only — no SSH/TCP probes; milliseconds).
- **FR-038**: The version is derived from git history at build time
  (`0.1.<commit-count>+g<short-sha>[-dirty]`) via `scripts/build.sh` and
  `-ldflags`; `cell version` prints it. Plain `go build` yields `0.0.0+dev`.
- **FR-039**: The oh-my-zsh plugin (`plugins/cell/cell.plugin.zsh`) provides
  aliases with baked flags (`ca` → `sudo cell attach --session`, `cl` →
  `sudo cell launch --repo`, `cs`, `cps`, `cstop`, `crm`, `cssh`, `cstat`,
  `cverify`, `clogs`, `cpull`, `crescue`, `cboot`, `cnet`, `cver`,
  `cstopall`) and live session completion via `cell __complete`.
- **FR-040**: Agent kinds are `opencode` (default; legacy `""` normalizes to
  opencode)`, `claude`, and `none`. `CELL_AGENT`/`launch --agent` select the
  kind; the session record persists `Agent` and `start`/`attach` honor the
  stored kind, not the current env.
- **FR-041**: `claude` guests install `claude` from the pinned npm registry
  tarball (`CELL_AGENT_URL` override; tarball extraction handles the npm
  `package/` prefix) and boot it as a raw tmux TUI over SSH — no opencode
  serve mode, no password file, `~/.claude/settings.json` gets
  `permissions.defaultMode=bypassPermissions` (VM boundary is the protection).
- **FR-042**: `none` skips agent install at bootstrap and tmux boot in the
  guest (SSH-only runtime). The rootfs stamp includes the agent kind so
  switching kinds rebuilds. Host attach dispatches per kind: opencode →
  tunnel TUI (host agent binary required), claude/none → `ssh -t` shell path
  (no host agent requirement), and the opencode server health probe is
  skipped for claude/none guests.

## Standing gates (always keep green)

1. `scripts/build.sh && ./cell version` → prints `cell 0.1.<count>+g<sha>`
   (`-dirty` suffix when the tree is not clean).
2. `./cell ps` → exit 0 without root (empty or populated listing).
3. `./cell launch --repo .` without root → prints
   `error: cell: launch requires root — run: sudo cell launch`, exit 1.
4. `go build ./... && go vet ./... && go test ./...` → all green.
5. `./cell __complete attach --session ''` (with a seeded session) → prints
   `id\trepo (state)` rows and the `NoFileComp` directive.

KVM-only behavior (real boot/attach/pull) is verified by the operator via
`sudo scripts/e2e-network.sh ./cell`.
