package ssh

import (
	"os"
	"testing"
)

func TestVerifyFirecrackerInvalidPID(t *testing.T) {
	if VerifyFirecracker(0, "/tmp/sock") {
		t.Fatal("pid 0 should fail")
	}
	if VerifyFirecracker(-1, "/tmp/sock") {
		t.Fatal("negative pid should fail")
	}
	if VerifyFirecracker(os.Getpid(), "") {
		t.Fatal("empty socket should fail")
	}
}

func TestVerifyFirecrackerMismatch(t *testing.T) {
	// Current process is not firecracker
	if VerifyFirecracker(os.Getpid(), "/tmp/nope.sock") {
		t.Fatal("non-firecracker process should fail")
	}
}

func TestVerifyFirecrackerCmdlineMatch(t *testing.T) {
	sock := "/run/cell/fc.sock"
	data := []byte("firecracker\x00--api-sock\x00" + sock + "\x00")
	if !cmdlineMatchesFirecracker(data, sock) {
		t.Fatal("expected cmdline match")
	}
	if cmdlineMatchesFirecracker([]byte("other\x00"), sock) {
		t.Fatal("expected mismatch")
	}
}
