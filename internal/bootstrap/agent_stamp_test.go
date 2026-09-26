package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRootfsAgentKind(t *testing.T) {
	cases := []struct {
		stamp string
		want  string
		ok    bool
	}{
		{"debootstrap:noble+apt+node:v24.20.0+uv:0.12.7+py:3.13+sp:off+agent:opencode", "opencode", true},
		{"debootstrap:noble+agent:claude", "claude", true},
		{"debootstrap:noble+agent:none", "none", true},
		{"debootstrap:noble+agent:all", "all", true},
		{"debootstrap:noble", "", false},
		{"", "", false},
		{"agent:bogus", "", false},
		{"agent:", "", false},
	}
	for _, c := range cases {
		got, ok := ParseRootfsAgentKind(c.stamp)
		if got != c.want || ok != c.ok {
			t.Fatalf("ParseRootfsAgentKind(%q) = (%q,%v) want (%q,%v)", c.stamp, got, ok, c.want, c.ok)
		}
	}
}

func TestCheckRootfsAgentKind(t *testing.T) {
	writeStamp := func(t *testing.T, stamp string) string {
		t.Helper()
		rootfs := filepath.Join(t.TempDir(), "rootfs.ext4")
		if err := os.WriteFile(rootfs, []byte("rootfs"), 0644); err != nil {
			t.Fatal(err)
		}
		if stamp != "" {
			if err := os.WriteFile(rootfsBuildStampPath(rootfs), []byte(stamp+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		return rootfs
	}

	opencode := writeStamp(t, "debootstrap:noble+agent:opencode")
	if err := CheckRootfsAgentKind(opencode, "opencode"); err != nil {
		t.Fatalf("matching opencode: %v", err)
	}
	err := CheckRootfsAgentKind(opencode, "claude")
	if err == nil || err.Error() != `rootfs built for agent "opencode"; run: sudo cell bootstrap --rebuild-rootfs` {
		t.Fatalf("mismatch err=%v", err)
	}

	all := writeStamp(t, "debootstrap:noble+agent:all")
	if err := CheckRootfsAgentKind(all, "opencode"); err != nil {
		t.Fatalf("all-opencode: %v", err)
	}
	if err := CheckRootfsAgentKind(all, "claude"); err != nil {
		t.Fatalf("all-claude: %v", err)
	}

	legacy := writeStamp(t, "")
	if err := CheckRootfsAgentKind(legacy, "opencode"); err != nil {
		t.Fatalf("legacy opencode: %v", err)
	}
	if err := CheckRootfsAgentKind(legacy, "claude"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("legacy claude err=%v", err)
	}

	if err := CheckRootfsAgentKind(legacy, "none"); err != nil {
		t.Fatalf("none must pass without an agent: %v", err)
	}
}
