package hypervisor

import (
	"fmt"
	"runtime"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
)

func NewHypervisor(cfg *config.CellConfig) (Hypervisor, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("cell requires Linux with KVM")
	}
	return NewFirecracker(cfg.FirecrackerBin), nil
}
