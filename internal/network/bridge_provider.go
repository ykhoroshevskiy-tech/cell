package network

import (
	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

// BridgeProvider attaches session TAPs to the shared cell0 bridge.
type BridgeProvider struct{}

func NewBridge() *BridgeProvider {
	return &BridgeProvider{}
}

func (b *BridgeProvider) Setup(cfg *models.NetworkConfig) error {
	verbose.V("net: attach tap=%s guest=%s", cfg.TapName, cfg.GuestIP)
	return AttachTap(cfg)
}

func (b *BridgeProvider) Teardown(cfg *models.NetworkConfig) error {
	verbose.V("net: detach tap=%s", cfg.TapName)
	return DetachTap(cfg.TapName)
}
