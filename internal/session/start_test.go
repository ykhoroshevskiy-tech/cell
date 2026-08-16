package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

func TestStartPreflightMissingDisk(t *testing.T) {
	s := &models.SessionRecord{ProjectDiskPath: filepath.Join(t.TempDir(), "nope.ext4")}
	err := StartPreflight(s, false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStartPreflightAlreadyRunning(t *testing.T) {
	s := &models.SessionRecord{ProjectDiskPath: filepath.Join(t.TempDir(), "disk.ext4")}
	if err := os.WriteFile(s.ProjectDiskPath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	err := StartPreflight(s, true)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("err=%v", err)
	}
}

func TestStartPreflightOK(t *testing.T) {
	s := &models.SessionRecord{ProjectDiskPath: filepath.Join(t.TempDir(), "disk.ext4")}
	if err := os.WriteFile(s.ProjectDiskPath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := StartPreflight(s, false); err != nil {
		t.Fatalf("err=%v", err)
	}
}
