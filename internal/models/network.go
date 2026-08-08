package models

import (
	"fmt"
	"strconv"
)

type NetworkConfig struct {
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

func AllocateNetwork(sessionID string) *NetworkConfig {
	octet := 1
	if len(sessionID) >= 2 {
		if v, err := strconv.ParseInt(sessionID[:2], 16, 64); err == nil {
			octet = int(v%250) + 1
		}
	}
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
		TapName:  fmt.Sprintf("ctap-%s", prefix),
		HostIP:   fmt.Sprintf("172.16.%d.1", octet),
		GuestIP:  fmt.Sprintf("172.16.%d.2", octet),
		Netmask:  "255.255.255.252",
		GuestMac: fmt.Sprintf("AA:FC:%s:%s:00:01", mac0, mac1),
		CIDR:     30,
	}
}
