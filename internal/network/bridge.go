package network

import (
	"fmt"
	"strings"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

// EnsureBridge creates cell0 with 172.16.107.1/24 if missing.
func EnsureBridge() error {
	if !linkExists(models.BridgeName) {
		if err := run("ip", "link", "add", "name", models.BridgeName, "type", "bridge"); err != nil {
			return fmt.Errorf("create bridge %s: %w", models.BridgeName, err)
		}
	}
	if err := run("ip", "addr", "replace", fmt.Sprintf("%s/%d", models.BridgeHostIP, models.BridgeCIDR), "dev", models.BridgeName); err != nil {
		return fmt.Errorf("bridge address: %w", err)
	}
	if err := run("ip", "link", "set", models.BridgeName, "up"); err != nil {
		return fmt.Errorf("bridge up: %w", err)
	}
	_ = run("ip", "route", "replace", fmt.Sprintf("%s/%d", trimHost(models.BridgeHostIP), models.BridgeCIDR), "dev", models.BridgeName)
	return nil
}

func trimHost(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ip
	}
	return fmt.Sprintf("%s.%s.%s.0", parts[0], parts[1], parts[2])
}

// AttachTap creates a session TAP on cell0 without an IP and enables port isolation.
func AttachTap(cfg *models.NetworkConfig) error {
	if err := models.ValidateNetworkConfig(cfg); err != nil {
		return err
	}
	if !linkExists(cfg.TapName) {
		if err := run("ip", "tuntap", "add", "dev", cfg.TapName, "mode", "tap"); err != nil {
			return fmt.Errorf("create tap %s: %w", cfg.TapName, err)
		}
	}
	if err := run("ip", "link", "set", cfg.TapName, "master", models.BridgeName); err != nil {
		return fmt.Errorf("enslave tap %s: %w", cfg.TapName, err)
	}
	if err := run("ip", "link", "set", cfg.TapName, "up"); err != nil {
		return fmt.Errorf("tap up %s: %w", cfg.TapName, err)
	}
	// L2 guest isolation on the bridge port
	if err := run("bridge", "link", "set", "dev", cfg.TapName, "isolated", "on"); err != nil {
		return fmt.Errorf("isolate tap %s: %w", cfg.TapName, err)
	}
	return nil
}

// DetachTap removes a session TAP if it exists.
func DetachTap(tapName string) error {
	if tapName == "" || !linkExists(tapName) {
		return nil
	}
	_ = run("ip", "link", "set", tapName, "down")
	return run("ip", "link", "del", tapName)
}

// ListCellTAPs returns ctap-* interfaces present on the host.
func ListCellTAPs() ([]string, error) {
	out, err := output("ip", "-o", "link", "show")
	if err != nil {
		return nil, err
	}
	var taps []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimSuffix(fields[1], ":")
		if strings.HasPrefix(name, "ctap-") {
			taps = append(taps, name)
		}
	}
	return taps, nil
}
