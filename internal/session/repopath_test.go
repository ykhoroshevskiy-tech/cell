package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRepoSourceEmpty(t *testing.T) {
	if _, err := ResolveRepoSource(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveRepoSourceRelativeBecomesAbs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "repo")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	got, err := ResolveRepoSource("./repo")
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(want) {
		t.Fatalf("got %q want %q", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("not abs: %q", got)
	}
}

func TestResolveRepoSourceMissing(t *testing.T) {
	_, err := ResolveRepoSource(filepath.Join(t.TempDir(), "nope"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveRepoSourceNotDir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveRepoSource(f)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not a directory") && !strings.Contains(err.Error(), "directory") {
		t.Fatalf("err=%v", err)
	}
}
