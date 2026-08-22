package models

import (
	"strings"
	"testing"
)

func TestNewNetworkConfigV2(t *testing.T) {
	cfg := NewNetworkConfig("abc123def", "172.16.107.10")
	if cfg.Version != NetworkVersion {
		t.Fatalf("version=%d", cfg.Version)
	}
	if cfg.TapName != "ctap-abc123de" {
		t.Fatalf("tap=%q", cfg.TapName)
	}
	if cfg.HostIP != BridgeHostIP || cfg.CIDR != BridgeCIDR {
		t.Fatalf("host=%s cidr=%d", cfg.HostIP, cfg.CIDR)
	}
	if cfg.GuestIP != "172.16.107.10" {
		t.Fatalf("guest=%s", cfg.GuestIP)
	}
	if !strings.Contains(cfg.KernelBootArgs(), "ip=172.16.107.10::172.16.107.1:255.255.255.0") {
		t.Fatalf("boot args: %s", cfg.KernelBootArgs())
	}
}

func TestValidateNetworkConfigOK(t *testing.T) {
	cfg := NewNetworkConfig("abc123", "172.16.107.2")
	if err := ValidateNetworkConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestValidateNetworkConfigLegacyVersion(t *testing.T) {
	cfg := NewNetworkConfig("abc123", "172.16.107.2")
	cfg.Version = 1
	if err := ValidateNetworkConfig(cfg); err == nil {
		t.Fatal("expected legacy rejection")
	}
}

func TestValidateNetworkConfigBadGuestIP(t *testing.T) {
	cfg := NewNetworkConfig("abc123", "172.16.107.1")
	if err := ValidateNetworkConfig(cfg); err == nil {
		t.Fatal("expected host IP rejection")
	}
}

func TestValidateNetworkConfigMissing(t *testing.T) {
	if err := ValidateNetworkConfig(nil); err == nil {
		t.Fatal("expected error")
	}
}
