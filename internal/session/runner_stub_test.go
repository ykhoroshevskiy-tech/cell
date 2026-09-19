package session

// silentRunner is a minimal network.Runner stub: host commands succeed
// without touching the system, so session tests exercise their own logic.
type silentRunner struct{}

func (silentRunner) Run(name string, args ...string) ([]byte, error) {
	return nil, nil
}

func (silentRunner) Output(name string, args ...string) ([]byte, error) {
	return nil, nil
}
