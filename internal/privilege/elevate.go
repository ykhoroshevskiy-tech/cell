package privilege

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// MaybeElevate runs the current cell invocation under sudo when not root.
// bootstrap and rescue need root (debootstrap chroot, setcap, groupadd, chown,
// loop mount); auto-elevation keeps the UX sudo-free. CELL_* environment
// variables are forwarded explicitly because sudo resets the environment.
// Returns nil when already root; otherwise execs sudo and exits with the
// child's exit code, or returns an error when sudo is unavailable.
func MaybeElevate(command string) error {
	if os.Geteuid() == 0 {
		return nil
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return fmt.Errorf("cell: %s requires root — run: sudo cell %s", command, command)
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cell: locate executable: %w", err)
	}
	sudoCmd := exec.Command("sudo", sudoArgList(exe, os.Environ(), os.Args[1:])...)
	sudoCmd.Stdin = os.Stdin
	sudoCmd.Stdout = os.Stdout
	sudoCmd.Stderr = os.Stderr
	if err := sudoCmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("cell: sudo %s: %w", command, err)
	}
	os.Exit(0)
	return nil
}

// sudoArgList builds "env CELL_*=… <exe> <args…>" for the sudo re-execution.
func sudoArgList(exe string, environ []string, args []string) []string {
	list := []string{"env"}
	for _, kv := range environ {
		if strings.HasPrefix(kv, "CELL_") {
			list = append(list, kv)
		}
	}
	list = append(list, exe)
	return append(list, args...)
}
