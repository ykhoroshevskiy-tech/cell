package network

import "github.com/ykhoroshevskiy-tech/cell/internal/models"

type NetworkProvider interface {
	Setup(cfg *models.NetworkConfig) error
	Teardown(cfg *models.NetworkConfig) error
}
