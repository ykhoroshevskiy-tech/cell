# Tasks: Cell Runtime

Checkboxes reflect current codebase status.

## Implemented (code present)

- [x] T001 Config + `CELL_*` load (`internal/config`)
- [x] T002 Models (`internal/models`)
- [x] T003 Bootstrap download/build + pin resolve (`internal/bootstrap`)
- [x] T004 Hypervisor Firecracker (`internal/hypervisor`, `internal/firecracker`)
- [x] T005 Network TAP (`internal/network`)
- [x] T006 Stage repo (`internal/stage`)
- [x] T007 Session lifecycle (`internal/session`)
- [x] T008 Guest init embed (`guestinit/`)
- [x] T009 CLI commands (`internal/cli`, `cmd/cell`)
- [x] T010 SSH attach / readiness (`internal/ssh`)
- [x] T011 Pull / auto-pull (`internal/sync`)
- [x] T012 Pinned artifacts via config defaults + env (FR-027 / 027a / 027b)
- [x] T013 Rootfs stamp invalidation
- [x] T014 `internal/sync/pull.go` — `PullWorkspace()` via rsync over SSH; SSH args include `-o IdentitiesOnly=yes`; rejects unsafe dest paths (`/`, `/usr`, `/bin`, `/etc`, `/var`, `/sbin`)
- [x] T015 `internal/ssh/attach.go` — `WaitForSSH()`, `WaitRuntimeReady()` (with fatal-pattern detection: kernel panic, `[guest-init] ERROR:`, `Attempted to kill init`), `Attach()`, `TmuxReady()`, session status probes
- [x] T016 `hypervisor.Stop` implements FR-028 (SIGTERM → 10s → SIGKILL, EPERM honesty)

## Open

- [ ] T017 Optional: restore non-empty `Artifact.SHA256` pins for kernel/fc
