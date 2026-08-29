package stage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStageRepositorySkipsFilterDir(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "README"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	filter := filepath.Join(src, ".filter", "agent-home")
	if err := os.MkdirAll(filter, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filter, "blob"), []byte("heavy"), 0644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "staged")
	if err := StageRepository(src, dst, true, nil); err != nil {
		t.Fatalf("StageRepository() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "README")); err != nil {
		t.Fatalf("README missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".filter")); !os.IsNotExist(err) {
		t.Fatal(".filter must not be staged from the host repo")
	}
}
