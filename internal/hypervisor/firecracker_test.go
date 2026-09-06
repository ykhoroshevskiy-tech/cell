package hypervisor

import (
	"os"
	"strings"
	"testing"
)

func TestStopPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; cannot simulate EPERM")
	}
	f := NewFirecracker("")
	err := f.Stop(1)
	if err == nil {
		t.Fatal("expected EPERM error stopping pid 1")
	}
	if !strings.Contains(err.Error(), "sudo cell stop") {
		t.Fatalf("err=%v", err)
	}
}

func TestStopInvalidPid(t *testing.T) {
	f := NewFirecracker("")
	if err := f.Stop(0); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := f.Stop(-1); err != nil {
		t.Fatalf("err=%v", err)
	}
}
