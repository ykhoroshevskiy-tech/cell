# Launch `--config` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let `cell launch --config <json>` inject a host OpenCode JSON file into the guest as `~/.config/opencode/opencode.json` without persisting it in `session.json` or the host repo.

**Architecture:** CLI resolves and validates the path, stores it on `SessionRecord.AgentConfigPath` (`json:"-"`). `buildDisk` copies it to `.filter/opencode.json` on the project disk. Guest init copies that file into agent home, falling back to today's default. No `.filter` rename.

**Tech Stack:** Go 1.26, cobra, `guestinit/guest-entry.sh`, existing `go test` packages `internal/session` and `guestinit`.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-09-04-launch-config-design.md`
- Config is ephemeral: `AgentConfigPath` must use `json:"-"` and must not appear in `session.json`.
- Guest target is global agent config only (`~/.config/opencode/opencode.json` via `.filter/agent-home`). Do not write `/project/opencode.json`.
- Keep the directory name `.filter`. Do not rename to `.cell`.
- Invalid JSON: warning to stderr, launch continues, file is still copied.
- Missing path or directory: `launch` returns an error.
- `--config` omitted: unchanged behavior (no `.filter/opencode.json` on disk).
- Do not git commit (user commits later).
- Tests: `GOCACHE=/tmp/cell-gocache go test ./internal/session ./guestinit ./internal/cli`
- Rebuild after Go changes: `GOCACHE=/tmp/cell-gocache go build -o cell ./cmd/cell` from repo root.
- Ponytail: no extra abstractions. Follow existing test style (`t.Fatal`, no testify). `CopyPasswordToDiskRoot` is the pattern for disk copies.
- Do not add e2e / KVM tests.

---

### Task 1: ResolveAgentConfig

**Files:**
- Create: `internal/session/agentconfig.go`
- Create: `internal/session/agentconfig_test.go`

**Interfaces:**
- Produces: `func ResolveAgentConfig(path string) (abs string, warn string, err error)`
- Produces: `const GuestAgentConfigRel = ".filter/opencode.json"` (used in Task 2)
- Empty path → error (CLI will not call this when the flag is omitted).
- Success: `filepath.Abs` + `filepath.Clean`, must be a regular file.
- Invalid JSON: `err == nil`, `abs` set, `warn` is `warning: --config is not valid JSON: <abs>`.
- Valid JSON: `warn == ""`.

- [ ] **Step 1: Write the failing tests** in `internal/session/agentconfig_test.go`:

```go
package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveAgentConfigEmpty(t *testing.T) {
	if _, _, err := ResolveAgentConfig(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveAgentConfigMissing(t *testing.T) {
	_, _, err := ResolveAgentConfig(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveAgentConfigDir(t *testing.T) {
	dir := t.TempDir()
	_, _, err := ResolveAgentConfig(dir)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not a file") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveAgentConfigRelativeBecomesAbs(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "opencode.json")
	if err := os.WriteFile(f, []byte(`{"model":"x"}`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	abs, warn, err := ResolveAgentConfig("./opencode.json")
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Fatalf("warn=%q", warn)
	}
	want, err := filepath.Abs(f)
	if err != nil {
		t.Fatal(err)
	}
	if abs != filepath.Clean(want) {
		t.Fatalf("got %q want %q", abs, want)
	}
	if !filepath.IsAbs(abs) {
		t.Fatalf("not abs: %q", abs)
	}
}

func TestResolveAgentConfigInvalidJSONWarns(t *testing.T) {
	f := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(f, []byte("{"), 0644); err != nil {
		t.Fatal(err)
	}
	abs, warn, err := ResolveAgentConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if abs == "" {
		t.Fatal("expected abs path")
	}
	if !strings.Contains(warn, "not valid JSON") {
		t.Fatalf("warn=%q", warn)
	}
	if !strings.Contains(warn, abs) {
		t.Fatalf("warn missing path: %q", warn)
	}
}

func TestResolveAgentConfigValidJSONNoWarn(t *testing.T) {
	f := filepath.Join(t.TempDir(), "ok.json")
	body := []byte(`{"$schema":"https://opencode.ai/config.json"}`)
	if !json.Valid(body) {
		t.Fatal("fixture")
	}
	if err := os.WriteFile(f, body, 0644); err != nil {
		t.Fatal(err)
	}
	_, warn, err := ResolveAgentConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Fatalf("warn=%q", warn)
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL** (undefined `ResolveAgentConfig`)

```sh
GOCACHE=/tmp/cell-gocache go test ./internal/session -run TestResolveAgentConfig -v
```

- [ ] **Step 3: Implement** `internal/session/agentconfig.go`:

```go
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const GuestAgentConfigRel = ".filter/opencode.json"

func ResolveAgentConfig(path string) (string, string, error) {
	if path == "" {
		return "", "", fmt.Errorf("agent config path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	abs = filepath.Clean(abs)
	fi, err := os.Stat(abs)
	if err != nil {
		return "", "", err
	}
	if !fi.Mode().IsRegular() {
		return "", "", fmt.Errorf("agent config is not a file: %s", abs)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", "", err
	}
	warn := ""
	if !json.Valid(data) {
		warn = fmt.Sprintf("warning: --config is not valid JSON: %s", abs)
	}
	return abs, warn, nil
}
```

- [ ] **Step 4: Re-run tests — expect PASS**

```sh
GOCACHE=/tmp/cell-gocache go test ./internal/session -run TestResolveAgentConfig -v
```

- [ ] **Step 5: Do not commit**

---

### Task 2: Copy onto project disk and wire launch

**Files:**
- Modify: `internal/session/agentconfig.go` (add `CopyAgentConfigToDiskRoot`)
- Modify: `internal/session/agentconfig_test.go`
- Modify: `internal/models/session.go` (`AgentConfigPath` with `json:"-"`)
- Modify: `internal/session/lifecycle.go` (`Launch` signature + `buildDisk`)
- Modify: `internal/cli/launch.go` (`--config` flag)

**Interfaces:**
- Consumes: `ResolveAgentConfig`, `GuestAgentConfigRel`
- Produces: `func CopyAgentConfigToDiskRoot(src, diskRoot string) error`
- Produces: `SessionRecord.AgentConfigPath string` with tag `json:"-"`
- Produces: `func (sm *SessionManager) Launch(ctx context.Context, repoPath, agentConfigPath string, attach bool) (*models.SessionRecord, error)`
- Empty `src` / empty `agentConfigPath`: copy is a no-op (no dest file).
- Dest: `{diskRoot}/.filter/opencode.json`, mode `0600` (mkdir `.filter` `0700`, chmod after write like `CopyPasswordToDiskRoot`).
- `buildDisk` calls `CopyAgentConfigToDiskRoot(session.AgentConfigPath, rootDir)` after `WriteServePortToDiskRoot`.
- `Launch` sets `session.AgentConfigPath = agentConfigPath` after `prepareSession` succeeds, before `buildDisk`.
- CLI: `--config` optional string. If set, call `ResolveAgentConfig`; on `err` return it; on `warn != ""` print warn to `cmd.ErrOrStderr()`; pass `abs` into `Launch`. If unset, pass `""`.

- [ ] **Step 1: Write the failing tests** — append to `internal/session/agentconfig_test.go`:

```go
func TestCopyAgentConfigToDiskRootEmpty(t *testing.T) {
	disk := t.TempDir()
	if err := CopyAgentConfigToDiskRoot("", disk); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(disk, GuestAgentConfigRel)); !os.IsNotExist(err) {
		t.Fatalf("unexpected dest: %v", err)
	}
}

func TestCopyAgentConfigToDiskRoot(t *testing.T) {
	src := filepath.Join(t.TempDir(), "opencode.json")
	body := []byte(`{"model":"x"}`)
	if err := os.WriteFile(src, body, 0644); err != nil {
		t.Fatal(err)
	}
	disk := t.TempDir()
	if err := CopyAgentConfigToDiskRoot(src, disk); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(disk, GuestAgentConfigRel)
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("got %q want %q", got, body)
	}
	st, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatalf("perm = %o, want 0600", st.Mode().Perm())
	}
}

func TestCopyAgentConfigToDiskRootMissing(t *testing.T) {
	err := CopyAgentConfigToDiskRoot(filepath.Join(t.TempDir(), "nope.json"), t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAgentConfigPathNotSerialized(t *testing.T) {
	s := models.SessionRecord{
		SessionID:       "abc",
		AgentConfigPath: "/tmp/secret-opencode.json",
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-opencode") || strings.Contains(string(data), "AgentConfig") {
		t.Fatalf("leaked: %s", data)
	}
}
```

Add `"github.com/ykhoroshevskiy-tech/cell/internal/models"` to the test imports.

- [ ] **Step 2: Run — expect FAIL** (undefined `CopyAgentConfigToDiskRoot` / missing field)

```sh
GOCACHE=/tmp/cell-gocache go test ./internal/session -run 'TestCopyAgentConfig|TestAgentConfigPath' -v
```

- [ ] **Step 3: Implement**

Append to `internal/session/agentconfig.go`:

```go
func CopyAgentConfigToDiskRoot(src, diskRoot string) error {
	if src == "" {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	dst := filepath.Join(diskRoot, GuestAgentConfigRel)
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0600); err != nil {
		return err
	}
	return os.Chmod(dst, 0600)
}
```

In `internal/models/session.go`, add this field to `SessionRecord` (next to `Error` is fine):

```go
	AgentConfigPath  string         `json:"-"`
```

In `internal/session/lifecycle.go`, change `Launch` to:

```go
func (sm *SessionManager) Launch(ctx context.Context, repoPath, agentConfigPath string, attach bool) (*models.SessionRecord, error) {
	verbose.V("launch: preparing session for %s", repoPath)
	session, err := sm.prepareSession(repoPath)
	if err != nil {
		return nil, err
	}
	session.AgentConfigPath = agentConfigPath
```

(rest of `Launch` unchanged).

In `buildDisk`, after `WriteServePortToDiskRoot`:

```go
	if err := CopyAgentConfigToDiskRoot(session.AgentConfigPath, rootDir); err != nil {
		return err
	}
```

In `internal/cli/launch.go`, add a `configPath` string flag `--config` with usage `OpenCode JSON for guest ~/.config/opencode/opencode.json`. Inside `RunE`, after `--repo` is known and before `sm.Launch`:

```go
			agentConfig := ""
			if configPath != "" {
				abs, warn, err := session.ResolveAgentConfig(configPath)
				if err != nil {
					return err
				}
				if warn != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), warn)
				}
				agentConfig = abs
			}
```

Call `sm.Launch(ctx, repo, agentConfig, !noAttach)`.

- [ ] **Step 4: Tests pass + build**

```sh
GOCACHE=/tmp/cell-gocache go test ./internal/session ./internal/cli
GOCACHE=/tmp/cell-gocache go build -o cell ./cmd/cell
```

- [ ] **Step 5: Do not commit**

---

### Task 3: Guest copy + README

**Files:**
- Modify: `guestinit/guest-entry.sh` (`setup_agent_home`)
- Modify: `guestinit/guest_entry_test.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: host file at `${MOUNT}/.filter/opencode.json`
- Writes: `${RW}/.config/opencode/opencode.json` (`RW` is `${MOUNT}/.filter/agent-home`)
- If host file exists and `cp` succeeds, that file wins (including over a previously persisted agent-home config).
- If host file exists and `cp` fails: `log "WARN: failed to copy host opencode.json; using default"` then fall through to the existing default-if-missing block.
- If host file is absent: existing default-if-missing behavior unchanged.
- README: document `--config` on `launch`. Do not mention renaming `.filter`.

- [ ] **Step 1: Write the failing test** — add these strings to the `want` list in `TestGuestEntryIgnoresFilterInGitignore` (keep the existing entries):

```go
		".filter/opencode.json",
		"WARN: failed to copy host opencode.json; using default",
```

- [ ] **Step 2: Run — expect FAIL** (strings missing)

```sh
GOCACHE=/tmp/cell-gocache go test ./guestinit -run TestGuestEntryIgnoresFilterInGitignore -v
```

- [ ] **Step 3: Implement** in `setup_agent_home`, after `CFG=...` and **before** the existing `if [ ! -f "${CFG}" ]` default block:

```sh
  if [ -f "${MOUNT}/.filter/opencode.json" ]; then
    if cp "${MOUNT}/.filter/opencode.json" "${CFG}"; then
      log "agent config from host .filter/opencode.json"
    else
      log "WARN: failed to copy host opencode.json; using default"
    fi
  fi
```

Leave the default `cat > "${CFG}"` block as-is after this.

In `README.md`:
- Quick start: add one example line `sudo cell launch --repo /path/to/your/repo --config /path/to/opencode.json`
- In "How it works" step 1 or 5, one sentence: optional `--config` is copied to `.filter/opencode.json` and loaded as the guest agent config (`~/.config/opencode/opencode.json`); it is not stored in `session.json` and is skipped by pull.

- [ ] **Step 4: Tests pass + rebuild**

```sh
GOCACHE=/tmp/cell-gocache go test ./internal/session ./guestinit ./internal/cli
GOCACHE=/tmp/cell-gocache go build -o cell ./cmd/cell
```

- [ ] **Step 5: Do not commit**
