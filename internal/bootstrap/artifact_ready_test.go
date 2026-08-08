package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArtifactReady(t *testing.T) {
	dir := t.TempDir()

	missing := filepath.Join(dir, "missing")
	if artifactReady(missing) {
		t.Fatal("missing path should not be ready")
	}

	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if artifactReady(empty) {
		t.Fatal("empty file should not be ready")
	}

	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if artifactReady(subdir) {
		t.Fatal("directory should not be ready")
	}

	file := filepath.Join(dir, "artifact")
	if err := os.WriteFile(file, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if !artifactReady(file) {
		t.Fatal("non-empty file should be ready")
	}
}
