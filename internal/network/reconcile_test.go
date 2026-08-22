package network

import (
	"strings"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

func TestFirewallRulesGlobalSubnet(t *testing.T) {
	rules := FirewallRulesForTest()
	if len(rules) == 0 {
		t.Fatal("no rules")
	}
	joined := strings.Join(rules, "\n")
	if !strings.Contains(joined, "172.16.107.0/24") {
		t.Fatalf("missing subnet masquerade: %s", joined)
	}
	if strings.Count(joined, "MASQUERADE") != 1 {
		t.Fatalf("want single MASQUERADE: %s", joined)
	}
}

func TestReconcileRemovesStaleTAP(t *testing.T) {
	fake := newFakeRunner()
	fake.links["ctap-deadbeef"] = true
	fake.bridges["cell0"] = true
	restore := SetRunnerForTest(fake)
	defer restore()

	cfg := models.NewNetworkConfig("live1234", "172.16.107.2")
	if err := ReconcileLive([]*models.NetworkConfig{cfg}); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.links["ctap-deadbeef"] {
		t.Fatal("stale tap not removed")
	}
	if !fake.links[cfg.TapName] {
		t.Fatalf("live tap %s missing", cfg.TapName)
	}
}

func TestEnsureFirewallIdempotent(t *testing.T) {
	fake := newFakeRunner()
	restore := SetRunnerForTest(fake)
	defer restore()
	if err := EnsureFirewall(); err != nil {
		t.Fatal(err)
	}
	if err := EnsureFirewall(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureBridgeIdempotent(t *testing.T) {
	fake := newFakeRunner()
	restore := SetRunnerForTest(fake)
	defer restore()

	if err := EnsureBridge(); err != nil {
		t.Fatal(err)
	}
	if err := EnsureBridge(); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if !fake.bridges["cell0"] {
		t.Fatal("bridge missing")
	}
}
