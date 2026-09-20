package guestinit

import (
	"strings"
	"testing"
)

func TestGuestEntryIgnoresCellInGitignore(t *testing.T) {
	data, err := Scripts.ReadFile("guest-entry.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, want := range []string{
		"ensure_cell_gitignore",
		".cell/",
		"setup_usr_local_rw",
		".cell/usr-local",
		"mount --bind",
		".cell/opencode.json",
		"WARN: failed to copy host opencode.json; using default",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("guest-entry.sh missing %q", want)
		}
	}
}

func TestGuestEntryPluginEntryConditionalOnSuperpowers(t *testing.T) {
	data, err := Scripts.ReadFile("guest-entry.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, want := range []string{
		"[ -d /opt/opencode-plugins/node_modules/superpowers ]",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("guest-entry.sh missing conditional plugin marker %q", want)
		}
	}
}

func TestGuestEntryAgentKindDispatch(t *testing.T) {
	data, err := Scripts.ReadFile("guest-entry.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, want := range []string{
		".cell/agent.kind",
		`AGENT_KIND="${AGENT_KIND:-opencode}"`,
		`[ "${AGENT_KIND}" = "claude" ]`,
		`[ "${AGENT_KIND}" = "none" ]`,
		"unknown agent kind in ${KIND_FILE}; defaulting to opencode",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("guest-entry.sh missing agent kind dispatch marker %q", want)
		}
	}
}

func TestGuestEntryClaudeBranch(t *testing.T) {
	data, err := Scripts.ReadFile("guest-entry.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, want := range []string{
		".claude/settings.json",
		`"defaultMode": "bypassPermissions"`,
		"exec claude --dangerously-skip-permissions",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("guest-entry.sh missing claude branch marker %q", want)
		}
	}
	start := strings.Index(script, "start_tmux_session()")
	if start < 0 {
		t.Fatal("start_tmux_session not found")
	}
	opencodeElse := strings.Index(script[start:], "\n  else")
	if opencodeElse < 0 {
		t.Fatal("claude/opencode tmux branch split not found")
	}
	claudeBlock := script[start : start+opencodeElse]
	if strings.Contains(claudeBlock, "opencode-server.pass") {
		t.Fatal("claude tmux branch must not require the opencode password file")
	}
	if strings.Contains(claudeBlock, "opencode serve") {
		t.Fatal("claude tmux branch must not run opencode serve")
	}
}

func TestGuestEntryOpencodeBranchUnchanged(t *testing.T) {
	data, err := Scripts.ReadFile("guest-entry.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, want := range []string{
		".cell/opencode-server.pass",
		".cell/opencode-serve.port",
		"/run/opencode.env",
		"exec opencode serve --hostname 127.0.0.1 --port ${SERVE_PORT}",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("guest-entry.sh missing opencode branch marker %q", want)
		}
	}
}

func TestGuestEntryNoneBranchSkipsTmuxBoot(t *testing.T) {
	data, err := Scripts.ReadFile("guest-entry.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, want := range []string{
		`[ "${AGENT_KIND}" = "none" ]`,
		"ssh-only runtime, no tmux boot",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("guest-entry.sh missing none branch marker %q", want)
		}
	}
}
