package network

import (
	"fmt"
	"runtime"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
)

func NewNetworkProvider(cfg *config.CellConfig) (NetworkProvider, error) {
	_ = cfg
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("cell requires Linux with KVM")
	}
	return NewTAP(), nil
}
