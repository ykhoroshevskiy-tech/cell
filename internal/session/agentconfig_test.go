package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

func TestResolveAgentConfigEmpty(t *testing.T) {
	if _, _, err := ResolveAgentConfig(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveAgentConfigMissing(t *testing.T) {
	_, _, err := ResolveAgentConfig(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveAgentConfigDir(t *testing.T) {
	dir := t.TempDir()
	_, _, err := ResolveAgentConfig(dir)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not a file") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveAgentConfigRelativeBecomesAbs(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "opencode.json")
	if err := os.WriteFile(f, []byte(`{"model":"x"}`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	abs, warn, err := ResolveAgentConfig("./opencode.json")
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Fatalf("warn=%q", warn)
	}
	want, err := filepath.Abs(f)
	if err != nil {
		t.Fatal(err)
	}
	if abs != filepath.Clean(want) {
		t.Fatalf("got %q want %q", abs, want)
	}
	if !filepath.IsAbs(abs) {
		t.Fatalf("not abs: %q", abs)
	}
}

func TestResolveAgentConfigInvalidJSONWarns(t *testing.T) {
	f := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(f, []byte("{"), 0644); err != nil {
		t.Fatal(err)
	}
	abs, warn, err := ResolveAgentConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if abs == "" {
		t.Fatal("expected abs path")
	}
	if !strings.Contains(warn, "not valid JSON") {
		t.Fatalf("warn=%q", warn)
	}
	if !strings.Contains(warn, abs) {
		t.Fatalf("warn missing path: %q", warn)
	}
}

func TestResolveAgentConfigValidJSONNoWarn(t *testing.T) {
	f := filepath.Join(t.TempDir(), "ok.json")
	body := []byte(`{"$schema":"https://opencode.ai/config.json"}`)
	if !json.Valid(body) {
		t.Fatal("fixture")
	}
	if err := os.WriteFile(f, body, 0644); err != nil {
		t.Fatal(err)
	}
	_, warn, err := ResolveAgentConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Fatalf("warn=%q", warn)
	}
}

func TestCopyAgentConfigToDiskRootEmpty(t *testing.T) {
	disk := t.TempDir()
	if err := CopyAgentConfigToDiskRoot("", disk); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(disk, GuestAgentConfigRel)); !os.IsNotExist(err) {
		t.Fatalf("unexpected dest: %v", err)
	}
}

func TestCopyAgentConfigToDiskRoot(t *testing.T) {
	src := filepath.Join(t.TempDir(), "opencode.json")
	body := []byte(`{"model":"x"}`)
	if err := os.WriteFile(src, body, 0644); err != nil {
		t.Fatal(err)
	}
	disk := t.TempDir()
	if err := CopyAgentConfigToDiskRoot(src, disk); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(disk, GuestAgentConfigRel)
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("got %q want %q", got, body)
	}
	st, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatalf("perm = %o, want 0600", st.Mode().Perm())
	}
}

func TestCopyAgentConfigToDiskRootMissing(t *testing.T) {
	err := CopyAgentConfigToDiskRoot(filepath.Join(t.TempDir(), "nope.json"), t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAgentConfigPathNotSerialized(t *testing.T) {
	s := models.SessionRecord{
		SessionID:       "abc",
		AgentConfigPath: "/tmp/secret-opencode.json",
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-opencode") || strings.Contains(string(data), "AgentConfig") {
		t.Fatalf("leaked: %s", data)
	}
}
