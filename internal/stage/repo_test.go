package stage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStageRepositorySkipsCellDir(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "README"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	cell := filepath.Join(src, ".cell", "agent-home")
	if err := os.MkdirAll(cell, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cell, "blob"), []byte("heavy"), 0644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "staged")
	if err := StageRepository(src, dst, true, nil); err != nil {
		t.Fatalf("StageRepository() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "README")); err != nil {
		t.Fatalf("README missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".cell")); !os.IsNotExist(err) {
		t.Fatal(".cell must not be staged from the host repo")
	}
}
