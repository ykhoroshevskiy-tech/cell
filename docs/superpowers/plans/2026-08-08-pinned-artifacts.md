# Pinned Bootstrap Artifacts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `cell bootstrap` download a hardcoded known-good artifact stack by default, overridable via `CELL_*` env vars, with no floating “latest” resolution.

**Architecture:** Add four pin fields to `CellConfig`. Change `resolveArtifacts` to build S3/GitHub URLs from those pins only (no S3 list, no GitHub latest). Wire `Ensure` to pass pins from config; keep path overrides as skip-download when the configured path already exists and `--force` is off.

**Tech Stack:** Go 1.26, existing `internal/config` (viper/`CELL_*`), `internal/bootstrap`.

## Global Constraints

- Defaults (verbatim from spec): `ci_prefix=firecracker-ci/20260708-f11c230ed107-0/`, `kernel_version=6.1.176`, `firecracker_version=v1.16.1`, `squashfs_version=24.04`
- Hot-path bootstrap must not call S3 listing or GitHub `/releases/latest`
- No auto-fallback to latest; HTTP failure must surface pin values
- Out of scope: `CELL_ARTIFACTS=latest`, stack YAML, compatibility validation
- Use `uv` is irrelevant here (Go project); do not add new Go dependencies

## File Structure

| File | Role |
|------|------|
| `internal/config/cell_config.go` | Pin fields, defaults, env bind |
| `internal/config/cell_config_test.go` | Env override + default pin tests |
| `internal/bootstrap/versions.go` | Pin-based URL construction; leave sort helpers for their unit tests only |
| `internal/bootstrap/versions_test.go` | Offline URL tests; remove/gate network resolve test |
| `internal/bootstrap/bootstrap.go` | Pass pins into resolve; path-wins skip; wrap download errors with pins |
| `README.md` | Document the four pin env vars |

---

### Task 1: Config pin fields

**Files:**
- Modify: `internal/config/cell_config.go`
- Modify: `internal/config/cell_config_test.go`

**Interfaces:**
- Produces: `CellConfig` fields `CIPrefix`, `KernelVersion`, `FirecrackerVersion`, `SquashfsVersion` (mapstructure `ci_prefix`, `kernel_version`, `firecracker_version`, `squashfs_version`); env `CELL_CI_PREFIX`, `CELL_KERNEL_VERSION`, `CELL_FIRECRACKER_VERSION`, `CELL_SQUASHFS_VERSION`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/cell_config_test.go`:

```go
func TestLoadArtifactPinDefaults(t *testing.T) {
	t.Setenv("CELL_CI_PREFIX", "")
	t.Setenv("CELL_KERNEL_VERSION", "")
	t.Setenv("CELL_FIRECRACKER_VERSION", "")
	t.Setenv("CELL_SQUASHFS_VERSION", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CIPrefix != "firecracker-ci/20260708-f11c230ed107-0/" {
		t.Fatalf("CIPrefix = %q", cfg.CIPrefix)
	}
	if cfg.KernelVersion != "6.1.176" {
		t.Fatalf("KernelVersion = %q", cfg.KernelVersion)
	}
	if cfg.FirecrackerVersion != "v1.16.1" {
		t.Fatalf("FirecrackerVersion = %q", cfg.FirecrackerVersion)
	}
	if cfg.SquashfsVersion != "24.04" {
		t.Fatalf("SquashfsVersion = %q", cfg.SquashfsVersion)
	}
}

func TestLoadArtifactPinEnvOverrides(t *testing.T) {
	t.Setenv("CELL_CI_PREFIX", "firecracker-ci/20260624-ce269725504a-0/")
	t.Setenv("CELL_KERNEL_VERSION", "6.1.174")
	t.Setenv("CELL_FIRECRACKER_VERSION", "v1.16.0")
	t.Setenv("CELL_SQUASHFS_VERSION", "22.04")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CIPrefix != "firecracker-ci/20260624-ce269725504a-0/" {
		t.Fatalf("CIPrefix = %q", cfg.CIPrefix)
	}
	if cfg.KernelVersion != "6.1.174" {
		t.Fatalf("KernelVersion = %q", cfg.KernelVersion)
	}
	if cfg.FirecrackerVersion != "v1.16.0" {
		t.Fatalf("FirecrackerVersion = %q", cfg.FirecrackerVersion)
	}
	if cfg.SquashfsVersion != "22.04" {
		t.Fatalf("SquashfsVersion = %q", cfg.SquashfsVersion)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run 'TestLoadArtifactPin' -v`

Expected: FAIL (unknown fields / empty values)

- [ ] **Step 3: Implement pin fields**

In `CellConfig` add:

```go
CIPrefix           string `mapstructure:"ci_prefix"`
KernelVersion      string `mapstructure:"kernel_version"`
FirecrackerVersion string `mapstructure:"firecracker_version"`
SquashfsVersion    string `mapstructure:"squashfs_version"`
```

In `Default()` set:

```go
CIPrefix:           "firecracker-ci/20260708-f11c230ed107-0/",
KernelVersion:      "6.1.176",
FirecrackerVersion: "v1.16.1",
SquashfsVersion:    "24.04",
```

Also update default `SquashfsPath` to `filepath.Join(imagesDir, "ubuntu-24.04.squashfs")` so it matches the pin (and fix `TestLoadDerivesArtifactPathsFromImagesDir` expectation to `ubuntu-24.04.squashfs`).

In `Load()`:
- Add the four keys to the `keys` slice
- `SetDefault` for each from `def`
- After unmarshal, if any pin string is empty, fill from `def` (same pattern as `RuntimeRoot` / `DataDir`)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`

Expected: PASS (including updated squashfs path expectation)

- [ ] **Step 5: Commit**

```bash
git add internal/config/cell_config.go internal/config/cell_config_test.go
git commit -m "$(cat <<'EOF'
Add CELL_* pin fields for bootstrap artifacts

Hardcode known-good CI prefix, kernel, Firecracker, and squashfs
versions with env overrides for reproducible bootstrap.
EOF
)"
```

---

### Task 2: Pin-based `resolveArtifacts` (offline)

**Files:**
- Modify: `internal/bootstrap/versions.go`
- Modify: `internal/bootstrap/versions_test.go`

**Interfaces:**
- Consumes: pin strings (same semantics as config fields)
- Produces:

```go
type ArtifactPins struct {
	CIPrefix           string
	KernelVersion      string
	FirecrackerVersion string
	SquashfsVersion    string
}

func resolveArtifacts(arch string, pins ArtifactPins) (Artifact, Artifact, Artifact, error)
```

- [ ] **Step 1: Write the failing test**

Replace network `TestResolveArtifacts` with an offline test in `versions_test.go`:

```go
func TestResolveArtifactsFromPins(t *testing.T) {
	pins := ArtifactPins{
		CIPrefix:           "firecracker-ci/20260708-f11c230ed107-0/",
		KernelVersion:      "6.1.176",
		FirecrackerVersion: "v1.16.1",
		SquashfsVersion:    "24.04",
	}
	kernel, fc, squash, err := resolveArtifacts("x86_64", pins)
	if err != nil {
		t.Fatalf("resolveArtifacts: %v", err)
	}
	wantKernel := "https://s3.amazonaws.com/spec.ccfc.min/firecracker-ci/20260708-f11c230ed107-0/x86_64/vmlinux-6.1.176"
	wantFC := "https://github.com/firecracker-microvm/firecracker/releases/download/v1.16.1/firecracker-v1.16.1-x86_64.tgz"
	wantSQ := "https://s3.amazonaws.com/spec.ccfc.min/firecracker-ci/20260708-f11c230ed107-0/x86_64/ubuntu-24.04.squashfs"
	if kernel.URL != wantKernel || kernel.Version != "6.1.176" {
		t.Fatalf("kernel = %+v", kernel)
	}
	if fc.URL != wantFC || fc.Version != "v1.16.1" {
		t.Fatalf("firecracker = %+v", fc)
	}
	if squash.URL != wantSQ || squash.Version != "24.04" {
		t.Fatalf("squashfs = %+v", squash)
	}
}

func TestResolveArtifactsRejectsEmptyPin(t *testing.T) {
	_, _, _, err := resolveArtifacts("x86_64", ArtifactPins{
		CIPrefix: "firecracker-ci/x/", KernelVersion: "6.1.176",
		FirecrackerVersion: "v1.16.1", // SquashfsVersion empty
	})
	if err == nil {
		t.Fatal("expected error for empty squashfs pin")
	}
}
```

Keep `TestSelectLatestCIPrefix` and `TestSelectLatestStablePrefixFallback` as-is (helpers remain for now; unused by bootstrap hot path).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/bootstrap/ -run 'TestResolveArtifacts' -v`

Expected: FAIL (wrong signature / still hits network / compile error)

- [ ] **Step 3: Implement pin-based resolve**

In `versions.go`:

```go
type ArtifactPins struct {
	CIPrefix           string
	KernelVersion      string
	FirecrackerVersion string
	SquashfsVersion    string
}

func resolveArtifacts(arch string, pins ArtifactPins) (Artifact, Artifact, Artifact, error) {
	if arch == "" {
		return Artifact{}, Artifact{}, Artifact{}, fmt.Errorf("arch is empty")
	}
	if pins.CIPrefix == "" || pins.KernelVersion == "" || pins.FirecrackerVersion == "" || pins.SquashfsVersion == "" {
		return Artifact{}, Artifact{}, Artifact{}, fmt.Errorf("incomplete artifact pins: ci_prefix=%q kernel=%q firecracker=%q squashfs=%q",
			pins.CIPrefix, pins.KernelVersion, pins.FirecrackerVersion, pins.SquashfsVersion)
	}
	prefix := pins.CIPrefix
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	kernel := Artifact{
		Name:    "kernel",
		Version: pins.KernelVersion,
		URL:     fmt.Sprintf("%s/%s%s/vmlinux-%s", specS3Base, prefix, arch, pins.KernelVersion),
	}
	firecracker := Artifact{
		Name:    "firecracker",
		Version: pins.FirecrackerVersion,
		URL:     fmt.Sprintf("https://github.com/firecracker-microvm/firecracker/releases/download/%s/firecracker-%s-%s.tgz",
			pins.FirecrackerVersion, pins.FirecrackerVersion, arch),
	}
	squashfs := Artifact{
		Name:    "ubuntu-squashfs",
		Version: pins.SquashfsVersion,
		URL:     fmt.Sprintf("%s/%s%s/ubuntu-%s.squashfs", specS3Base, prefix, arch, pins.SquashfsVersion),
	}
	return kernel, firecracker, squashfs, nil
}
```

Remove use of `listS3Prefixes` / `latestFirecrackerReleaseTag` from this function. Keep those helpers in the file for now (still covered by prefix unit tests); do not call them from `Ensure`.

Delete or rename away the old network `TestResolveArtifacts` that called `resolveArtifacts("x86_64")` with one argument.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/bootstrap/ -run 'TestResolveArtifacts|TestSelectLatest' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bootstrap/versions.go internal/bootstrap/versions_test.go
git commit -m "$(cat <<'EOF'
Resolve bootstrap artifacts from explicit pins

Build kernel/squashfs/Firecracker URLs from pinned versions instead of
listing S3 or following GitHub latest.
EOF
)"
```

---

### Task 3: Wire `Ensure` + path-wins + pin errors

**Files:**
- Modify: `internal/bootstrap/bootstrap.go`

**Interfaces:**
- Consumes: `resolveArtifacts(arch, ArtifactPins{... from cfg})`
- Path-wins: if `!force` and path is a non-empty file, skip that artifact’s download/extract

- [ ] **Step 1: Confirm default pin URLs still exist**

Run:

```bash
curl -sI "https://s3.amazonaws.com/spec.ccfc.min/firecracker-ci/20260708-f11c230ed107-0/x86_64/vmlinux-6.1.176" | head -1
curl -sI "https://s3.amazonaws.com/spec.ccfc.min/firecracker-ci/20260708-f11c230ed107-0/x86_64/ubuntu-24.04.squashfs" | head -1
curl -sI "https://github.com/firecracker-microvm/firecracker/releases/download/v1.16.1/firecracker-v1.16.1-x86_64.tgz" | head -1
```

Expected: HTTP 200 (or 302 for GitHub). If any hard-fail (404), stop and update the four default pins in `config.Default()` + tests + this plan’s Global Constraints before continuing.

- [ ] **Step 2: Wire pins into `Ensure`**

Replace:

```go
kernelArt, fcArt, sqArt, err := resolveArtifacts(arch)
```

with:

```go
pins := ArtifactPins{
	CIPrefix:           cfg.CIPrefix,
	KernelVersion:      cfg.KernelVersion,
	FirecrackerVersion: cfg.FirecrackerVersion,
	SquashfsVersion:    cfg.SquashfsVersion,
}
kernelArt, fcArt, sqArt, err := resolveArtifacts(arch, pins)
if err != nil {
	return err
}
```

- [ ] **Step 3: Path-wins skips**

Add a small helper in `bootstrap.go` (or reuse inline):

```go
func artifactReady(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Size() > 0
}
```

For kernel: if `!force && artifactReady(cfg.KernelPath)` → skip download/validate/symlink for kernel; else existing download + `ValidateKernel` + symlink flow.

For firecracker: if `!force && artifactReady(cfg.FirecrackerBin)` → skip tgz download/extract/symlink; else existing flow.

For squashfs: if `!force && artifactReady(cfg.SquashfsPath)` → keep using `cfg.SquashfsPath` (do not overwrite with versioned path); else download to versioned path and set `cfg.SquashfsPath = sqVersioned` as today.

- [ ] **Step 4: Wrap download errors with pins**

When kernel/fc/squash download returns an error, wrap:

```go
return fmt.Errorf("download %s failed (ci_prefix=%s kernel=%s firecracker=%s squashfs=%s): %w",
	art.Name, pins.CIPrefix, pins.KernelVersion, pins.FirecrackerVersion, pins.SquashfsVersion, err)
```

(apply per failing download call site)

- [ ] **Step 5: Compile + unit tests**

Run: `go test ./internal/bootstrap/ ./internal/config/ ./...`

Expected: PASS (no network required for unit tests)

- [ ] **Step 6: Commit**

```bash
git add internal/bootstrap/bootstrap.go
git commit -m "$(cat <<'EOF'
Wire pinned artifacts into bootstrap Ensure

Pass config pins into resolveArtifacts, skip downloads when path
overrides already exist, and include pins in download errors.
EOF
)"
```

---

### Task 4: README + full verify

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Document pins in Configuration**

Replace/extend the Configuration table so it includes:

```markdown
Artifact pins (defaults are fixed for reproducible bootstrap; override to change the stack):

| Setting | Default |
|---------|---------|
| `CELL_CI_PREFIX` | `firecracker-ci/20260708-f11c230ed107-0/` |
| `CELL_KERNEL_VERSION` | `6.1.176` |
| `CELL_FIRECRACKER_VERSION` | `v1.16.1` |
| `CELL_SQUASHFS_VERSION` | `24.04` |
```

Keep the existing useful runtime vars (`CELL_DATA_DIR`, etc.). One short sentence: path overrides (`CELL_KERNEL_PATH`, `CELL_FIRECRACKER_BIN`, `CELL_SQUASHFS_PATH`) still win when the file already exists.

- [ ] **Step 2: Full test + build**

Run:

```bash
go test ./...
go build -o /tmp/cell-build-check ./cmd/cell
```

Expected: all tests PASS; build succeeds

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "$(cat <<'EOF'
Document pinned artifact CELL_* variables in README
EOF
)"
```

---

## Spec coverage (self-review)

| Spec requirement | Task |
|------------------|------|
| Four pin fields + defaults + env | Task 1 |
| resolve from pins only (no S3/GitHub latest in hot path) | Task 2–3 |
| URL shapes | Task 2 |
| Path wins when file exists | Task 3 |
| Errors include pins | Task 3 |
| Confirm URLs 200 before shipping defaults | Task 3 Step 1 |
| Unit tests offline + config tests | Task 1–2 |
| Remove/gate network resolve test | Task 2 |
| README | Task 4 |
| Out of scope latest escape / YAML / compat | not planned |
