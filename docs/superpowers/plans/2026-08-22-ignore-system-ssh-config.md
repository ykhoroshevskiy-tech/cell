# Ignore System SSH Config Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Host→guest SSH started by cell ignores system/user ssh_config so SSSD `sss_ssh_knownhosts` cannot hang wait-ready or pull.

**Architecture:** Put `-F /dev/null` first in `sshBaseArgs`. Export `RemoteShell(keyPath)` as `"ssh " + joined sshBaseArgs` and use it as rsync `-e`. One source of flags.

**Tech Stack:** Go, OpenSSH client, existing `go test` packages `internal/ssh` and `internal/sync`.

## Global Constraints

- `sshBaseArgs` first two elements after construction: `-F` then `/dev/null`.
- Keep existing options: `-i` key, `IdentitiesOnly=yes`, `BatchMode=yes`, `ConnectTimeout=5`, `StrictHostKeyChecking=no`, `UserKnownHostsFile=/dev/null`.
- Do not add `KnownHostsCommand=none` as the primary fix.
- `RemoteShell(keyPath string) string` lives in `internal/ssh` and is the rsync `-e` value.
- `internal/sync/pull.go` must not keep a parallel `ssh -i ...` format string.
- Do not change wait-ready logging, ConnectTimeout value, firewall, or e2e.
- After Go edits: `go build -o cell ./cmd/cell` from repo root (do not tell the user to rebuild).
- Commit only the files this task touches (plus the spec/plan docs). Do not stage unrelated dirty files.

---

### Task 1: `-F /dev/null` in sshBaseArgs + RemoteShell for pull

**Files:**
- Modify: `internal/ssh/attach.go`
- Modify: `internal/ssh/attach_test.go`
- Modify: `internal/sync/pull.go`
- Docs already on disk (do not rewrite): `docs/superpowers/specs/2026-08-22-ignore-system-ssh-config-design.md`, `docs/superpowers/plans/2026-08-22-ignore-system-ssh-config.md`

**Interfaces:**
- Consumes: existing unexported `sshBaseArgs(keyPath string) []string`
- Produces: `func RemoteShell(keyPath string) string` — returns `ssh` plus `sshBaseArgs` joined with spaces, e.g. `ssh -F /dev/null -i /tmp/id -o IdentitiesOnly=yes ...`

- [ ] **Step 1: Write the failing tests**

In `internal/ssh/attach_test.go`, insert `-F`, `/dev/null` immediately before `"-i", "/tmp/id"` in `TestTunnelSSHArgs` `want`. Add:

```go
func TestRemoteShell(t *testing.T) {
	got := ssh.RemoteShell("/tmp/id")
	if !strings.HasPrefix(got, "ssh -F /dev/null ") {
		t.Fatalf("RemoteShell=%q", got)
	}
	if !strings.Contains(got, "-i /tmp/id") {
		t.Fatalf("missing key: %q", got)
	}
	if strings.Contains(got, "KnownHostsCommand") {
		t.Fatalf("must not use KnownHostsCommand: %q", got)
	}
}
```

Do not add a sync-package test. `TestRemoteShell` is the contract for rsync `-e`.

If `RemoteShell` is not yet defined, `TestRemoteShell` fails to compile. That is the RED for that test. `TestTunnelSSHArgs` fails on `arg[i]` mismatch for `-F`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ssh -count=1 -run 'TestTunnelSSHArgs|TestRemoteShell'`

Expected: FAIL — `TestRemoteShell` undefined `RemoteShell` and/or `TestTunnelSSHArgs` `arg[N]` want `-F`.

- [ ] **Step 3: Write minimal implementation**

`internal/ssh/attach.go` — `sshBaseArgs` and `RemoteShell`:

```go
func sshBaseArgs(keyPath string) []string {
	return []string{
		"-F", "/dev/null",
		"-i", keyPath,
		"-o", "IdentitiesOnly=yes",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
	}
}

func RemoteShell(keyPath string) string {
	return "ssh " + strings.Join(sshBaseArgs(keyPath), " ")
}
```

`strings` is already imported in `attach.go`.

`internal/sync/pull.go` — import `github.com/ykhoroshevskiy-tech/cell/internal/ssh` and replace the `rsh := fmt.Sprintf(...)` block with:

```go
	rsh := ssh.RemoteShell(session.SSHKeyPath)
```

Remove the unused `fmt.Sprintf` for rsh. Keep `fmt` if still used elsewhere in the file (it is).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ssh ./internal/sync -count=1`

Expected: PASS (all tests in those packages).

Run: `go build -o cell ./cmd/cell`

Expected: exit 0, binary `./cell` updated.

- [ ] **Step 5: Commit**

Stage **only**:

- `internal/ssh/attach.go`
- `internal/ssh/attach_test.go`
- `internal/sync/pull.go`
- `docs/superpowers/specs/2026-08-22-ignore-system-ssh-config-design.md`
- `docs/superpowers/plans/2026-08-22-ignore-system-ssh-config.md`

```bash
git add internal/ssh/attach.go internal/ssh/attach_test.go internal/sync/pull.go docs/superpowers/specs/2026-08-22-ignore-system-ssh-config-design.md docs/superpowers/plans/2026-08-22-ignore-system-ssh-config.md

git commit -m "$(cat <<'EOF'
Ignore host ssh_config for guest SSH so SSSD cannot hang probes.

EOF
)"
```

Do not `git add -A`. Do not commit other dirty network/e2e files.
