package hypervisor

import (
	"context"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

type Hypervisor interface {
	Start(ctx context.Context, cfg *models.VmConfigDocument, serialLogPath, socketPath string) (pid int, err error)
	Stop(pid int) error
}
