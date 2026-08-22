package ssh

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

func cmdlineMatchesFirecracker(data []byte, socketPath string) bool {
	cmdline := strings.ReplaceAll(string(data), "\x00", " ")
	return strings.Contains(cmdline, "firecracker") && strings.Contains(cmdline, socketPath)
}

// VerifyFirecracker checks PID liveness and /proc cmdline against the session API socket.
func VerifyFirecracker(pid int, socketPath string) bool {
	if pid <= 0 || socketPath == "" {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if proc.Signal(syscall.Signal(0)) != nil {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	return cmdlineMatchesFirecracker(data, socketPath)
}

// VMRunningForSession uses strict Firecracker identity, not PID reuse alone.
func VMRunningForSession(session *models.SessionRecord) bool {
	if session == nil {
		return false
	}
	return VerifyFirecracker(session.FCPid, session.SocketPath)
}
