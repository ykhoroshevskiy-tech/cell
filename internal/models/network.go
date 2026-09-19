package models

import (
	"fmt"
	"net"
)

const (
	NetworkVersion = 2
	BridgeName     = "cell0"
	BridgeHostIP   = "172.16.107.1"
	BridgeCIDR     = 24
	BridgeNetmask  = "255.255.255.0"
	GuestIPFirst   = 2
	GuestIPLast    = 254
)

type NetworkConfig struct {
	Version  int    `json:"network_version"`
	TapName  string `json:"tap_name"`
	HostIP   string `json:"host_ip"`
	GuestIP  string `json:"guest_ip"`
	Netmask  string `json:"netmask"`
	GuestMac string `json:"guest_mac"`
	CIDR     int    `json:"cidr"`
}

func (n *NetworkConfig) KernelIPArg() string {
	return fmt.Sprintf("ip=%s::%s:%s::eth0:off", n.GuestIP, n.HostIP, n.Netmask)
}

func (n *NetworkConfig) KernelBootArgs() string {
	return fmt.Sprintf("console=ttyS0 reboot=k panic=1 pci=off init=/opt/guest-init/guest-entry.sh %s", n.KernelIPArg())
}

// NewNetworkConfig builds a v2 bridge-network intent for a session.
func NewNetworkConfig(sessionID, guestIP string) *NetworkConfig {
	prefix := sessionID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	mac0, mac1 := "00", "00"
	if len(sessionID) >= 2 {
		mac0 = sessionID[0:2]
	}
	if len(sessionID) >= 4 {
		mac1 = sessionID[2:4]
	}
	return &NetworkConfig{
		Version:  NetworkVersion,
		TapName:  fmt.Sprintf("ctap-%s", prefix),
		HostIP:   BridgeHostIP,
		GuestIP:  guestIP,
		Netmask:  BridgeNetmask,
		GuestMac: fmt.Sprintf("AA:FC:%s:%s:00:01", mac0, mac1),
		CIDR:     BridgeCIDR,
	}
}

// ValidateNetworkConfig rejects legacy or malformed network records.
func ValidateNetworkConfig(cfg *NetworkConfig) error {
	if cfg == nil {
		return fmt.Errorf("network config missing")
	}
	if cfg.Version != NetworkVersion {
		return fmt.Errorf("unsupported network_version %d (need %d); remove session with cell rm and launch again", cfg.Version, NetworkVersion)
	}
	if cfg.HostIP != BridgeHostIP || cfg.CIDR != BridgeCIDR || cfg.Netmask != BridgeNetmask {
		return fmt.Errorf("legacy network layout; remove session with cell rm and launch again")
	}
	ip := net.ParseIP(cfg.GuestIP)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("invalid guest_ip %q", cfg.GuestIP)
	}
	last := int(ip.To4()[3])
	if last < GuestIPFirst || last > GuestIPLast {
		return fmt.Errorf("guest_ip %s outside pool .%d-.%d", cfg.GuestIP, GuestIPFirst, GuestIPLast)
	}
	if cfg.TapName == "" {
		return fmt.Errorf("tap_name missing")
	}
	return nil
}
