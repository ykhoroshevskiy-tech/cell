package ssh_test

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/ssh"
)

func TestCheckFatal(t *testing.T) {
	cases := []struct {
		log   string
		fatal bool
	}{
		{"[guest-init] ERROR: missing marker", true},
		{"Kernel panic - not syncing", true},
		{"Attempted to kill init!", true},
		{"runtime ready", false},
	}
	for _, c := range cases {
		got := ssh.CheckFatalForTest(c.log)
		if (got != "") != c.fatal {
			t.Fatalf("log=%q got=%q fatal=%v", c.log, got, c.fatal)
		}
	}
}

func TestHealthProbeRemote(t *testing.T) {
	cfg := &config.CellConfig{GuestRepoDir: "/project", AgentServePort: 4096}
	got := ssh.HealthProbeRemote(cfg)
	want := "curl -sf --connect-timeout 2 --max-time 3 -u opencode:$(cat /project/.filter/opencode-server.pass) http://127.0.0.1:4096/global/health"
	if got != want {
		t.Fatalf("probe = %q want %q", got, want)
	}
}

func TestReadTailFromBytes(t *testing.T) {
	data := []byte("line1\nline2\nline3\n")
	got := ssh.ReadTailFromBytes(data, 2)
	if got != "line2\nline3" {
		t.Fatalf("tail=%q", got)
	}
}

func TestTunnelSSHArgs(t *testing.T) {
	session := &models.SessionRecord{
		SSHKeyPath:    "/tmp/id",
		NetworkConfig: &models.NetworkConfig{GuestIP: "172.16.1.2"},
	}
	cfg := &config.CellConfig{SSHUser: "agent", AgentServePort: 4096}
	got := ssh.TunnelSSHArgs(session, cfg, 18000)
	want := []string{
		"-N",
		"-o", "ExitOnForwardFailure=yes",
		"-L", "18000:127.0.0.1:4096",
		"-F", "/dev/null",
		"-i", "/tmp/id",
		"-o", "IdentitiesOnly=yes",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5",
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

func TestWaitTunnelForwardReady(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ssh.WaitTunnelForwardReadyForTest(port, nil, 2*time.Second, nil); err != nil {
		t.Fatalf("ready listener: %v", err)
	}
	_ = ln.Close()

	tail := ssh.NewStderrTail()
	_, _ = tail.Write([]byte("Connection refused\n"))

	dead := exec.Command("true")
	if err := dead.Start(); err != nil {
		t.Fatal(err)
	}
	if err := dead.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := ssh.WaitTunnelForwardReadyForTest(59999, dead.Process, time.Second, tail); err == nil {
		t.Fatal("expected error when tunnel exited")
	} else if got := err.Error(); got != "ssh tunnel exited before forward ready; ssh tunnel stderr (tail): Connection refused" {
		t.Fatalf("exit err=%q", got)
	}

	if err := ssh.WaitTunnelForwardReadyForTest(59999, nil, 500*time.Millisecond, tail); err == nil {
		t.Fatal("expected timeout error")
	} else if got := err.Error(); got != "ssh tunnel forward not ready; ssh tunnel stderr (tail): Connection refused" {
		t.Fatalf("timeout err=%q", got)
	}
}

// A tunnel that emits stderr then dies: the attach failure must explain itself
// with the captured stderr tail lines (root cause visible, fake process).
func TestTunnelFailureCarriesStderrTail(t *testing.T) {
	tail := ssh.NewStderrTail()
	proc := exec.Command("/bin/sh", "-c", "echo 'Permission denied (publickey)' >&2; exit 0")
	proc.Stderr = tail
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	_ = proc.Wait()

	err := ssh.WaitTunnelForwardReadyForTest(59999, proc.Process, 300*time.Millisecond, tail)
	if err == nil {
		t.Fatal("expected failure")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ssh tunnel exited before forward ready") {
		t.Fatalf("missing base error: %q", msg)
	}
	if !strings.Contains(msg, "ssh tunnel stderr (tail):") || !strings.Contains(msg, "Permission denied (publickey)") {
		t.Fatalf("missing stderr tail: %q", msg)
	}
}

func TestTunnelFailureRootCauseWording(t *testing.T) {
	tail := ssh.NewStderrTail()
	_, _ = tail.Write([]byte("boom\n"))

	// Dead VM: guidance to `cell start`, not a tunnel complaint.
	dead := &models.SessionRecord{SessionID: "abc123", FCPid: 0, State: "stopped"}
	err := ssh.TunnelFailureErrorForTest(dead, errors.New("ssh tunnel forward not ready"), tail)
	if err == nil || err.Error() != "VM not running while attaching; cell start --session abc123" {
		t.Fatalf("dead VM err=%v", err)
	}

	// VM alive per Firecracker identity: surface the tunnel wait error.
	live := &models.SessionRecord{SessionID: "abc123"}
	live.SocketPath = "empty.socket"
	proc := exec.Command("/bin/sh", "-c", "sleep 2", "firecracker", "--api-sock", live.SocketPath)
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = proc.Process.Kill(); _, _ = proc.Process.Wait() }()
	live.FCPid = proc.Process.Pid
	for range 50 {
		if data, rerr := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", live.FCPid)); rerr == nil && len(data) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	err = ssh.TunnelFailureErrorForTest(live, errors.New("ssh tunnel forward not ready"), tail)
	if err == nil || !strings.Contains(err.Error(), "attach failed: ssh tunnel forward not ready") {
		t.Fatalf("live VM err=%v", err)
	}
}

func TestShellAttachArgs(t *testing.T) {
	session := &models.SessionRecord{
		SSHKeyPath:    "/tmp/id",
		NetworkConfig: &models.NetworkConfig{GuestIP: "172.16.1.2"},
	}
	cfg := &config.CellConfig{
		SSHUser:           "agent",
		TmuxSessionName:   "agent",
		GuestRepoDir:      "/project",
		GuestAttachScript: "/opt/guest-init/tmux-attach.sh",
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
