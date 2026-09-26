package cli

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestVersionCmdPrintsToStdout(t *testing.T) {
	cmd := newVersionCmd()
	cmd.SetArgs([]string{"version"})

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	execErr := cmd.Execute()

	_ = w.Close()
	os.Stdout = oldStdout
	data, _ := io.ReadAll(r)
	out := string(data)

	if execErr != nil {
		t.Fatalf("Execute() error = %v", execErr)
	}
	if !strings.HasPrefix(out, "cell ") {
		t.Fatalf("version output on stdout = %q, want prefix \"cell \"", out)
	}
}
