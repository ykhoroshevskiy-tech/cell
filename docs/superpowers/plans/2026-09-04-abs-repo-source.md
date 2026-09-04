# Absolute repo_source Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist launch `--repo` as an absolute directory and refuse relative pull destinations so rsync never follows cwd.

**Architecture:** One helper at session create (`ResolveRepoSource`). Pull checks `filepath.IsAbs` after dest is chosen. No Abs on load/pull.

**Tech Stack:** Go 1.26, existing `go test` packages `internal/session` and `internal/sync`.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-09-04-abs-repo-source-design.md`
- Do not call `filepath.Abs` in pull or `loadSession`.
- Do not auto-migrate existing `session.json`.
- Do not `EvalSymlinks`.
- Do not add a `cell doctor` command.
- Do not git commit (user commits later).
- Tests: `GOCACHE=/tmp/cell-gocache go test ./internal/session ./internal/sync`
- Rebuild after Go changes: `GOCACHE=/tmp/cell-gocache go build -o cell ./cmd/cell` from repo root.
- Ponytail: no extra abstractions. Follow existing test style (`t.Fatal`, no testify).

---

### Task 1: ResolveRepoSource at launch

**Files:**
- Create: `internal/session/repopath.go`
- Create: `internal/session/repopath_test.go`
- Modify: `internal/session/lifecycle.go` (`prepareSession`)

**Interfaces:**
- Produces: `func ResolveRepoSource(path string) (string, error)`

- [ ] **Step 1: Write the failing tests** in `internal/session/repopath_test.go`:

```go
package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRepoSourceEmpty(t *testing.T) {
	if _, err := ResolveRepoSource(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveRepoSourceRelativeBecomesAbs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "repo")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	got, err := ResolveRepoSource("./repo")
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(want) {
		t.Fatalf("got %q want %q", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("not abs: %q", got)
	}
}

func TestResolveRepoSourceMissing(t *testing.T) {
	_, err := ResolveRepoSource(filepath.Join(t.TempDir(), "nope"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveRepoSourceNotDir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveRepoSource(f)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not a directory") && !strings.Contains(err.Error(), "directory") {
		t.Fatalf("err=%v", err)
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL** (undefined `ResolveRepoSource`)

```sh
GOCACHE=/tmp/cell-gocache go test ./internal/session -run TestResolveRepoSource -v
```

- [ ] **Step 3: Implement** `internal/session/repopath.go`:

```go
package session

import (
	"fmt"
	"os"
	"path/filepath"
)

func ResolveRepoSource(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("repo path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	fi, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("repo path is not a directory: %s", abs)
	}
	return abs, nil
}
```

In `prepareSession`, after kernel/rootfs/firecracker stat checks, before allocating the session:

```go
	resolved, err := ResolveRepoSource(repoSource)
	if err != nil {
		return nil, err
	}
```

Set `RepoSource: resolved` on the `SessionRecord`.

- [ ] **Step 4: Re-run tests — expect PASS**

```sh
GOCACHE=/tmp/cell-gocache go test ./internal/session -run TestResolveRepoSource -v
```

- [ ] **Step 5: Do not commit**

---

### Task 2: Pull rejects relative dest + README

**Files:**
- Modify: `internal/sync/pull.go` (`PullWorkspace`)
- Modify: `internal/sync/pull_test.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: dest from `opts.Dest` or `session.RepoSource`
- After dest is chosen, require `filepath.IsAbs(dest)`

- [ ] **Step 1: Write failing tests** in `internal/sync/pull_test.go`:

```go
func TestValidateDestRelative(t *testing.T) {
	for _, dest := range []string{".", "./", "foo", "rel/path"} {
		if err := sync.ValidateDest(dest); err == nil {
			t.Fatalf("expected error for %q", dest)
		}
	}
}
```

Put the absolute check in `ValidateDest` so every pull path (explicit dest, session default, auto-pull) is covered. Relative dest error must include the dest string. Keep existing unsafe-path checks. `ValidateDest("/tmp/cell-pull")` still succeeds.

If you prefer the check only in `PullWorkspace`, also test via a small exported helper used by both — but spec says the check happens after dest is chosen; `ValidateDest` is already that gate. Use `ValidateDest`.

- [ ] **Step 2: Run — expect FAIL** for `.` / `./` currently accepted

```sh
GOCACHE=/tmp/cell-gocache go test ./internal/sync -run 'TestValidateDest' -v
```

- [ ] **Step 3: Implement** in `ValidateDest` after trim: if `!filepath.IsAbs(clean)` (treat `""` after trim as `/` first, then unsafe map, then IsAbs). Empty/unsafe stay as today. Relative (including `.` and `./`) → `fmt.Errorf("relative pull destination: %s", dest)`.

README: under launch / pull, one sentence: `--repo` is stored as an absolute path; pull dest must be absolute.

- [ ] **Step 4: Tests pass**

```sh
GOCACHE=/tmp/cell-gocache go test ./internal/session ./internal/sync
```

- [ ] **Step 5: Rebuild**

```sh
GOCACHE=/tmp/cell-gocache go build -o cell ./cmd/cell
```

- [ ] **Step 6: Do not commit**
