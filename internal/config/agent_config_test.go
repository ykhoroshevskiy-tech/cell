package config

import (
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

func TestLoadCellAgentDefault(t *testing.T) {
	t.Setenv("CELL_AGENT", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CellAgent != models.AgentKindOpenCode {
		t.Fatalf("CellAgent = %q want opencode", cfg.CellAgent)
	}
	if cfg.AgentBin != "opencode" || cfg.HostAgentBin != "opencode" {
		t.Fatalf("opencode defaults drifted: bin=%q host=%q", cfg.AgentBin, cfg.HostAgentBin)
	}
	if cfg.AgentServePort != 4096 {
		t.Fatalf("AgentServePort = %d want 4096", cfg.AgentServePort)
	}
}

func TestLoadCellAgentDispatch(t *testing.T) {
	cases := []struct {
		env string
	}{
		{models.AgentKindOpenCode},
		{models.AgentKindClaude},
		{models.AgentKindNone},
	}
	for _, c := range cases {
		t.Setenv("CELL_AGENT", c.env)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("CELL_AGENT=%s: Load() error = %v", c.env, err)
		}
		if cfg.CellAgent != c.env {
			t.Fatalf("CELL_AGENT=%s: CellAgent = %q", c.env, cfg.CellAgent)
		}
		wantURL := "https://github.com/anomalyco/opencode/releases/latest/download/opencode-{target}.tar.gz"
		if c.env == models.AgentKindClaude {
			wantURL = defaultClaudeAgentURL()
		}
		if cfg.AgentURL != wantURL {
			t.Fatalf("CELL_AGENT=%s: AgentURL = %q want %q", c.env, cfg.AgentURL, wantURL)
		}
	}
}

func TestLoadCellAgentClaudeDefaults(t *testing.T) {
	t.Setenv("CELL_AGENT", "claude")
	t.Setenv("CELL_AGENT_SERVE_PORT", "")
	t.Setenv("CELL_HOST_AGENT_BIN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CellAgent != "claude" {
		t.Fatalf("CellAgent = %q want claude", cfg.CellAgent)
	}
	wantURL := "https://registry.npmjs.org/@anthropic-ai/claude-code/-/claude-code-" + claudeCodeVersion + ".tgz"
	if cfg.AgentURL != wantURL {
		t.Fatalf("AgentURL = %q want %q", cfg.AgentURL, wantURL)
	}
	if cfg.AgentBin != "claude" {
		t.Fatalf("AgentBin = %q want claude", cfg.AgentBin)
	}
	if cfg.AgentServePort != 0 {
		t.Fatalf("AgentServePort = %d want 0", cfg.AgentServePort)
	}
	if cfg.HostAgentBin != "claude" {
		t.Fatalf("HostAgentBin = %q want claude", cfg.HostAgentBin)
	}
}

func TestLoadCellAgentURLOverride(t *testing.T) {
	t.Setenv("CELL_AGENT", "claude")
	custom := "https://example.com/claude/claude.tgz"
	t.Setenv("CELL_AGENT_URL", custom)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentURL != custom {
		t.Fatalf("AgentURL = %q want custom override honored", cfg.AgentURL)
	}

	t.Setenv("CELL_AGENT", "")
	t.Setenv("CELL_AGENT_URL", custom)
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentURL != custom {
		t.Fatalf("AgentURL override must be kind-independent: %q", cfg.AgentURL)
	}
}

func TestLoadCellAgentInvalid(t *testing.T) {
	t.Setenv("CELL_AGENT", "cursor")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid CELL_AGENT")
	}
}
