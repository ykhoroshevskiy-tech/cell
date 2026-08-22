package session

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/ssh"
	"github.com/ykhoroshevskiy-tech/cell/internal/sync"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

var ErrAlreadyRunning = errors.New("vm already running")

func prepareVMBootArtifacts(session *models.SessionRecord) error {
	if err := os.Remove(session.SocketPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.OpenFile(session.SerialLogPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}

func StartPreflight(session *models.SessionRecord, vmRunning bool) error {
	if vmRunning {
		return ErrAlreadyRunning
	}
	if _, err := os.Stat(session.ProjectDiskPath); err != nil {
		return fmt.Errorf("project disk missing: %w", err)
	}
	return nil
}

func (sm *SessionManager) Start(ctx context.Context, sessionID string, attach bool) error {
	session, err := sm.loadSession(sessionID)
	if err != nil {
		return err
	}
	if err := StartPreflight(session, ssh.VMRunningForSession(session)); err != nil {
		return err
	}
	if err := models.ValidateNetworkConfig(session.NetworkConfig); err != nil {
		return err
	}
	verbose.V("start: booting existing disk %s", session.ProjectDiskPath)
	if err := prepareVMBootArtifacts(session); err != nil {
		return err
	}
	if err := sm.startVM(ctx, session); err != nil {
		return err
	}
	if err := sm.waitReady(session); err != nil {
		return err
	}
	if !attach {
		return nil
	}
	pullCtx, cancelPull := context.WithCancel(ctx)
	defer cancelPull()
	if sm.cfg.AutoPull {
		go sync.StartAutoPull(pullCtx, session, sm.cfg, sm.pullAdapter)
	}
	return sm.attachTUI(session)
}
