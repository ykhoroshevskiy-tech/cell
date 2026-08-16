package ssh_test

import (
	"net"
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
		log    string
		fatal  bool
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
	cfg := &config.CellConfig{AgentServePort: 4096}
	got := ssh.HealthProbeRemote(cfg)
	want := "curl -sf http://127.0.0.1:4096/global/health"
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

func TestWaitTunnelForwardReady(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ssh.WaitTunnelForwardReadyForTest(port, nil, 2*time.Second); err != nil {
		t.Fatalf("ready listener: %v", err)
	}
	_ = ln.Close()

	dead := exec.Command("true")
	if err := dead.Start(); err != nil {
		t.Fatal(err)
	}
	if err := dead.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := ssh.WaitTunnelForwardReadyForTest(59999, dead.Process, time.Second); err == nil {
		t.Fatal("expected error when tunnel exited")
	} else if err.Error() != "ssh tunnel exited before forward ready" {
		t.Fatalf("exit err=%q", err)
	}

	if err := ssh.WaitTunnelForwardReadyForTest(59999, nil, 500*time.Millisecond); err == nil {
		t.Fatal("expected timeout error")
	} else if err.Error() != "ssh tunnel forward not ready" {
		t.Fatalf("timeout err=%q", err)
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
