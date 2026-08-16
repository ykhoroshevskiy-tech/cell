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

func TestPrepareVMBootArtifacts(t *testing.T) {
	dir := t.TempDir()
	s := &models.SessionRecord{SessionID: "abc123"}
	s.ArtifactPaths(dir)
	if err := os.MkdirAll(s.SessionDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.SocketPath, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.SerialLogPath, []byte("runtime ready\nkernel panic\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := prepareVMBootArtifacts(s); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if _, err := os.Stat(s.SocketPath); !os.IsNotExist(err) {
		t.Fatalf("socket still exists: %v", err)
	}
	data, err := os.ReadFile(s.SerialLogPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("serial log not truncated: %q", data)
	}
}
