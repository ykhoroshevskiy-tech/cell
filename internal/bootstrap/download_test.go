package bootstrap

import (
	"os"
	"testing"
)

func TestIsTTYOnPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if isTTY(w) {
		t.Fatal("pipe write end should not be a TTY")
	}
}
