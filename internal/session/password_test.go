package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteServerPasswordModeAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	pw, err := WriteServerPassword(dir)
	if err != nil {
		t.Fatalf("WriteServerPassword: %v", err)
	}
	if len(pw) < 16 {
		t.Fatalf("password too short: %q", pw)
	}
	st, err := os.Stat(ServerPasswordPath(dir))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatalf("perm = %o, want 0600", st.Mode().Perm())
	}
	got, err := ReadServerPassword(dir)
	if err != nil {
		t.Fatalf("ReadServerPassword: %v", err)
	}
	if got != pw {
		t.Fatalf("got %q want %q", got, pw)
	}
}

func TestWriteServerPasswordEmptyExistingFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(ServerPasswordPath(dir), []byte("  \n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := WriteServerPassword(dir)
	if err == nil {
		t.Fatal("expected error for empty existing file")
	}
}

func TestWriteServerPasswordDoesNotRotate(t *testing.T) {
	dir := t.TempDir()
	first, err := WriteServerPassword(dir)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := WriteServerPassword(dir)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first != second {
		t.Fatalf("rotated password")
	}
	_ = filepath.Separator
}

func TestCopyPasswordToDiskRootFixesExistingPerm(t *testing.T) {
	sessionDir := t.TempDir()
	diskRoot := t.TempDir()
	pw, err := WriteServerPassword(sessionDir)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	dst := filepath.Join(diskRoot, GuestPasswordRel)
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := CopyPasswordToDiskRoot(sessionDir, diskRoot); err != nil {
		t.Fatalf("copy: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read guest copy: %v", err)
	}
	if strings.TrimSpace(string(got)) != pw {
		t.Fatalf("guest copy = %q want %q", got, pw)
	}
	st, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatalf("guest perm = %o, want 0600", st.Mode().Perm())
	}
}

func TestWriteServePortToDiskRoot(t *testing.T) {
	diskRoot := t.TempDir()
	if err := WriteServePortToDiskRoot(diskRoot, 4097); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(diskRoot, GuestServePortRel))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.TrimSpace(string(got)) != "4097" {
		t.Fatalf("port = %q want 4097", got)
	}
	st, _ := os.Stat(filepath.Join(diskRoot, GuestServePortRel))
	if st.Mode().Perm() != 0600 {
		t.Fatalf("perm = %o", st.Mode().Perm())
	}
}

func TestCopyPasswordToDiskRoot(t *testing.T) {
	sessionDir := t.TempDir()
	diskRoot := t.TempDir()
	pw, err := WriteServerPassword(sessionDir)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := CopyPasswordToDiskRoot(sessionDir, diskRoot); err != nil {
		t.Fatalf("copy: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(diskRoot, GuestPasswordRel))
	if err != nil {
		t.Fatalf("read guest copy: %v", err)
	}
	if strings.TrimSpace(string(got)) != pw {
		t.Fatalf("guest copy = %q want %q", got, pw)
	}
	st, _ := os.Stat(filepath.Join(diskRoot, GuestPasswordRel))
	if st.Mode().Perm() != 0600 {
		t.Fatalf("guest perm = %o", st.Mode().Perm())
	}
}
