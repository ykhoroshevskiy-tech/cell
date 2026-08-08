package models_test

import (
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

func TestAllocateNetwork(t *testing.T) {
	cfg := models.AllocateNetwork("ab9e30a73a2f")
	if cfg.TapName != "ctap-ab9e30a7" {
		t.Fatalf("tap=%q", cfg.TapName)
	}
	if cfg.GuestIP != "172.16.172.2" {
		t.Fatalf("guest_ip=%q", cfg.GuestIP)
	}
	if cfg.HostIP != "172.16.172.1" {
		t.Fatalf("host_ip=%q", cfg.HostIP)
	}
	if cfg.CIDR != 30 {
		t.Fatalf("cidr=%d", cfg.CIDR)
	}
}

func TestKernelBootArgs(t *testing.T) {
	cfg := models.AllocateNetwork("ab9e30a73a2f")
	want := "console=ttyS0 reboot=k panic=1 pci=off init=/opt/guest-init/guest-entry.sh ip=172.16.172.2::172.16.172.1:255.255.255.252::eth0:off"
	if got := cfg.KernelBootArgs(); got != want {
		t.Fatalf("KernelBootArgs()=%q want %q", got, want)
	}
}
