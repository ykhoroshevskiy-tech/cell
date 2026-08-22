package network

import (
	"fmt"
)

// LiveSession is a verified running session whose TAP should exist.
type LiveSession struct {
	TapName string
}

// Reconcile converges bridge/TAP runtime state to desired live sessions.
func Reconcile(live []LiveSession) error {
	if err := EnsureBridge(); err != nil {
		return err
	}
	if err := EnsureFirewall(); err != nil {
		return err
	}

	want := make(map[string]struct{}, len(live))
	for _, s := range live {
		if s.TapName != "" {
			want[s.TapName] = struct{}{}
		}
	}

	taps, err := ListCellTAPs()
	if err != nil {
		return err
	}
	for _, tap := range taps {
		if _, ok := want[tap]; !ok {
			if err := DetachTap(tap); err != nil {
				return fmt.Errorf("remove stale tap %s: %w", tap, err)
			}
		}
	}

	return nil
}
