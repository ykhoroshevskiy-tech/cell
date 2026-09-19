package network

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

// Runner executes host networking commands (injectable for tests).
type Runner interface {
	Run(name string, args ...string) ([]byte, error)
	Output(name string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

func (execRunner) Output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

var defaultRunner Runner = execRunner{}

// SetRunnerForTest replaces the command runner (tests only).
func SetRunnerForTest(r Runner) func() {
	old := defaultRunner
	defaultRunner = r
	return func() { defaultRunner = old }
}

func run(name string, args ...string) error {
	if verbose.Enabled() {
		verbose.V("net: %s %s", name, strings.Join(args, " "))
	}
	out, err := defaultRunner.Run(name, args...)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, msg)
		}
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func output(name string, args ...string) ([]byte, error) {
	return defaultRunner.Output(name, args...)
}

func linkExists(name string) bool {
	_, err := output("ip", "link", "show", name)
	return err == nil
}

func writeSysctl(path, value string) error {
	return os.WriteFile(path, []byte(value), 0644)
}
