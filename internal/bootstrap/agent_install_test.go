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
		if got := rootfsBuildStamp(cfg); got != c.want {
			t.Fatalf("rootfsBuildStamp(agent=%q) = %q want %q", c.agent, got, c.want)
		}
	}
	opencode := rootfsBuildStamp(&config.CellConfig{CellAgent: models.AgentKindOpenCode})
	claude := rootfsBuildStamp(&config.CellConfig{CellAgent: models.AgentKindClaude})
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

func TestInstallClaudeFromNpmLayout(t *testing.T) {
	work := t.TempDir()
	if err := os.MkdirAll(filepath.Join(work, "package"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "package", "cli.js"), []byte("// cli"), 0644); err != nil {
		t.Fatal(err)
	}
	tarball := filepath.Join(t.TempDir(), "claude-code.tgz")
	if out, err := exec.Command("tar", "-czf", tarball, "-C", work, "package").CombinedOutput(); err != nil {
		t.Fatalf("fabricate tarball: %v\n%s", err, out)
	}
	extract := t.TempDir()
	if out, err := exec.Command("tar", "-xzf", tarball, "-C", extract).CombinedOutput(); err != nil {
		t.Fatalf("extract tarball: %v\n%s", err, out)
	}

	root := t.TempDir()
	if err := installClaudeFromExtract(root, extract); err != nil {
		t.Fatalf("installClaudeFromExtract: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "opt", "agent", "claude-code", "cli.js")); err != nil {
		t.Fatalf("package tree not installed: %v", err)
	}
	wrapper := filepath.Join(root, "usr", "local", "bin", "claude")
	st, err := os.Stat(wrapper)
	if err != nil {
		t.Fatalf("wrapper missing: %v", err)
	}
	if st.Mode().Perm() != 0755 {
		t.Fatalf("wrapper mode = %v want 0755", st.Mode().Perm())
	}
	body, err := os.ReadFile(wrapper)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "#!/bin/sh\nexec node /opt/agent/claude-code/cli.js \"$@\"\n" {
		t.Fatalf("wrapper body = %q", body)
	}
}

func TestInstallOpencodeFromRootLayout(t *testing.T) {
	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "opencode"), []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}
	tarball := filepath.Join(t.TempDir(), "opencode.tgz")
	if out, err := exec.Command("tar", "-czf", tarball, "-C", work, "opencode").CombinedOutput(); err != nil {
		t.Fatalf("fabricate tarball: %v\n%s", err, out)
	}
	extract := t.TempDir()
	if out, err := exec.Command("tar", "-xzf", tarball, "-C", extract).CombinedOutput(); err != nil {
		t.Fatalf("extract tarball: %v\n%s", err, out)
	}

	root := t.TempDir()
	if err := installOpencodeFromExtract(root, extract, "opencode"); err != nil {
		t.Fatalf("installOpencodeFromExtract: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "opt", "agent", "bin", "opencode")); err != nil {
		t.Fatalf("binary not installed: %v", err)
	}
	link := filepath.Join(root, "usr", "local", "bin", "opencode")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("symlink missing: %v", err)
	}
	if target != "/opt/agent/bin/opencode" {
		t.Fatalf("symlink target = %q", target)
	}
}
