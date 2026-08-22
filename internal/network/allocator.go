package network

import (
	"fmt"
	"net"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

// OccupiedGuestIPs returns guest IPs reserved by persisted sessions.
func OccupiedGuestIPs(sessions []*models.SessionRecord) map[string]struct{} {
	occupied := make(map[string]struct{})
	for _, s := range sessions {
		if s == nil || s.NetworkConfig == nil || s.NetworkConfig.GuestIP == "" {
			continue
		}
		occupied[s.NetworkConfig.GuestIP] = struct{}{}
	}
	return occupied
}

// AllocateGuestIP picks the lowest free address in 172.16.107.2–254.
func AllocateGuestIP(occupied map[string]struct{}) (string, error) {
	prefix := net.ParseIP(models.BridgeHostIP).To4()
	if prefix == nil {
		return "", fmt.Errorf("invalid bridge host ip")
	}
	for last := models.GuestIPFirst; last <= models.GuestIPLast; last++ {
		ip := fmt.Sprintf("%d.%d.%d.%d", prefix[0], prefix[1], prefix[2], last)
		if _, used := occupied[ip]; !used {
			return ip, nil
		}
	}
	return "", fmt.Errorf("guest IP pool exhausted (%d addresses)", models.GuestIPLast-models.GuestIPFirst+1)
}
