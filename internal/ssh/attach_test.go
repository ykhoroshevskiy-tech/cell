package ssh_test

import (
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/ssh"
)

func TestCheckFatal(t *testing.T) {
	cases := []struct {
		log    string
		fatal  bool
	}{
		{"[guest-init] ERROR: missing marker", true},
		{"Kernel panic - not syncing", true},
		{"Attempted to kill init!", true},
		{"runtime ready", false},
	}
	for _, c := range cases {
		got := ssh.CheckFatalForTest(c.log)
		if (got != "") != c.fatal {
			t.Fatalf("log=%q got=%q fatal=%v", c.log, got, c.fatal)
		}
	}
}

func TestHealthProbeRemote(t *testing.T) {
	cfg := &config.CellConfig{AgentServePort: 4096}
	got := ssh.HealthProbeRemote(cfg)
	want := "curl -sf http://127.0.0.1:4096/global/health"
	if got != want {
		t.Fatalf("probe = %q want %q", got, want)
	}
}

func TestReadTailFromBytes(t *testing.T) {
	data := []byte("line1\nline2\nline3\n")
	got := ssh.ReadTailFromBytes(data, 2)
	if got != "line2\nline3" {
		t.Fatalf("tail=%q", got)
	}
}
