package guestinit

import (
	"strings"
	"testing"
)

func TestGuestEntryIgnoresFilterInGitignore(t *testing.T) {
	data, err := Scripts.ReadFile("guest-entry.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, want := range []string{
		"ensure_filter_gitignore",
		".filter/",
		"setup_usr_local_rw",
		".filter/usr-local",
		"mount --bind",
		".filter/opencode.json",
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
		".filter/agent.kind",
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
		".filter/opencode-server.pass",
		".filter/opencode-serve.port",
		"/run/opencode.env",
		"exec opencode serve --hostname 127.0.0.1 --port ${SERVE_PORT}",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("guest-entry.sh missing opencode branch marker %q", want)
		}
	}
}

func TestGuestEntryTmuxFailureEmitsFatalError(t *testing.T) {
	data, err := Scripts.ReadFile("guest-entry.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, want := range []string{
		`log "ERROR: tmux session ${TMUX_SESSION} failed for agent ${AGENT_KIND}"`,
		`log "runtime degraded: ssh only (tmux failed)"`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("guest-entry.sh missing tmux failure marker %q", want)
		}
	}
	noneIdx := strings.Index(script, "ssh-only runtime, no tmux boot")
	fatalIdx := strings.Index(script, "ERROR: tmux session")
	if noneIdx < 0 || fatalIdx < 0 || noneIdx > fatalIdx {
		t.Fatal("agent none must return before the fatal tmux error log")
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
