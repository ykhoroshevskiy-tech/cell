package sync

import (
	"context"
	"time"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

type PullFunc func(ctx context.Context, session *models.SessionRecord, opts models.PullOptions) (*models.PullResult, error)

func StartAutoPull(ctx context.Context, session *models.SessionRecord, cfg *config.CellConfig, pull PullFunc) {
	if !cfg.AutoPull {
		return
	}
	interval := time.Duration(cfg.AutoPullIntervalSec) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	opts := models.PullOptions{Dest: session.RepoSource, Quiet: true}
	for {
		select {
		case <-ctx.Done():
			_, _ = pull(context.Background(), session, opts)
			return
		case <-ticker.C:
			_, _ = pull(ctx, session, opts)
		}
	}
}
