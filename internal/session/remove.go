package session

import (
	"context"
	"fmt"
	"os"

	"github.com/ykhoroshevskiy-tech/cell/internal/network"
	"github.com/ykhoroshevskiy-tech/cell/internal/ssh"
)

// Remove deletes a stopped session and frees its network lease.
func (sm *SessionManager) Remove(ctx context.Context, sessionID string) error {
	session, err := sm.loadSession(sessionID)
	if err != nil {
		return err
	}
	if ssh.VMRunningForSession(session) {
		return fmt.Errorf("session %s is running; stop it first", sessionID)
	}
	if err := network.NetSetup(); err != nil {
		return err
	}
	return network.WithNetworkLock(func() error {
		if err := os.RemoveAll(session.SessionDir); err != nil {
			return fmt.Errorf("remove session dir: %w", err)
		}
		sessions, err := sm.loadAllSessions()
		if err != nil {
			return err
		}
		if err := sm.repairStaleSessions(sessions); err != nil {
			return err
		}
		return network.ReconcileLive(sm.liveNetworkConfigs(sessions))
	})
}
