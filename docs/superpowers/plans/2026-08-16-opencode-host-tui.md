# OpenCode Host TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run `opencode serve` in the Firecracker guest and attach the host TUI over an SSH local forward, without killing the VM on TUI exit, and with `cell start` to boot the same `project.ext4` after host reboot.

**Architecture:** Guest tmux babysits `opencode serve` on `127.0.0.1:4096`. Host `cell attach` / `launch` opens `ssh -N -L` then `opencode attach`. Password lives in the session dir and on the project disk under `.filter/`. `cell start` reuses the existing disk (no restage/mkfs). TAP/iptables stay as they are.

**Tech Stack:** Go, cobra/viper, OpenSSH local forward, guest `tmux` + `curl`, existing Firecracker session manager.

## Global Constraints

- No TAP/iptables changes; serve binds `127.0.0.1` only.
- No Firecracker in unit tests.
- No new Go dependencies.
- Do not `Stop` the VM when the TUI exits or when host `opencode` is missing after boot.
- Do not rotate the serve password on `start`.
- Host TUI binary is `CELL_HOST_AGENT_BIN` (default `opencode`); cell does not install it.
- `--dir /project` is the path inside the guest.
- Auto-pull runs only while attach is in the foreground.
- `docs/` is globally gitignored; commit files there with `git add -f`.

## File map

| File | Role |
|------|------|
| `internal/config/cell_config.go` | `AgentServePort`, `HostAgentBin`; default `AgentCmd` is serve |
| `internal/models/session.go` | `HostForwardPort` |
| `internal/models/status.go` | `ServerReady` replaces `TmuxReady` |
| `internal/session/password.go` | generate/read `opencode-server.pass` mode 0600 |
| `internal/session/start.go` | `StartPreflight`, `Start`, `Attach` (TUI) |
| `internal/session/lifecycle.go` | password into disk; launch no longer stops VM |
| `internal/ssh/attach.go` | tunnel args, TUI attach, shell attach, health probe |
| `guestinit/guest-entry.sh` | start serve in tmux |
| `guestinit/tmux-attach.sh` | debug attach only (no TUI send-keys) |
| `internal/bootstrap/bootstrap.go` | install/verify `curl` |
| `internal/cli/attach.go` | `cell attach` |
| `internal/cli/start.go` | `cell start` |
| `internal/cli/ssh_cmd.go` | guest shell, not TUI |
| `internal/cli/status.go`, `verify.go` | print `server=` |
| `README.md` | new flow |

---

### Task 1: Config defaults

**Files:**
- Modify: `internal/config/cell_config.go`
- Test: `internal/config/cell_config_test.go`

**Interfaces:**
- Consumes: existing `Load()` / `Default()`
- Produces: `CellConfig.AgentServePort int` (`mapstructure:"agent_serve_port"`), `CellConfig.HostAgentBin string` (`mapstructure:"host_agent_bin"`). Defaults: `AgentCmd = "opencode serve --hostname 127.0.0.1 --port 4096"`, `AgentServePort = 4096`, `HostAgentBin = "opencode"`.

- [ ] **Step 1: Write the failing test**

Add to `internal/config/cell_config_test.go`:

```go
func TestLoadAgentServeDefaults(t *testing.T) {
	t.Setenv("CELL_AGENT_CMD", "")
	t.Setenv("CELL_AGENT_SERVE_PORT", "")
	t.Setenv("CELL_HOST_AGENT_BIN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AgentCmd != "opencode serve --hostname 127.0.0.1 --port 4096" {
		t.Fatalf("AgentCmd = %q", cfg.AgentCmd)
	}
	if cfg.AgentServePort != 4096 {
		t.Fatalf("AgentServePort = %d", cfg.AgentServePort)
	}
	if cfg.HostAgentBin != "opencode" {
		t.Fatalf("HostAgentBin = %q", cfg.HostAgentBin)
	}
}

func TestLoadAgentServeEnvOverrides(t *testing.T) {
	t.Setenv("CELL_AGENT_CMD", "opencode serve --hostname 127.0.0.1 --port 4097")
	t.Setenv("CELL_AGENT_SERVE_PORT", "4097")
	t.Setenv("CELL_HOST_AGENT_BIN", "/usr/local/bin/opencode")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AgentCmd != "opencode serve --hostname 127.0.0.1 --port 4097" {
		t.Fatalf("AgentCmd = %q", cfg.AgentCmd)
	}
	if cfg.AgentServePort != 4097 {
		t.Fatalf("AgentServePort = %d", cfg.AgentServePort)
	}
	if cfg.HostAgentBin != "/usr/local/bin/opencode" {
		t.Fatalf("HostAgentBin = %q", cfg.HostAgentBin)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run 'TestLoadAgentServe' -v`

Expected: FAIL — `AgentCmd` still `opencode --auto`, fields missing.

- [ ] **Step 3: Write minimal implementation**

In `CellConfig` add:

```go
AgentServePort int    `mapstructure:"agent_serve_port"`
HostAgentBin   string `mapstructure:"host_agent_bin"`
```

In `Default()`:

```go
AgentCmd:        "opencode serve --hostname 127.0.0.1 --port 4096",
AgentServePort:  4096,
HostAgentBin:    "opencode",
```

Bind env keys `"agent_serve_port"`, `"host_agent_bin"`. Set viper defaults. After unmarshal, if `AgentServePort == 0` set 4096; if `HostAgentBin == ""` set `"opencode"`. Keep existing empty-`AgentCmd` fill from `def.AgentCmd`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config -run 'TestLoadAgentServe' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/cell_config.go internal/config/cell_config_test.go
git commit -m "$(cat <<'EOF'
Default the in-guest agent to opencode serve.

Host TUI binary and guest serve port are now first-class CELL_* settings.
EOF
)"
```

---

### Task 2: Server password file

**Files:**
- Create: `internal/session/password.go`
- Test: `internal/session/password_test.go`

**Interfaces:**
- Consumes: session directory path
- Produces:
  - `func ServerPasswordPath(sessionDir string) string` → `{sessionDir}/opencode-server.pass`
  - `func WriteServerPassword(sessionDir string) (string, error)` — 32 hex bytes from `crypto/rand`, write mode `0600`, return password
  - `func ReadServerPassword(sessionDir string) (string, error)`
  - `const GuestPasswordRel = ".filter/opencode-server.pass"`

- [ ] **Step 1: Write the failing test**

```go
package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteServerPasswordModeAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	pw, err := WriteServerPassword(dir)
	if err != nil {
		t.Fatalf("WriteServerPassword: %v", err)
	}
	if len(pw) < 16 {
		t.Fatalf("password too short: %q", pw)
	}
	st, err := os.Stat(ServerPasswordPath(dir))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatalf("perm = %o, want 0600", st.Mode().Perm())
	}
	got, err := ReadServerPassword(dir)
	if err != nil {
		t.Fatalf("ReadServerPassword: %v", err)
	}
	if got != pw {
		t.Fatalf("got %q want %q", got, pw)
	}
}

func TestWriteServerPasswordDoesNotRotate(t *testing.T) {
	dir := t.TempDir()
	first, err := WriteServerPassword(dir)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := WriteServerPassword(dir)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first != second {
		t.Fatalf("rotated password")
	}
	_ = filepath.Separator
}
```

`WriteServerPassword` must **not** overwrite an existing file (so `start` / re-launch helpers cannot rotate). If the file exists, read and return it.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/session -run 'TestWriteServerPassword' -v`

Expected: FAIL — undefined functions.

- [ ] **Step 3: Write minimal implementation**

`internal/session/password.go`:

```go
package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const GuestPasswordRel = ".filter/opencode-server.pass"

func ServerPasswordPath(sessionDir string) string {
	return filepath.Join(sessionDir, "opencode-server.pass")
}

func WriteServerPassword(sessionDir string) (string, error) {
	path := ServerPasswordPath(sessionDir)
	if b, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(b)), nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	pw := hex.EncodeToString(buf)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(pw+"\n"), 0600); err != nil {
		return "", err
	}
	return pw, nil
}

func ReadServerPassword(sessionDir string) (string, error) {
	b, err := os.ReadFile(ServerPasswordPath(sessionDir))
	if err != nil {
		return "", fmt.Errorf("read serve password: %w", err)
	}
	pw := strings.TrimSpace(string(b))
	if pw == "" {
		return "", fmt.Errorf("serve password file empty")
	}
	return pw, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/session -run 'TestWriteServerPassword' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/session/password.go internal/session/password_test.go
git commit -m "$(cat <<'EOF'
Store a one-shot OpenCode serve password per session.

Reuse the existing file so start never rotates credentials on disk.
EOF
)"
```

---

### Task 3: Copy password onto the project disk

**Files:**
- Modify: `internal/session/lifecycle.go` (`prepareSession` / `Launch` / `buildDisk`)
- Test: `internal/session/password_test.go` (keep disk-copy helper here)

**Interfaces:**
- Consumes: `WriteServerPassword`, `GuestPasswordRel`
- Produces: `func CopyPasswordToDiskRoot(sessionDir, diskRoot string) error` — copies `{sessionDir}/opencode-server.pass` to `{diskRoot}/.filter/opencode-server.pass` mode 0600. `Launch` calls `WriteServerPassword` after `prepareSession` and `CopyPasswordToDiskRoot` from `buildDisk` after `authorized_keys`.

- [ ] **Step 1: Write the failing test**

```go
func TestCopyPasswordToDiskRoot(t *testing.T) {
	sessionDir := t.TempDir()
	diskRoot := t.TempDir()
	pw, err := WriteServerPassword(sessionDir)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := CopyPasswordToDiskRoot(sessionDir, diskRoot); err != nil {
		t.Fatalf("copy: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(diskRoot, GuestPasswordRel))
	if err != nil {
		t.Fatalf("read guest copy: %v", err)
	}
	if strings.TrimSpace(string(got)) != pw {
		t.Fatalf("guest copy = %q want %q", got, pw)
	}
	st, _ := os.Stat(filepath.Join(diskRoot, GuestPasswordRel))
	if st.Mode().Perm() != 0600 {
		t.Fatalf("guest perm = %o", st.Mode().Perm())
	}
}
```

Add `"strings"` to the test imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/session -run TestCopyPasswordToDiskRoot -v`

Expected: FAIL — `CopyPasswordToDiskRoot` undefined.

- [ ] **Step 3: Write minimal implementation**

In `password.go`:

```go
func CopyPasswordToDiskRoot(sessionDir, diskRoot string) error {
	pw, err := ReadServerPassword(sessionDir)
	if err != nil {
		return err
	}
	dst := filepath.Join(diskRoot, GuestPasswordRel)
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	return os.WriteFile(dst, []byte(pw+"\n"), 0600)
}
```

In `Launch`, after `prepareSession` succeeds:

```go
if _, err := WriteServerPassword(session.SessionDir); err != nil {
	return nil, err
}
```

In `buildDisk`, after writing `authorized_keys`:

```go
if err := CopyPasswordToDiskRoot(session.SessionDir, rootDir); err != nil {
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/session -run 'TestCopyPassword|TestWriteServerPassword' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/session/password.go internal/session/password_test.go internal/session/lifecycle.go
git commit -m "$(cat <<'EOF'
Bake the serve password into the project disk.

Guest init can read it from /project/.filter without a second SSH hop.
EOF
)"
```

---

### Task 4: Health probe + ServerReady

**Files:**
- Modify: `internal/ssh/attach.go`
- Modify: `internal/models/status.go`
- Modify: `internal/session/lifecycle.go` (`List` field)
- Modify: `internal/cli/status.go`, `internal/cli/verify.go`
- Test: `internal/ssh/attach_test.go`

**Interfaces:**
- Consumes: `cfg.AgentServePort`, existing `sshBaseArgs`
- Produces:
  - `func HealthProbeRemote(cfg *config.CellConfig) string` → `curl -sf http://127.0.0.1:{port}/global/health`
  - `func ServerReady(session *models.SessionRecord, cfg *config.CellConfig) bool` — SSH that remote command
  - Rename `TmuxReady` → `ServerReady` on `SessionStatus` and `SessionListEntry` with json `server_ready`
  - `WaitRuntimeReady` waits `RuntimeReady && SSHReachable && ServerReady`

- [ ] **Step 1: Write the failing test**

```go
func TestHealthProbeRemote(t *testing.T) {
	cfg := &config.CellConfig{AgentServePort: 4096}
	got := ssh.HealthProbeRemote(cfg)
	want := "curl -sf http://127.0.0.1:4096/global/health"
	if got != want {
		t.Fatalf("probe = %q want %q", got, want)
	}
}
```

Add import `"github.com/ykhoroshevskiy-tech/cell/internal/config"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ssh -run TestHealthProbeRemote -v`

Expected: FAIL — undefined.

- [ ] **Step 3: Write minimal implementation**

Replace `TmuxReady` with:

```go
func HealthProbeRemote(cfg *config.CellConfig) string {
	return fmt.Sprintf("curl -sf http://127.0.0.1:%d/global/health", cfg.AgentServePort)
}

func ServerReady(session *models.SessionRecord, cfg *config.CellConfig) bool {
	if session.SSHKeyPath == "" || session.NetworkConfig == nil {
		return false
	}
	args := append(sshBaseArgs(session.SSHKeyPath),
		fmt.Sprintf("%s@%s", cfg.SSHUser, session.NetworkConfig.GuestIP),
		HealthProbeRemote(cfg),
	)
	cmd := exec.Command("ssh", args...)
	return cmd.Run() == nil
}
```

In `SessionStatus`, set `st.ServerReady = ServerReady(session, cfg)` when SSH reachable.

In `WaitRuntimeReady`, require `status.ServerReady` instead of `TmuxReady`. Verbose log `server=%v`.

Models:

```go
ServerReady bool `json:"server_ready"`
```

Delete `TmuxReady` fields. Update `lifecycle.go` List mapping, `status.go` printf (`server=%v`), `verify.go` (`server=`).

- [ ] **Step 4: Run tests**

Run: `go test ./internal/ssh ./internal/session ./internal/cli ./internal/models`

Expected: PASS (compile-clean).

- [ ] **Step 5: Commit**

```bash
git add internal/ssh/attach.go internal/ssh/attach_test.go internal/models/status.go internal/session/lifecycle.go internal/cli/status.go internal/cli/verify.go
git commit -m "$(cat <<'EOF'
Treat OpenCode HTTP health as guest readiness.

tmux remaining up is no longer the attach gate.
EOF
)"
```

---

### Task 5: Tunnel and TUI attach args

**Files:**
- Modify: `internal/ssh/attach.go`
- Modify: `internal/models/session.go` (`HostForwardPort int \`json:"host_forward_port,omitempty"\``)
- Test: `internal/ssh/attach_test.go`

**Interfaces:**
- Consumes: session SSH key, guest IP, `cfg.AgentServePort`, `cfg.HostAgentBin`, `cfg.GuestRepoDir`, `cfg.SSHUser`
- Produces:
  - `func PickLocalPort() (int, error)` — `net.Listen("tcp", "127.0.0.1:0")`, close, return port
  - `func TunnelSSHArgs(session *models.SessionRecord, cfg *config.CellConfig, hostPort int) []string`
  - `func TUIArgs(cfg *config.CellConfig, hostPort int, password string) []string`
  - `func LookPathHostAgent(cfg *config.CellConfig) (string, error)`
  - `func AttachTUI(session *models.SessionRecord, cfg *config.CellConfig) error`
  - `func AttachShell(session *models.SessionRecord, cfg *config.CellConfig) error` — current `ssh -t … tmux-attach.sh` **without** `AGENT_CMD`

Tunnel argv (order):

```
-N -o ExitOnForwardFailure=yes -L {hostPort}:127.0.0.1:{cfg.AgentServePort} -i {key} -o IdentitiesOnly=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null {user}@{guestIP}
```

TUI argv:

```
{HostAgentBin} attach http://127.0.0.1:{hostPort} --dir {GuestRepoDir} --continue -p {password}
```

`AttachTUI`: `LookPath` first; pick port; `ssh` tunnel `Start`; defer `Process.Kill`; run TUI with stdin/stdout/stderr; do not touch the VM. Persist `session.HostForwardPort` via a callback is optional — `AttachTUI` may return the port and the session manager saves it. Simpler: `AttachTUI` sets `session.HostForwardPort` in memory; caller `saveSession`.

- [ ] **Step 1: Write the failing test**

```go
func TestTunnelSSHArgs(t *testing.T) {
	session := &models.SessionRecord{
		SSHKeyPath: "/tmp/id",
		NetworkConfig: &models.NetworkConfig{GuestIP: "172.16.1.2"},
	}
	cfg := &config.CellConfig{SSHUser: "agent", AgentServePort: 4096}
	got := ssh.TunnelSSHArgs(session, cfg, 18000)
	want := []string{
		"-N",
		"-o", "ExitOnForwardFailure=yes",
		"-L", "18000:127.0.0.1:4096",
		"-i", "/tmp/id",
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"agent@172.16.1.2",
	}
	if len(got) != len(want) {
		t.Fatalf("len=%d got=%v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestTUIArgs(t *testing.T) {
	cfg := &config.CellConfig{HostAgentBin: "opencode", GuestRepoDir: "/project"}
	got := ssh.TUIArgs(cfg, 18000, "secret")
	want := []string{"attach", "http://127.0.0.1:18000", "--dir", "/project", "--continue", "-p", "secret"}
	if len(got) != len(want) {
		t.Fatalf("got=%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestShellAttachArgsNoAgentCmd(t *testing.T) {
	session := &models.SessionRecord{
		SSHKeyPath:    "/tmp/id",
		NetworkConfig: &models.NetworkConfig{GuestIP: "172.16.1.2"},
	}
	cfg := &config.CellConfig{
		SSHUser:           "agent",
		TmuxSessionName:   "agent",
		GuestRepoDir:      "/project",
		GuestAttachScript: "/opt/guest-init/tmux-attach.sh",
		AgentCmd:          "opencode serve --hostname 127.0.0.1 --port 4096",
	}
	got := ssh.ShellAttachArgs(session, cfg)
	joined := strings.Join(got, " ")
	if strings.Contains(joined, "AGENT_CMD=") {
		t.Fatalf("shell attach still sends AGENT_CMD: %s", joined)
	}
	if !strings.Contains(joined, cfg.GuestAttachScript) {
		t.Fatalf("missing attach script: %s", joined)
	}
}
```

Imports: `models`, `strings`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ssh -run 'TestTunnelSSHArgs|TestTUIArgs|TestShellAttachArgsNoAgentCmd' -v`

Expected: FAIL — undefined.

- [ ] **Step 3: Write minimal implementation**

Implement the arg builders and:

```go
func LookPathHostAgent(cfg *config.CellConfig) (string, error) {
	bin := cfg.HostAgentBin
	if bin == "" {
		bin = "opencode"
	}
	if filepath.IsAbs(bin) {
		if _, err := os.Stat(bin); err != nil {
			return "", fmt.Errorf("host agent missing: %s", bin)
		}
		return bin, nil
	}
	p, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("host agent %q not on PATH: %w", bin, err)
	}
	return p, nil
}

func AttachTUI(session *models.SessionRecord, cfg *config.CellConfig) error {
	if _, err := LookPathHostAgent(cfg); err != nil {
		return err
	}
	pw, err := os.ReadFile(/* caller should pass password */)
	_ = pw
	return fmt.Errorf("not wired")
}
```

Do **not** leave `not wired`. Full `AttachTUI`:

```go
func AttachTUI(session *models.SessionRecord, cfg *config.CellConfig, password string) error {
	bin, err := LookPathHostAgent(cfg)
	if err != nil {
		return err
	}
	if session.SSHKeyPath == "" || session.NetworkConfig == nil {
		return fmt.Errorf("session missing ssh key or network config")
	}
	hostPort, err := PickLocalPort()
	if err != nil {
		return err
	}
	session.HostForwardPort = hostPort
	tunnel := exec.Command("ssh", TunnelSSHArgs(session, cfg, hostPort)...)
	if err := tunnel.Start(); err != nil {
		return fmt.Errorf("ssh tunnel: %w", err)
	}
	defer func() { _ = tunnel.Process.Kill(); _ = tunnel.Wait() }()

	tui := exec.Command(bin, TUIArgs(cfg, hostPort, password)...)
	tui.Stdin = os.Stdin
	tui.Stdout = os.Stdout
	tui.Stderr = os.Stderr
	return tui.Run()
}
```

`PickLocalPort`:

```go
func PickLocalPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port, nil
}
```

Rename current `Attach` body to `AttachShell` / `ShellAttachArgs`. Keep `func Attach(...) { return AttachShell(...) }` until Task 7 so `lifecycle.go` still compiles.

`ShellAttachArgs` remote: `TMUX_SESSION=… REPO_DIR=… {GuestAttachScript}` only.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/ssh -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ssh/attach.go internal/ssh/attach_test.go internal/models/session.go
git commit -m "$(cat <<'EOF'
Build SSH tunnel and host opencode attach argv.

Guest TUI attach is no longer the default SSH remote command.
EOF
)"
```

---

### Task 6: Guest serve + curl + debug tmux

**Files:**
- Modify: `guestinit/guest-entry.sh`
- Modify: `guestinit/tmux-attach.sh`
- Modify: `internal/bootstrap/bootstrap.go`

**Interfaces:**
- Consumes: `/project/.filter/opencode-server.pass`, `CELL_AGENT_CMD` is unused in guest-init (hardcode serve flags from env with defaults matching config)
- Produces: tmux window runs serve; `curl` in rootfs; `tmux-attach.sh` only `tmux attach-session`

Guest env (set in `guest-entry.sh`):

```
SERVE_PORT="${SERVE_PORT:-4096}"
PASS_FILE="/project/.filter/opencode-server.pass"
```

Serve is started from the guest binary on PATH (`opencode`). Kernel cmdline does not pass `SERVE_PORT`; 4096 is fine (matches default). Password from `PASS_FILE`.

- [ ] **Step 1: No unit test for shell; assert bootstrap list includes curl**

If there is no existing bootstrap package-list test, add a tiny helper test only if `debPkg` list is extractable. Otherwise skip to implementation — guest scripts are the check. Do **not** add a test framework.

Optional: `internal/bootstrap` already tests behavior elsewhere; do not invent a chroot test.

- [ ] **Step 2: Implement guest-entry tmux command**

Replace the inner `tmux new-session` command so the pane is serve, not `zsh -l`. After `cd` into repo:

```sh
PASS_FILE="${REPO_DIR}/.filter/opencode-server.pass"
if [ ! -s "${PASS_FILE}" ]; then
  log "ERROR: missing ${PASS_FILE}"
  return 1
fi
agent_tmux "
  cd '${REPO_DIR}' &&
  tmux new-session -d -s '${TMUX_SESSION}' -c '${REPO_DIR}' \
    'export HOME=${AGENT_HOME}; export TMPDIR=/tmp; export BUN_TMPDIR=/tmp; \
     export npm_config_cache=/tmp/npm-cache; \
     export XDG_CACHE_HOME=${AGENT_HOME}/.cache; export XDG_CONFIG_HOME=${AGENT_HOME}/.config; \
     export PATH=/usr/local/bin:/usr/bin:/bin; \
     export OPENCODE_SERVER_PASSWORD=\$(cat ${PASS_FILE}); \
     export OPENCODE_PERMISSION={\"*\":\"allow\"}; \
     exec opencode serve --hostname 127.0.0.1 --port 4096'
"
```

Keep the same HOME/PATH exports. On success log `runtime ready` as today. Host still waits on HTTP health.

Quote carefully: `OPENCODE_PERMISSION` JSON inside single-quoted tmux command — use a file or `OPENCODE_PERMISSION='{"*":"allow"}'` inside the double-quoted `su -c` string. Prefer writing `/run/opencode.env` from guest-init (root) then `set -a; . /run/opencode.env` in the tmux command to avoid quoting hell:

```sh
cat > /run/opencode.env <<EOF
OPENCODE_SERVER_PASSWORD=$(cat "${PASS_FILE}")
OPENCODE_PERMISSION={"*":"allow"}
EOF
chmod 600 /run/opencode.env
chown "${AGENT_USER}:${AGENT_USER}" /run/opencode.env
```

tmux command: `. /run/opencode.env; exec opencode serve --hostname 127.0.0.1 --port 4096`

- [ ] **Step 3: Simplify `tmux-attach.sh`**

Replace the file with:

```sh
#!/bin/sh
set -eu
SESSION="${TMUX_SESSION:-agent}"
REPO_DIR="${REPO_DIR:-/project}"
if tmux has-session -t "${SESSION}" 2>/dev/null; then
  exec tmux attach-session -t "${SESSION}"
fi
cd "${REPO_DIR}" || exit 1
exec tmux new-session -s "${SESSION}" -c "${REPO_DIR}"
```

No `AGENT_CMD`, no `send-keys`.

- [ ] **Step 4: Add curl to jammy debs**

In `ensureJammyDebs` list append `{Name: "curl", Binary: "/usr/bin/curl"}`. In `customize` add `command -v curl`. If extract fails on missing libs, add the same style of extra debs as `libpopt0` for rsync (fix from `ldd` when you actually rebuild; do not guess a pile of libs in advance — first try `curl` alone, the squashfs likely has libssl/libcurl).

- [ ] **Step 5: Commit**

```bash
git add guestinit/guest-entry.sh guestinit/tmux-attach.sh internal/bootstrap/bootstrap.go
git commit -m "$(cat <<'EOF'
Start opencode serve in the guest tmux session.

curl is in the rootfs so the host can probe /global/health over SSH.
EOF
)"
```

Note for the operator: `sudo cell bootstrap --rebuild-rootfs` is required once after this lands. Put that sentence in the commit body is enough; README is Task 9.

---

### Task 7: Launch does not stop the VM; wire TUI attach

**Files:**
- Modify: `internal/session/lifecycle.go`
- Modify: `internal/cli/ssh_cmd.go`

**Interfaces:**
- Consumes: `WriteServerPassword`, `ssh.AttachTUI`, `ReadServerPassword`, `ssh.AttachShell`
- Produces: `SessionManager.Attach` = TUI; `SessionManager.AttachShell` = debug SSH. `Launch` on attach: auto-pull while TUI runs; **never** `Stop` after TUI. On TUI error after boot: still no Stop. Print is CLI-side in Task 8; here return the session.

- [ ] **Step 1: Change `Launch` attach branch**

Replace the attach block with:

```go
if attach {
	pullCtx, cancelPull := context.WithCancel(ctx)
	defer cancelPull()
	if sm.cfg.AutoPull {
		verbose.V("launch: auto-pull enabled (interval %ds)", sm.cfg.AutoPullIntervalSec)
		go sync.StartAutoPull(pullCtx, session, sm.cfg, sm.pullAdapter)
	}
	verbose.V("launch: attaching host TUI")
	if err := sm.attachTUI(session); err != nil {
		cancelPull()
		return session, err
	}
	cancelPull()
}
return session, nil
```

```go
func (sm *SessionManager) attachTUI(session *models.SessionRecord) error {
	pw, err := ReadServerPassword(session.SessionDir)
	if err != nil {
		return err
	}
	err = ssh.AttachTUI(session, sm.cfg, pw)
	_ = sm.saveSession(session) // persist HostForwardPort
	return err
}

func (sm *SessionManager) Attach(ctx context.Context, sessionID string) error {
	_ = ctx
	session, err := sm.loadSession(sessionID)
	if err != nil {
		return err
	}
	if !ssh.VMRunning(session.FCPid) {
		return fmt.Errorf("VM not running; cell start --session %s", sessionID)
	}
	pullCtx, cancelPull := context.WithCancel(ctx)
	defer cancelPull()
	if sm.cfg.AutoPull {
		go sync.StartAutoPull(pullCtx, session, sm.cfg, sm.pullAdapter)
	}
	return sm.attachTUI(session)
}

func (sm *SessionManager) AttachShell(ctx context.Context, sessionID string) error {
	_ = ctx
	session, err := sm.loadSession(sessionID)
	if err != nil {
		return err
	}
	return ssh.AttachShell(session, sm.cfg)
}
```

`newSSHCmd` calls `sm.AttachShell`.

- [ ] **Step 2: Compile**

Run: `go test ./internal/session ./internal/cli ./internal/ssh`

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/session/lifecycle.go internal/cli/ssh_cmd.go
git commit -m "$(cat <<'EOF'
Keep the VM running after the host TUI exits.

cell ssh is a debug shell; launch/attach drive opencode on the host.
EOF
)"
```

---

### Task 8: `cell attach` and `cell start`

**Files:**
- Create: `internal/cli/attach.go`
- Create: `internal/cli/start.go`
- Create: `internal/session/start.go`
- Test: `internal/session/start_test.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/launch.go` (print session still running after attach returns)

**Interfaces:**
- Consumes: `ssh.VMRunning`, `session.ProjectDiskPath`, existing `startVM` / `waitReady` / `Attach`
- Produces:
  - `var ErrAlreadyRunning = errors.New("vm already running")`
  - `func StartPreflight(session *models.SessionRecord, vmRunning bool) error` — if `vmRunning` return `ErrAlreadyRunning`; if `os.Stat(ProjectDiskPath)` fails, `fmt.Errorf("project disk missing: %w", err)`; else nil
  - `func (sm *SessionManager) Start(ctx context.Context, sessionID string, attach bool) error`

`Start` algorithm:

1. load session
2. `vmRunning := ssh.VMRunning(session.FCPid)`
3. `if err := StartPreflight(session, vmRunning); err != nil { return err }`
4. `startVM` + `waitReady`
5. if `attach` { same auto-pull + `attachTUI` as Launch; no Stop }

CLI `cell start --session ID [--no-attach]`:
- if `ErrAlreadyRunning`: print `session %s already running; cell attach --session %s\n` to stdout, exit 0
- else run Start

CLI `cell attach --session ID`: `sm.Attach`

CLI `launch` after successful attach return: print `session %s still running; cell attach --session %s` / `cell stop --session %s`

- [ ] **Step 1: Write the failing test**

```go
package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

func TestStartPreflightMissingDisk(t *testing.T) {
	s := &models.SessionRecord{ProjectDiskPath: filepath.Join(t.TempDir(), "nope.ext4")}
	err := StartPreflight(s, false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStartPreflightAlreadyRunning(t *testing.T) {
	s := &models.SessionRecord{ProjectDiskPath: filepath.Join(t.TempDir(), "disk.ext4")}
	if err := os.WriteFile(s.ProjectDiskPath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	err := StartPreflight(s, true)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("err=%v", err)
	}
}

func TestStartPreflightOK(t *testing.T) {
	s := &models.SessionRecord{ProjectDiskPath: filepath.Join(t.TempDir(), "disk.ext4")}
	if err := os.WriteFile(s.ProjectDiskPath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := StartPreflight(s, false); err != nil {
		t.Fatalf("err=%v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/session -run TestStartPreflight -v`

Expected: FAIL — undefined.

- [ ] **Step 3: Implement StartPreflight + Start + CLI**

`internal/session/start.go`:

```go
package session

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/ssh"
	"github.com/ykhoroshevskiy-tech/cell/internal/sync"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

var ErrAlreadyRunning = errors.New("vm already running")

func StartPreflight(session *models.SessionRecord, vmRunning bool) error {
	if vmRunning {
		return ErrAlreadyRunning
	}
	if _, err := os.Stat(session.ProjectDiskPath); err != nil {
		return fmt.Errorf("project disk missing: %w", err)
	}
	return nil
}

func (sm *SessionManager) Start(ctx context.Context, sessionID string, attach bool) error {
	session, err := sm.loadSession(sessionID)
	if err != nil {
		return err
	}
	if err := StartPreflight(session, ssh.VMRunning(session.FCPid)); err != nil {
		return err
	}
	verbose.V("start: booting existing disk %s", session.ProjectDiskPath)
	if err := sm.startVM(ctx, session); err != nil {
		return err
	}
	if err := sm.waitReady(session); err != nil {
		return err
	}
	if !attach {
		return nil
	}
	pullCtx, cancelPull := context.WithCancel(ctx)
	defer cancelPull()
	if sm.cfg.AutoPull {
		go sync.StartAutoPull(pullCtx, session, sm.cfg, sm.pullAdapter)
	}
	return sm.attachTUI(session)
}
```

`internal/cli/start.go` — cobra `--session` required, `--no-attach` like launch. Handle `errors.Is(err, session.ErrAlreadyRunning)`.

`internal/cli/attach.go` — cobra `--session` required, call `sm.Attach`.

Register both on root.

Launch `RunE`: after `sm.Launch` returns nil and `!noAttach`, print the still-running hint using `sess.SessionID`. If Launch returned session+err because TUI failed, still print the id if `sess != nil` so the user can `cell attach`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/session ./internal/cli ./internal/ssh ./internal/config`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/session/start.go internal/session/start_test.go internal/cli/attach.go internal/cli/start.go internal/cli/root.go internal/cli/launch.go
git commit -m "$(cat <<'EOF'
Add cell attach and cell start for a persistent guest server.

start reboots the existing project disk; attach reconnects the host TUI.
EOF
)"
```

---

### Task 9: README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Rewrite the How it works / commands / agent config sections**

How it works:

1. Stage — copy the repo onto a project disk (includes `.filter/opencode-server.pass`)
2. Boot — Firecracker microVM
3. Serve — `opencode serve` in the guest
4. Attach — host `opencode attach` over SSH `-L`
5. Sync — `cell pull` / auto-pull while attached

Commands table: add `attach`, `start`; change `ssh` to “Debug SSH + tmux (serve logs)”; `status`/`verify` say server not tmux.

Agent table: `CELL_AGENT_CMD` serve default; add `CELL_AGENT_SERVE_PORT`, `CELL_HOST_AGENT_BIN`. Note: host must have OpenCode; rebuild rootfs once for curl.

Clipboard: host TUI, not SSH.

Lifecycle: TUI exit leaves VM; `cell stop` to kill; `cell start` after reboot.

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "$(cat <<'EOF'
Document host TUI attach and session start after reboot.
EOF
)"
```

---

## Spec coverage

| Spec item | Task |
|-----------|------|
| serve in guest, TUI on host, SSH `-L` | 5, 6, 7 |
| bind 127.0.0.1, no TAP change | 5, 6 (implicit) |
| TUI exit does not Stop | 7 |
| `cell attach` / `start` / `stop` / `ssh` | 7, 8 |
| password 0600 + disk copy, no rotate | 2, 3 |
| health not tmux | 4 |
| curl in rootfs | 6 |
| HostForwardPort ephemeral | 5 |
| missing host opencode fail fast, VM stays | 5 `LookPath` before tunnel; 7 no Stop |
| start missing disk / already running | 8 |
| auto-pull only while attached | 7, 8 |
| README | 9 |
| `--auto` on server via `OPENCODE_PERMISSION` | 6 |
| `--dir /project` | 5 `TUIArgs` |

## Placeholder scan

None left: arg lists, file paths, and commands are concrete. Guest quoting uses `/run/opencode.env` so JSON is not nested in tmux strings.
