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
