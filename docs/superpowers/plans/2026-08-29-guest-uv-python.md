# Guest uv + CPython Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pin `uv` 0.12.7 and CPython 3.13 into the guest rootfs at bootstrap so `uv` and `python3` are on PATH.

**Architecture:** Same as Node: host downloads a pinned GitHub tarball into `ImagesDir`, installs `uv` into the chroot `/usr/local/bin`, then chroot `uv python install --default` with `UV_PYTHON_INSTALL_DIR` / `UV_PYTHON_BIN_DIR`. Rootfs stamp includes both pins.

**Tech Stack:** Go, debootstrap chroot, curl, tar, uv.

## Global Constraints

- `CELL_UV_VERSION` default `0.12.7`
- `CELL_PYTHON_VERSION` default `3.13` (minor pin; patch is whatever uv resolves at bootstrap)
- uv tarball: `https://github.com/astral-sh/uv/releases/download/{version}/uv-{arch}-unknown-linux-gnu.tar.gz` where arch is `x86_64` (amd64) or `aarch64` (arm64)
- After uv is installed: `UV_PYTHON_INSTALL_DIR=/usr/local/share/uv/python`, `UV_PYTHON_BIN_DIR=/usr/local/bin`, `uv python install --default {CELL_PYTHON_VERSION}`
- Stamp appends `+uv:{UvVersion}+py:{PythonVersion}` to the existing `debootstrap:noble+apt+node:{NodeVersion}` string
- `guestCustomizeScript` must contain `command -v uv` and `command -v python3`
- Do not add `cell launch --snapshot` / `--language` / `--env`
- Do not change `CELL_NODE_VERSION` default `v24.20.0`
- No new Go dependencies
- TDD: failing test first, then implementation
- After Go changes: `go build -o cell ./cmd/cell` from repo root
- Do not commit `.cursor/`, `docs/`, `opencode.json`

---

## File map

- `internal/config/cell_config.go` — `UvVersion`, `PythonVersion`, defaults, env
- `internal/config/cell_config_test.go` — default + env tests
- `internal/bootstrap/bootstrap.go` — stamp, install uv, chroot python, customize
- `internal/bootstrap/guest_git_sudo_test.go` — URL, stamp, customize
- `internal/bootstrap/bootstrap_behavior_test.go` — stamp strings
- `README.md` — env table + one sentence

### Task 1: Config pins for uv and Python

**Files:**
- Modify: `internal/config/cell_config.go`
- Modify: `internal/config/cell_config_test.go`

**Interfaces:**
- Consumes: existing `Load()` / `Default()` pattern for `NodeVersion`
- Produces: `CellConfig.UvVersion string` (`mapstructure:"uv_version"`), `CellConfig.PythonVersion string` (`mapstructure:"python_version"`). Defaults `0.12.7` and `3.13`. Env `CELL_UV_VERSION`, `CELL_PYTHON_VERSION`. Empty after unmarshal filled from `Default()` like Node.

- [ ] **Step 1: Write the failing tests**

In `TestLoadAgentServeDefaults` after the NodeVersion check, add:

```go
	if cfg.UvVersion != "0.12.7" {
		t.Fatalf("UvVersion = %q, want 0.12.7", cfg.UvVersion)
	}
	if cfg.PythonVersion != "3.13" {
		t.Fatalf("PythonVersion = %q, want 3.13", cfg.PythonVersion)
	}
```

In `TestLoadDiskAndNodeEnvOverrides` set:

```go
	t.Setenv("CELL_UV_VERSION", "0.12.0")
	t.Setenv("CELL_PYTHON_VERSION", "3.12")
```

and assert `cfg.UvVersion == "0.12.0"` and `cfg.PythonVersion == "3.12"`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -count=1`

Expected: FAIL on `UvVersion` / `PythonVersion` (zero value / missing field).

- [ ] **Step 3: Implement config**

Add fields next to `NodeVersion`:

```go
	UvVersion     string `mapstructure:"uv_version"`
	PythonVersion string `mapstructure:"python_version"`
```

In `Default()`: `UvVersion: "0.12.7"`, `PythonVersion: "3.13"`.

In `Load()` keys append `"uv_version", "python_version"`. SetDefault both. After unmarshal, if either is `""`, fill from `def` like `NodeVersion`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -count=1` then `go test ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/cell_config.go internal/config/cell_config_test.go
git commit -m "Add CELL_UV_VERSION and CELL_PYTHON_VERSION config pins."
```

### Task 2: Install uv + CPython in rootfs

**Files:**
- Modify: `internal/bootstrap/bootstrap.go`
- Modify: `internal/bootstrap/guest_git_sudo_test.go`
- Modify: `internal/bootstrap/bootstrap_behavior_test.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: `cfg.UvVersion`, `cfg.PythonVersion` from Task 1
- Produces:
  - `func uvDownloadTarget() (string, error)` — `x86_64` / `aarch64`
  - `func uvTarballURL(version, arch string) string`
  - `func installUvHostSide(root, imagesDir, version string) error`
  - `func guestPythonInstallScript(pythonVersion string) string`
  - `squashfsBuildStamp` returns `debootstrap:noble+apt+node:{node}+uv:{uv}+py:{py}` with the same empty-string defaults as Node (`v24.20.0`, `0.12.7`, `3.13`)

- [ ] **Step 1: Write the failing tests**

`uvTarballURL("0.12.7", "x86_64")` must equal
`https://github.com/astral-sh/uv/releases/download/0.12.7/uv-x86_64-unknown-linux-gnu.tar.gz`

`squashfsBuildStamp` with Node `v24.20.0`, Uv `0.12.7`, Python `3.13` must equal
`debootstrap:noble+apt+node:v24.20.0+uv:0.12.7+py:3.13`

Update `TestSquashfsBuildStampUsesNodePin`: with only `NodeVersion: "v24.18.0"` (Uv/Python empty) stamp is
`debootstrap:noble+apt+node:v24.18.0+uv:0.12.7+py:3.13`

`guestCustomizeScript()` must contain `command -v uv` and `command -v python3`.

`guestPythonInstallScript("3.13")` must contain
`UV_PYTHON_INSTALL_DIR=/usr/local/share/uv/python`,
`UV_PYTHON_BIN_DIR=/usr/local/bin`,
and `uv python install --default 3.13`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/bootstrap/ -count=1`

Expected: FAIL (undefined funcs and old stamp strings).

- [ ] **Step 3: Implement**

Mirror `installNodeHostSide`: curl cache `uv-{version}-{arch}.tar.gz`, extract to a temp dir, find a file named `uv` (not a directory), `install -m 755` to `{root}/usr/local/bin/uv`. If extract fails, delete the cache file.

In `buildRootfs`, after `installNodeHostSide` and before superpowers:

```go
	if err := installUvHostSide(root, imagesDir, cfg.UvVersion); err != nil {
		return err
	}
	fmt.Println("chroot: uv python install…")
	if err := runChroot(root, "/bin/bash", "-c", guestPythonInstallScript(cfg.PythonVersion)); err != nil {
		return fmt.Errorf("chroot uv python: %w", err)
	}
```

If `cfg.PythonVersion == ""` inside `guestPythonInstallScript`, use `3.13`.

README: add `CELL_UV_VERSION` / `CELL_PYTHON_VERSION` to the runtime table. In the guest-rootfs paragraph add that uv 0.12.7 and CPython 3.13 are installed to `/usr/local`.

- [ ] **Step 4: Run tests and rebuild**

Run: `go test ./...` then `go build -o cell ./cmd/cell`

Expected: PASS, binary built.

- [ ] **Step 5: Commit**

```bash
git add internal/bootstrap/bootstrap.go internal/bootstrap/guest_git_sudo_test.go internal/bootstrap/bootstrap_behavior_test.go README.md
git commit -m "Install pinned uv and CPython 3.13 into the guest rootfs."
```
