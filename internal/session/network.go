package session

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/network"
	"github.com/ykhoroshevskiy-tech/cell/internal/ssh"
)

func (sm *SessionManager) loadAllSessions() ([]*models.SessionRecord, error) {
	entries, err := os.ReadDir(sm.cfg.SessionDataDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sessions []*models.SessionRecord
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		session, err := sm.loadSession(entry.Name())
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (sm *SessionManager) repairStaleSessions(sessions []*models.SessionRecord) error {
	for _, session := range sessions {
		if session.FCPid <= 0 {
			continue
		}
		if ssh.VerifyFirecracker(session.FCPid, session.SocketPath) {
			continue
		}
		session.FCPid = 0
		if session.State == models.StateRunning {
			session.State = models.StateStopped
		}
		if err := sm.saveSession(session); err != nil {
			return err
		}
	}
	return nil
}

func (sm *SessionManager) liveNetworkConfigs(sessions []*models.SessionRecord) []*models.NetworkConfig {
	var live []*models.NetworkConfig
	for _, session := range sessions {
		if !ssh.VerifyFirecracker(session.FCPid, session.SocketPath) {
			continue
		}
		if session.NetworkConfig == nil {
			continue
		}
		live = append(live, session.NetworkConfig)
	}
	return live
}

// allocateNetwork assigns a stable guest IP under the global network lock.
func (sm *SessionManager) allocateNetwork(sessionID string) (*models.NetworkConfig, error) {
	var netCfg *models.NetworkConfig
	err := network.WithNetworkLock(func() error {
		sessions, err := sm.loadAllSessions()
		if err != nil {
			return err
		}
		ip, err := network.AllocateGuestIP(network.OccupiedGuestIPs(sessions))
		if err != nil {
			return err
		}
		netCfg = models.NewNetworkConfig(sessionID, ip)
		return nil
	})
	return netCfg, err
}

// reconcileNetwork repairs stale records and converges bridge/TAP/firewall state.
func (sm *SessionManager) reconcileNetwork(include *models.NetworkConfig) error {
	return network.WithNetworkLock(func() error {
		sessions, err := sm.loadAllSessions()
		if err != nil {
			return err
		}
		if err := sm.repairStaleSessions(sessions); err != nil {
			return err
		}
		live := sm.liveNetworkConfigs(sessions)
		if include != nil {
			found := false
			for _, cfg := range live {
				if cfg.TapName == include.TapName {
					found = true
					break
				}
			}
			if !found {
				live = append(live, include)
			}
		}
		return network.ReconcileLive(live)
	})
}

func writeNetworkJSON(sessionDir string, cfg *models.NetworkConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(sessionDir, "network.json"), data, 0644)
}
