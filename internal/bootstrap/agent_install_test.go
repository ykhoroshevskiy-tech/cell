package bootstrap

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

func TestSquashfsBuildStampIncludesAgentKind(t *testing.T) {
	cases := []struct {
		agent string
		want  string
	}{
		{"opencode", "debootstrap:noble+apt+node:v24.20.0+uv:0.12.7+py:3.13+sp:off+agent:opencode"},
		{"claude", "debootstrap:noble+apt+node:v24.20.0+uv:0.12.7+py:3.13+sp:off+agent:claude"},
		{"none", "debootstrap:noble+apt+node:v24.20.0+uv:0.12.7+py:3.13+sp:off+agent:none"},
		{"", "debootstrap:noble+apt+node:v24.20.0+uv:0.12.7+py:3.13+sp:off+agent:opencode"},
	}
	for _, c := range cases {
		cfg := &config.CellConfig{CellAgent: c.agent}
		if got := squashfsBuildStamp(cfg); got != c.want {
			t.Fatalf("squashfsBuildStamp(agent=%q) = %q want %q", c.agent, got, c.want)
		}
	}
	opencode := squashfsBuildStamp(&config.CellConfig{CellAgent: models.AgentKindOpenCode})
	claude := squashfsBuildStamp(&config.CellConfig{CellAgent: models.AgentKindClaude})
	if opencode == claude {
		t.Fatal("stamp must differ between agent kinds (forces rebuild)")
	}
}

func TestFindAgentBinaryHandlesPackagePrefix(t *testing.T) {
	tarball := filepath.Join(t.TempDir(), "claude-code.tgz")
	work := t.TempDir()
	if err := os.MkdirAll(filepath.Join(work, "package"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "package", "claude"), []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("tar", "-czf", tarball, "-C", work, "package").CombinedOutput(); err != nil {
		t.Fatalf("fabricate tarball: %v\n%s", err, out)
	}

	extract := t.TempDir()
	if err := exec.Command("tar", "-xzf", tarball, "-C", extract).Run(); err != nil {
		t.Fatal(err)
	}
	src, err := findAgentBinary(extract, "claude")
	if err != nil {
		t.Fatalf("findAgentBinary: %v", err)
	}
	if filepath.Base(src) != "claude" {
		t.Fatalf("src = %q want claude", src)
	}
}

func TestFindAgentBinaryRootedAtTarballRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "opencode"), []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}
	src, err := findAgentBinary(dir, "opencode")
	if err != nil {
		t.Fatal(err)
	}
	if src != filepath.Join(dir, "opencode") {
		t.Fatalf("src = %q", src)
	}
}

func TestFindAgentBinaryMissing(t *testing.T) {
	if _, err := findAgentBinary(t.TempDir(), "claude"); err == nil {
		t.Fatal("expected error when binary missing in tarball")
	}
}
