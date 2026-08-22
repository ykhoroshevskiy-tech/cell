package ssh

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

var fatalPatterns = []struct {
	pattern *regexp.Regexp
	label   string
}{
	{regexp.MustCompile(`(?i)Kernel panic`), "kernel panic"},
	{regexp.MustCompile(`(?i)\[guest-init\] ERROR:`), "guest-init error"},
	{regexp.MustCompile(`(?i)Attempted to kill init`), "init exited"},
}

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

func sshPortOpen(guestIP string, port int) bool {
	addr := net.JoinHostPort(guestIP, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func WaitForSSH(guestIP string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := net.JoinHostPort(guestIP, fmt.Sprintf("%d", port))
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		verbose.V("ssh: waiting for %s (%v remaining)", addr, time.Until(deadline).Round(time.Second))
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("SSH not reachable at %s within %v", addr, timeout)
}

func CheckFatalForTest(logText string) string {
	return checkFatal(logText)
}

func checkFatal(logText string) string {
	for _, fp := range fatalPatterns {
		if fp.pattern.MatchString(logText) {
			if m := fp.pattern.FindString(logText); m != "" {
				return fmt.Sprintf("guest failed (%s): %s", fp.label, m)
			}
		}
	}
	return ""
}

func ReadTail(path string, lines int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return ReadTailFromBytes(data, lines)
}

func ReadTailFromBytes(data []byte, lines int) string {
	all := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, "\n")
}

func VMRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func RuntimeReady(serialLogPath string) bool {
	data, err := os.ReadFile(serialLogPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "runtime ready")
}

func WaitRuntimeReady(session *models.SessionRecord, cfg *config.CellConfig) error {
	deadline := time.Now().Add(cfg.SSHReadyTimeoutSec)
	var offset int64

	for time.Now().Before(deadline) {
		if session.FCPid > 0 && !VMRunningForSession(session) {
			return fmt.Errorf("VM process exited before runtime ready\n--- serial tail ---\n%s",
				ReadTail(session.SerialLogPath, 20))
		}

		if f, err := os.Open(session.SerialLogPath); err == nil {
			buf := make([]byte, 8192)
			n, _ := f.ReadAt(buf, offset)
			offset += int64(n)
			chunk := string(buf[:n])
			f.Close()
			if verbose.Enabled() && n > 0 {
				os.Stderr.WriteString(chunk)
			}
			if fatal := checkFatal(chunk); fatal != "" {
				return fmt.Errorf("%s\n--- serial tail ---\n%s", fatal, ReadTail(session.SerialLogPath, 20))
			}
		}

		status := SessionStatus(session, cfg)
		if status.RuntimeReady && status.SSHReachable && status.ServerReady {
			return nil
		}
		verbose.V("wait: vm=%v ssh=%v runtime=%v server=%v (%v remaining)",
			status.VMRunning, status.SSHReachable, status.RuntimeReady, status.ServerReady,
			time.Until(deadline).Round(time.Second))
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("guest not ready within %v\n--- serial tail ---\n%s",
		cfg.SSHReadyTimeoutSec, ReadTail(session.SerialLogPath, 20))
}

func HealthProbeRemote(cfg *config.CellConfig) string {
	passFile := fmt.Sprintf("%s/.filter/opencode-server.pass", cfg.GuestRepoDir)
	return fmt.Sprintf(
		"curl -sf --connect-timeout 2 --max-time 3 -u opencode:$(cat %s) http://127.0.0.1:%d/global/health",
		passFile, cfg.AgentServePort,
	)
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

func SessionStatus(session *models.SessionRecord, cfg *config.CellConfig) *models.SessionStatus {
	st := &models.SessionStatus{SessionID: session.SessionID}
	st.VMRunning = VMRunningForSession(session)
	if session.NetworkConfig != nil {
		st.GuestIP = session.NetworkConfig.GuestIP
		st.TapName = session.NetworkConfig.TapName
		if st.VMRunning {
			st.SSHReachable = sshPortOpen(session.NetworkConfig.GuestIP, 22)
		}
	}
	st.RuntimeReady = RuntimeReady(session.SerialLogPath)
	if st.SSHReachable {
		st.ServerReady = ServerReady(session, cfg)
	}
	return st
}

func PickLocalPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port, nil
}

func TunnelSSHArgs(session *models.SessionRecord, cfg *config.CellConfig, hostPort int) []string {
	args := []string{
		"-N",
		"-o", "ExitOnForwardFailure=yes",
		"-L", fmt.Sprintf("%d:127.0.0.1:%d", hostPort, cfg.AgentServePort),
	}
	args = append(args, sshBaseArgs(session.SSHKeyPath)...)
	args = append(args, fmt.Sprintf("%s@%s", cfg.SSHUser, session.NetworkConfig.GuestIP))
	return args
}

func TUIArgs(cfg *config.CellConfig, hostPort int, password string) []string {
	return []string{
		"attach",
		fmt.Sprintf("http://127.0.0.1:%d", hostPort),
		"--dir", cfg.GuestRepoDir,
		"--continue",
		"-p", password,
	}
}

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

func ShellAttachArgs(session *models.SessionRecord, cfg *config.CellConfig) []string {
	remote := fmt.Sprintf(
		"TMUX_SESSION=%s REPO_DIR=%s %s",
		cfg.TmuxSessionName, cfg.GuestRepoDir, cfg.GuestAttachScript,
	)
	args := append([]string{"-t"}, sshBaseArgs(session.SSHKeyPath)...)
	args = append(args,
		fmt.Sprintf("%s@%s", cfg.SSHUser, session.NetworkConfig.GuestIP),
		remote,
	)
	return args
}

func AttachShell(session *models.SessionRecord, cfg *config.CellConfig) error {
	if session.SSHKeyPath == "" || session.NetworkConfig == nil {
		return fmt.Errorf("session missing ssh key or network config")
	}
	cmd := exec.Command("ssh", ShellAttachArgs(session, cfg)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

const tunnelForwardWaitTimeout = 5 * time.Second

func tunnelProcessAlive(proc *os.Process) bool {
	if proc == nil {
		return true
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func waitTunnelForwardReady(hostPort int, proc *os.Process, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", hostPort))
	for time.Now().Before(deadline) {
		if !tunnelProcessAlive(proc) {
			return fmt.Errorf("ssh tunnel exited before forward ready")
		}
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("ssh tunnel forward not ready")
}

func WaitTunnelForwardReadyForTest(hostPort int, proc *os.Process, timeout time.Duration) error {
	return waitTunnelForwardReady(hostPort, proc, timeout)
}

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

	if err := waitTunnelForwardReady(hostPort, tunnel.Process, tunnelForwardWaitTimeout); err != nil {
		return err
	}

	tui := exec.Command(bin, TUIArgs(cfg, hostPort, password)...)
	tui.Stdin = os.Stdin
	tui.Stdout = os.Stdout
	tui.Stderr = os.Stderr
	return tui.Run()
}

