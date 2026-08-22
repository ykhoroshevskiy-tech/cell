package network

import (
	"fmt"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

func TestAllocateGuestIPLowestFree(t *testing.T) {
	occupied := map[string]struct{}{
		"172.16.107.2": {},
		"172.16.107.3": {},
	}
	ip, err := AllocateGuestIP(occupied)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "172.16.107.4" {
		t.Fatalf("ip=%q want 172.16.107.4", ip)
	}
}

func TestAllocateGuestIPExhausted(t *testing.T) {
	occupied := make(map[string]struct{})
	for i := models.GuestIPFirst; i <= models.GuestIPLast; i++ {
		occupied[fmt.Sprintf("172.16.107.%d", i)] = struct{}{}
	}
	_, err := AllocateGuestIP(occupied)
	if err == nil {
		t.Fatal("expected exhaustion error")
	}
}

func TestOccupiedGuestIPsIncludesStopped(t *testing.T) {
	sessions := []*models.SessionRecord{
		{NetworkConfig: models.NewNetworkConfig("aaa", "172.16.107.5")},
		{NetworkConfig: models.NewNetworkConfig("bbb", "172.16.107.6")},
	}
	occupied := OccupiedGuestIPs(sessions)
	if len(occupied) != 2 {
		t.Fatalf("occupied=%d", len(occupied))
	}
}
