# Pinned bootstrap artifacts

## Goal

Make `cell bootstrap` reproducible by default. Artifact versions are pinned in code; operators override via `CELL_*` env vars. No silent float to “latest”.

## Motivation

Today bootstrap always resolves the newest Firecracker CI prefix, kernel, squashfs, and GitHub Firecracker release. Paths (`CELL_KERNEL_PATH`, etc.) can point at local files, but versions themselves cannot be pinned. That breaks repro and makes regressions hard to bisect.

## Approach

Four pin fields on `CellConfig`, with hardcoded defaults and env overrides:

| Config field           | Env                          | Default (initial)                                      |
|------------------------|------------------------------|--------------------------------------------------------|
| `ci_prefix`            | `CELL_CI_PREFIX`             | `firecracker-ci/20260708-f11c230ed107-0/`              |
| `kernel_version`       | `CELL_KERNEL_VERSION`        | `6.1.176`                                              |
| `firecracker_version`  | `CELL_FIRECRACKER_VERSION`   | `v1.16.1`                                              |
| `squashfs_version`     | `CELL_SQUASHFS_VERSION`      | `24.04`                                                |

Initial defaults are the last known-good stack. Before shipping, confirm the four URLs still return 200; adjust pins if not.

## Resolve rules

1. Bootstrap builds download URLs only from pins — no S3 listing, no GitHub `latest` redirect in the hot path.
2. URL shapes (unchanged):
   - kernel: `{s3}/{ci_prefix}{arch}/vmlinux-{kernel_version}`
   - squashfs: `{s3}/{ci_prefix}{arch}/ubuntu-{squashfs_version}.squashfs`
   - firecracker: `https://github.com/firecracker-microvm/firecracker/releases/download/{tag}/firecracker-{tag}-{arch}.tgz`
3. Path overrides win: if `CELL_KERNEL_PATH` / `CELL_FIRECRACKER_BIN` / `CELL_SQUASHFS_PATH` is set and the file exists, skip download for that artifact (version unused for it).
4. No auto-fallback to latest. HTTP failure → error that includes the pin values used.
5. Partial env override is allowed; stack compatibility is the operator’s problem.

## Code changes

- `internal/config/cell_config.go` — fields, defaults, bind/env.
- `internal/bootstrap/versions.go` — `resolveArtifacts(arch, pins)` constructs URLs from pins. Keep existing “select latest” helpers only if still useful for unit tests of sorting; they must not run during bootstrap.
- `internal/bootstrap/bootstrap.go` — pass pins from `cfg` into resolve.
- README Configuration — document the four vars and that defaults are pinned.

## Tests

- Unit (no network): given pins + arch, URLs and versions match expected strings.
- Config: env overrides populate the four fields.
- Existing network `TestResolveArtifacts`: remove or gate behind `testing.Short` / build tag so CI does not depend on live S3/GitHub.

## Out of scope

- `CELL_ARTIFACTS=latest` escape hatch
- Stack YAML / named profiles
- Automatic compatibility checks between FC/kernel/squashfs
- Changing path-override behavior beyond “path wins”
