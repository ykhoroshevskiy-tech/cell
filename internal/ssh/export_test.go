package ssh

import (
	"os"
	"time"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

// Exports for external (ssh_test) tests.
func CheckFatalForTest(logText string) string { return checkFatal(logText) }

func WaitTunnelForwardReadyForTest(hostPort int, proc *os.Process, timeout time.Duration, tail *StderrTail) error {
	return waitTunnelForwardReady(hostPort, proc, timeout, tail)
}

func TunnelFailureErrorForTest(session *models.SessionRecord, waitErr error, tail *StderrTail) error {
	return tunnelFailureError(session, waitErr, tail)
}
