package session

import (
	"context"
	"fmt"
	"os"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/network"
	"github.com/ykhoroshevskiy-tech/cell/internal/ssh"
)

// Remove deletes a stopped session and frees its network lease.
func (sm *SessionManager) Remove(ctx context.Context, sessionID string) error {
	_ = ctx
	session, err := sm.loadSession(sessionID)
	if err != nil {
		return err
	}
	if ssh.VMRunningForSession(session) {
		return fmt.Errorf("session %s is running; stop it first", sessionID)
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

// RemoveLegacy drops sessions without network_version 2 (no live VM required).
func (sm *SessionManager) RemoveLegacy(ctx context.Context, sessionID string) error {
	_ = ctx
	session, err := sm.loadSession(sessionID)
	if err != nil {
		return err
	}
	if ssh.VMRunningForSession(session) {
		return fmt.Errorf("session %s is running; stop it first", sessionID)
	}
	if session.NetworkConfig != nil && session.NetworkConfig.Version == models.NetworkVersion {
		return fmt.Errorf("session %s uses network v2; use normal rm", sessionID)
	}
	return sm.Remove(ctx, sessionID)
}
