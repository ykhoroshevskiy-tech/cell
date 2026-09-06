package session

import (
	"os"
	"strings"
	"testing"
)

func TestCheckKvmAccessMissing(t *testing.T) {
	if _, err := os.Stat("/dev/kvm"); err == nil {
		t.Skip("/dev/kvm present")
	}
	err := checkKvmAccess()
	if err == nil {
		t.Fatal("expected error without /dev/kvm")
	}
	if !strings.Contains(err.Error(), "kvm") {
		t.Fatalf("err=%v", err)
	}
}
