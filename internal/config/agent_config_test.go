package config

import "testing"

const defaultOpencodeURL = "https://github.com/anomalyco/opencode/releases/latest/download/opencode-{target}.tar.gz"

func TestLoadAgentDefaults(t *testing.T) {
	t.Setenv("CELL_AGENT_BIN", "")
	t.Setenv("CELL_AGENT_SERVE_PORT", "")
	t.Setenv("CELL_HOST_AGENT_BIN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AgentURL != defaultOpencodeURL {
		t.Fatalf("AgentURL = %q want %q", cfg.AgentURL, defaultOpencodeURL)
	}
	if cfg.AgentBin != "opencode" || cfg.HostAgentBin != "opencode" {
		t.Fatalf("opencode defaults drifted: bin=%q host=%q", cfg.AgentBin, cfg.HostAgentBin)
	}
	if cfg.AgentServePort != 4096 {
		t.Fatalf("AgentServePort = %d want 4096", cfg.AgentServePort)
	}
}

func TestLoadCellAgentEnvIgnored(t *testing.T) {
	t.Setenv("CELL_AGENT", "claude")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v (removed CELL_AGENT must be ignored)", err)
	}
	if cfg.AgentURL != defaultOpencodeURL {
		t.Fatalf("removed CELL_AGENT must not affect AgentURL: %q", cfg.AgentURL)
	}
	if cfg.AgentBin != "opencode" {
		t.Fatalf("removed CELL_AGENT must not affect AgentBin: %q", cfg.AgentBin)
	}
}

func TestLoadCellAgentURLOverride(t *testing.T) {
	custom := "https://example.com/opencode.tgz"
	t.Setenv("CELL_AGENT_URL", custom)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentURL != custom {
		t.Fatalf("AgentURL = %q want custom override honored", cfg.AgentURL)
	}
}

func TestLoadCellAgentURLEmptySkipsInstall(t *testing.T) {
	t.Setenv("CELL_AGENT_URL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentURL != "" {
		t.Fatalf("AgentURL = %q want empty (skip install)", cfg.AgentURL)
	}
}

func TestLoadCellAgentBinOverride(t *testing.T) {
	t.Setenv("CELL_AGENT_BIN", "my-opencode")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentBin != "my-opencode" {
		t.Fatalf("AgentBin = %q want my-opencode", cfg.AgentBin)
	}
}

func TestDefaultClaudeAgentURL(t *testing.T) {
	want := "https://registry.npmjs.org/@anthropic-ai/claude-code/-/claude-code-" + claudeCodeVersion + ".tgz"
	if got := DefaultClaudeAgentURL(); got != want {
		t.Fatalf("DefaultClaudeAgentURL() = %q want %q", got, want)
	}
}
