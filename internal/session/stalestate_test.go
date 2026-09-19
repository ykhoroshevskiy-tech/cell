package session

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/network"
)

type fakeHypervisor struct{}

func (fakeHypervisor) Start(context.Context, *models.VmConfigDocument, string, string) (int, error) {
	return 0, os.ErrNotExist
}

func (fakeHypervisor) Stop(int) error { return nil }

// launchStaleFakeVM starts a process whose /proc cmdline matches the
// "firecracker" + socket-path identity check, so VerifyFirecracker reports
// the VM as running without real KVM.
func launchStaleFakeVM(t *testing.T, socketPath string) int {
	t.Helper()
	proc := exec.Command("/bin/sh", "-c", "sleep 30", "firecracker", "--api-sock", socketPath)
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = proc.Process.Kill()
		_, _ = proc.Process.Wait()
	})
	pid := proc.Process.Pid
	for range 50 {
		if data, rerr := os.ReadFile("/proc/" + itoa(pid) + "/cmdline"); rerr == nil && len(data) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return pid
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

// A dead FCPid with stale State "running" must be repaired: List reports
// VMRunning=false and State=stopped, and session.json is demoted on disk
// (so `ps` can never print "running" for a dead VM).
func TestListRepairsStaleRunningRecord(t *testing.T) {
	cfg := testConfig(t)
	sm := &SessionManager{cfg: cfg}

	stale := &models.SessionRecord{SessionID: "abc123", State: models.StateRunning, FCPid: 9999999}
	stale.ArtifactPaths(cfg.SessionDataDir)
	stale.SocketPath = stale.SessionDir + "/firecracker.socket"
	writeSession(t, cfg, stale)

	list, err := sm.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list=%d", len(list))
	}
	if list[0].VMRunning {
		t.Fatal("VMRunning must be false for a dead record")
	}
	if list[0].State != models.StateStopped {
		t.Fatalf("State after repair = %q, want stopped", list[0].State)
	}

	onDisk, err := sm.loadSession("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.FCPid != 0 || onDisk.State != models.StateStopped {
		t.Fatalf("session.json not repaired: pid=%d state=%q", onDisk.FCPid, onDisk.State)
	}
}

// StopAll on a mixed set: the actually-running kill+count is 1; the
// stale-running record is demoted to stopped by repair and not counted.
func TestStopAllRepairsAndCounts(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	restoreLock := network.SetLockPathForTest(filepath.Join(t.TempDir(), "cell-network.lock"))
	defer restoreLock()
	restoreNet := network.SetNetSetupForTest(func() error { return nil })
	defer restoreNet()
	_ = network.SetRunnerForTest(network.NewFakeRunnerForTest())

	cfg := testConfig(t)
	sm := &SessionManager{cfg: cfg, hypervisor: fakeHypervisor{}}

	live := &models.SessionRecord{SessionID: "aaa111", State: models.StateRunning}
	live.ArtifactPaths(cfg.SessionDataDir)
	writeSession(t, cfg, live)
	live.FCPid = launchStaleFakeVM(t, live.SocketPath)
	writeSession(t, cfg, live)

	stale := &models.SessionRecord{SessionID: "bbb222", State: models.StateRunning, FCPid: 9999999}
	stale.ArtifactPaths(cfg.SessionDataDir)
	stale.SocketPath = stale.SessionDir + "/firecracker.socket"
	writeSession(t, cfg, stale)

	stopped, errs := sm.StopAll(context.Background())
	if len(errs) != 0 {
		t.Fatalf("errs=%v", errs)
	}
	if stopped != 1 {
		t.Fatalf("stopped=%d, want 1", stopped)
	}

	onDisk, err := sm.loadSession("bbb222")
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.FCPid != 0 || onDisk.State != models.StateStopped {
		t.Fatalf("stale record not repaired: pid=%d state=%q", onDisk.FCPid, onDisk.State)
	}
}
