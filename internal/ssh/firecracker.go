package ssh

import (
	"fmt"
	"os"
	"strings"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

func cmdlineMatchesFirecracker(data []byte, socketPath string) bool {
	cmdline := strings.ReplaceAll(string(data), "\x00", " ")
	return strings.Contains(cmdline, "firecracker") && strings.Contains(cmdline, socketPath)
}

// VerifyFirecracker checks /proc/<pid>/cmdline (world-readable, even for
// root-owned processes) against the session API socket, so liveness stays
// truthful when run without root.
func VerifyFirecracker(pid int, socketPath string) bool {
	if pid <= 0 || socketPath == "" {
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
