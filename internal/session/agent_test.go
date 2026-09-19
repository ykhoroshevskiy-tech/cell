package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

func TestAttachDispatch(t *testing.T) {
	cases := []struct {
		agent string
		want  string
	}{
		{models.AgentKindOpenCode, "tui"},
		{models.AgentKindClaude, "shell"},
		{models.AgentKindNone, "shell"},
		{"", "tui"}, // legacy sessions attach via the opencode tunnel path
	}
	for _, c := range cases {
		if got := attachDispatch(c.agent); got != c.want {
			t.Fatalf("attachDispatch(%q) = %q want %q", c.agent, got, c.want)
		}
	}
}

func TestWriteAgentKindToDiskRoot(t *testing.T) {
	cases := []struct {
		kind string
		want string
	}{
		{models.AgentKindOpenCode, "opencode"},
		{models.AgentKindClaude, "claude"},
		{models.AgentKindNone, "none"},
		{"", "opencode"},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			diskRoot := t.TempDir()
			if err := WriteAgentKindToDiskRoot(diskRoot, c.kind); err != nil {
				t.Fatalf("write: %v", err)
			}
			got, err := os.ReadFile(filepath.Join(diskRoot, GuestAgentKindRel))
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(string(got)) != c.want {
				t.Fatalf("agent.kind = %q want %q", got, c.want)
			}
		})
	}
}

func TestWriteAgentKindToDiskRootInvalid(t *testing.T) {
	if err := WriteAgentKindToDiskRoot(t.TempDir(), "cursor"); err == nil {
		t.Fatal("expected error for invalid kind")
	}
}
