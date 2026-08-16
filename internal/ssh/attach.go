package ssh

import (
	"fmt"
	"net"
	"os"
	"os/exec"
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
		"-i", keyPath,
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
	}
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
		if session.FCPid > 0 && !VMRunning(session.FCPid) {
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

func SessionStatus(session *models.SessionRecord, cfg *config.CellConfig) *models.SessionStatus {
	st := &models.SessionStatus{SessionID: session.SessionID}
	st.VMRunning = VMRunning(session.FCPid)
	if session.NetworkConfig != nil {
		st.GuestIP = session.NetworkConfig.GuestIP
		st.TapName = session.NetworkConfig.TapName
		if st.VMRunning {
			st.SSHReachable = WaitForSSH(session.NetworkConfig.GuestIP, 22, time.Second) == nil
		}
	}
	st.RuntimeReady = RuntimeReady(session.SerialLogPath)
	if st.SSHReachable {
		st.ServerReady = ServerReady(session, cfg)
	}
	return st
}

func Attach(session *models.SessionRecord, cfg *config.CellConfig) error {
	if session.SSHKeyPath == "" || session.NetworkConfig == nil {
		return fmt.Errorf("session missing ssh key or network config")
	}
	// Single-quote AGENT_CMD so spaces survive the remote shell.
	cmdQuoted := "'" + strings.ReplaceAll(cfg.AgentCmd, "'", `'\''`) + "'"
	remote := fmt.Sprintf(
		"TMUX_SESSION=%s REPO_DIR=%s AGENT_BIN=%s AGENT_CMD=%s %s",
		cfg.TmuxSessionName, cfg.GuestRepoDir, cfg.AgentBin, cmdQuoted, cfg.GuestAttachScript,
	)
	args := append([]string{"-t"}, sshBaseArgs(session.SSHKeyPath)...)
	args = append(args,
		fmt.Sprintf("%s@%s", cfg.SSHUser, session.NetworkConfig.GuestIP),
		remote,
	)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
