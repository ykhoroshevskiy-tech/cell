package session

import (
	"os"
	"path/filepath"
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
