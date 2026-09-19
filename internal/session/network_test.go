package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/network"
)

func testConfig(t *testing.T) *config.CellConfig {
	t.Helper()
	dir := t.TempDir()
	return &config.CellConfig{SessionDataDir: dir}
}

func writeSession(t *testing.T, cfg *config.CellConfig, session *models.SessionRecord) {
	t.Helper()
	session.ArtifactPaths(cfg.SessionDataDir)
	if err := os.MkdirAll(session.SessionDir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(session.SessionDir, "session.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveFreesLease(t *testing.T) {
	restoreLock := network.SetLockPathForTest(filepath.Join(t.TempDir(), "cell-network.lock"))
	defer restoreLock()
	restoreNet := network.SetNetSetupForTest(func() error { return nil })
	defer restoreNet()
	restore := network.SetRunnerForTest(silentRunner{})
	defer restore()

	cfg := testConfig(t)
	sm := &SessionManager{cfg: cfg}

	keep := &models.SessionRecord{
		SessionID:     "aaa111",
		NetworkConfig: models.NewNetworkConfig("aaa111", "172.16.107.2"),
		State:         models.StateStopped,
	}
	remove := &models.SessionRecord{
		SessionID:     "bbb222",
		NetworkConfig: models.NewNetworkConfig("bbb222", "172.16.107.3"),
		State:         models.StateStopped,
	}
	writeSession(t, cfg, keep)
	writeSession(t, cfg, remove)

	if err := sm.Remove(context.Background(), "bbb222"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(remove.SessionDir); !os.IsNotExist(err) {
		t.Fatalf("session dir still exists: %v", err)
	}

	sessions, err := sm.loadAllSessions()
	if err != nil {
		t.Fatal(err)
	}
	ip, err := sm.allocateNetwork("ccc333")
	if err != nil {
		t.Fatal(err)
	}
	if ip.GuestIP != "172.16.107.3" {
		t.Fatalf("lease not freed: got %s", ip.GuestIP)
	}
	_ = sessions
}

func TestAllocateNetworkSequentialUniqueness(t *testing.T) {
	restoreLock := network.SetLockPathForTest(filepath.Join(t.TempDir(), "cell-network.lock"))
	defer restoreLock()
	restoreNet := network.SetNetSetupForTest(func() error { return nil })
	defer restoreNet()
	cfg := testConfig(t)
	sm := &SessionManager{cfg: cfg}
	seen := map[string]struct{}{}
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("sess%04d", i)
		netCfg, err := sm.allocateNetwork(id)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := seen[netCfg.GuestIP]; ok {
			t.Fatalf("duplicate ip %s", netCfg.GuestIP)
		}
		seen[netCfg.GuestIP] = struct{}{}
		session := &models.SessionRecord{SessionID: id, NetworkConfig: netCfg, State: models.StateCreated}
		writeSession(t, cfg, session)
	}
}
