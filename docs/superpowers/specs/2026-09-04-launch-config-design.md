# Design: `cell launch --config` for OpenCode agent config

## Problem

Users want to run a coding agent in a microVM with a custom OpenCode configuration (e.g. non-default model, provider settings) without editing the repository's own `opencode.json` and without polluting `cell pull` results.

## Goal

Add an optional `--config <path>` flag to `cell launch` that injects a custom OpenCode JSON config into the guest VM as the **global agent config** (`~/.config/opencode/opencode.json`).

## Non-Goals

- Changing project-level `/project/opencode.json`.
- Renaming `.filter` to `.cell` (deferred; no breaking changes in this iteration).
- Persisting the config on the host after launch.
- Hot-reloading the config into an already-running VM.

## Design

### CLI

```
cell launch --repo <path> [--config <path-to-opencode.json>]
```

- `--config` is optional; no default.
- The path is resolved to an absolute path via `filepath.Abs` + `filepath.Clean`.
- `os.Stat` must succeed and the target must be a regular file (not a directory).
- The file is validated with `json.Valid`; on invalid JSON a warning is printed to stderr and launch continues.
- The config is **ephemeral**: it is not stored in `session.json` and is not copied to any host location after launch.

### Host-side injection

During `SessionManager.buildDisk`, if a `--config` path was provided at launch, the file is copied into the temporary disk root as:

```
rootDir/.filter/opencode.json
```

with mode `0600`. This places it alongside `authorized_keys` and `opencode-server.pass` on the project disk, where the guest can read it at boot.

Because the file lives under `.filter/`, `stage.StageRepository` and `cell pull` already exclude it from synchronization.

### Guest-side injection

`guestinit/guest-entry.sh` already contains `setup_agent_home`, which creates a default `~/.config/opencode/opencode.json` if none exists.

A small addition overrides that default when the host provided a config:

```sh
CFG="${RW}/.config/opencode/opencode.json"
if [ -f "${MOUNT}/.filter/opencode.json" ]; then
  cp "${MOUNT}/.filter/opencode.json" "${CFG}"
fi
```

If the file is absent, behavior is unchanged.

### Error handling

| Case | Behavior |
|------|----------|
| `--config` file missing | `launch` returns an error |
| `--config` is a directory | `launch` returns an error |
| `--config` contains invalid JSON | Warning to stderr; launch continues |
| `--config` omitted | Unchanged behavior |
| Guest cannot read `.filter/opencode.json` | Guest logs `[guest-init] WARN`; default config is used |

## Files to change

| File | Change |
|------|--------|
| `internal/cli/launch.go` | Add `--config` flag; resolve, stat, and validate path |
| `internal/session/lifecycle.go` | Accept config path in `prepareSession`/`buildDisk`; copy to `.filter/opencode.json` |
| `internal/models/session.go` | Add `AgentConfigPath` field with `json:"-"` (transient, not serialized) |
| `guestinit/guest-entry.sh` | Copy `.filter/opencode.json` into agent home if present |
| `internal/session/lifecycle_test.go` | Unit tests for buildDisk with/without config |
| `guestinit/guest_entry_test.go` | Shell test for config override |
| `README.md` | Document the new flag |

## Success criteria

1. `cell launch --repo . --config ./my-opencode.json` starts a VM whose agent uses the provided config.
2. `cell launch --repo .` behaves exactly as before.
3. `cell pull` never brings `.filter/opencode.json` back to the host repo.
4. Invalid JSON triggers a warning but does not block the launch.
5. All new and existing tests pass.
