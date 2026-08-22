package network

import (
	"fmt"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

// ReconcileLive converges bridge/TAP/firewall to the given live session configs.
func ReconcileLive(live []*models.NetworkConfig) error {
	liveTaps := make([]LiveSession, 0, len(live))
	for _, cfg := range live {
		if cfg != nil && cfg.TapName != "" {
			liveTaps = append(liveTaps, LiveSession{TapName: cfg.TapName})
		}
	}
	if err := Reconcile(liveTaps); err != nil {
		return err
	}
	for _, cfg := range live {
		if cfg == nil {
			continue
		}
		if err := AttachTap(cfg); err != nil {
			return fmt.Errorf("ensure tap %s: %w", cfg.TapName, err)
		}
	}
	return nil
}
