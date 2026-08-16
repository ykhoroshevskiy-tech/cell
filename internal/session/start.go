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
	if err := StartPreflight(session, ssh.VMRunning(session.FCPid)); err != nil {
		return err
	}
	verbose.V("start: booting existing disk %s", session.ProjectDiskPath)
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
