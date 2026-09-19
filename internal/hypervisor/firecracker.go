package hypervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/ykhoroshevskiy-tech/cell/internal/firecracker"
	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

type FirecrackerHypervisor struct {
	BinPath string
}

func NewFirecracker(binPath string) *FirecrackerHypervisor {
	return &FirecrackerHypervisor{BinPath: binPath}
}

func (f *FirecrackerHypervisor) Start(ctx context.Context, cfg *models.VmConfigDocument, serialLogPath, socketPath string) (int, error) {
	_ = ctx // configure/start are fast; process lifetime must outlive this call
	logFile, err := os.OpenFile(serialLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("open serial log: %w", err)
	}

	// Long-lived: plain Command, NOT CommandContext — a cancelled parent ctx would SIGKILL FC.
	cmd := exec.Command(f.BinPath, "--api-sock", socketPath)
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		_ = logFile.Close()
		return 0, fmt.Errorf("open /dev/null: %w", err)
	}
	cmd.Stdin = stdin
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = logFile.Close()
		return 0, fmt.Errorf("start firecracker: %w", err)
	}
	// stdin/logFile stay open for the child's lifetime; FC holds serial_log.
	// Reap in background when FC exits so we don't leave zombies forever.
	go func() {
		_ = cmd.Wait()
		_ = stdin.Close()
		_ = logFile.Close()
	}()

	pid := cmd.Process.Pid
	verbose.V("fc: waiting for api socket %s", socketPath)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			return 0, fmt.Errorf("firecracker exited before socket ready")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		_ = cmd.Process.Kill()
		return 0, fmt.Errorf("firecracker socket timeout")
	}
	verbose.V("fc: socket ready")

	client := firecracker.NewClient(socketPath)
	defer client.Close()
	if err := client.Configure(cfg); err != nil {
		_ = cmd.Process.Kill()
		return 0, fmt.Errorf("configure firecracker: %w", err)
	}
	verbose.V("fc: configured, starting instance")
	if err := client.StartInstance(); err != nil {
		_ = cmd.Process.Kill()
		return 0, fmt.Errorf("start instance: %w", err)
	}
	verbose.V("fc: instance started")

	return pid, nil
}

func (f *FirecrackerHypervisor) Stop(pid int) error {
	if pid <= 0 {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("cannot stop VM pid %d (started as another user?): run: sudo cell stop", pid)
		}
		return nil // already gone (reaped by Start's Wait goroutine)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return nil // already gone (reaped by Start's Wait goroutine)
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = proc.Kill()
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}
