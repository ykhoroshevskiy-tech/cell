package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuestJammyDebsIncludesGitAndSudo(t *testing.T) {
	pkgs := guestJammyDebs()
	want := map[string]string{
		"git":  "/usr/bin/git",
		"sudo": "/usr/bin/sudo",
	}
	for name, binary := range want {
		found := false
		for _, p := range pkgs {
			if p.Name == name {
				found = true
				if p.Binary != binary {
					t.Fatalf("guestJammyDebs()[%q].Binary = %q, want %q", name, p.Binary, binary)
				}
			}
		}
		if !found {
			t.Fatalf("guestJammyDebs() missing package %q", name)
		}
	}
}

func TestWriteAgentSudoers(t *testing.T) {
	root := t.TempDir()
	if err := writeAgentSudoers(root); err != nil {
		t.Fatalf("writeAgentSudoers() error = %v", err)
	}
	path := filepath.Join(root, "etc", "sudoers.d", "agent")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sudoers file: %v", err)
	}
	if got := string(data); got != agentSudoersBody() {
		t.Fatalf("sudoers content:\n got %q\nwant %q", got, agentSudoersBody())
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat sudoers file: %v", err)
	}
	if fi.Mode().Perm() != 0440 {
		t.Fatalf("sudoers mode = %o, want 0440", fi.Mode().Perm())
	}
}

func TestGuestCustomizeScriptChecksGitSudo(t *testing.T) {
	script := guestCustomizeScript()
	for _, want := range []string{
		"command -v git",
		"command -v sudo",
		"sudo -n true",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("guestCustomizeScript() missing %q", want)
		}
	}
}
