package network

import (
	"fmt"
	"strings"
	"sync"
)

// fakeRunner records commands for unit tests.
type fakeRunner struct {
	mu       sync.Mutex
	commands []string
	outputs  map[string][]byte
	errors   map[string]error
	links    map[string]bool // tap name -> exists
	bridges  map[string]bool
	chains   map[string]bool // "filter:CHAIN" or "nat:CHAIN" -> exists
}

// NewFakeRunnerForTest returns an in-memory command runner (tests only).
func NewFakeRunnerForTest() Runner {
	return newFakeRunner()
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{
		outputs: make(map[string][]byte),
		errors:  make(map[string]error),
		links:   make(map[string]bool),
		bridges: map[string]bool{"cell0": false},
		chains:  make(map[string]bool),
	}
}

func (f *fakeRunner) key(name string, args ...string) string {
	return name + " " + strings.Join(args, " ")
}

func (f *fakeRunner) record(name string, args ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, f.key(name, args...))
}

func (f *fakeRunner) Run(name string, args ...string) ([]byte, error) {
	f.record(name, args...)
	k := f.key(name, args...)
	if err, ok := f.errors[k]; ok {
		return nil, err
	}
	return f.simulate(name, args...)
}

func (f *fakeRunner) Output(name string, args ...string) ([]byte, error) {
	f.record(name, args...)
	k := f.key(name, args...)
	if out, ok := f.outputs[k]; ok {
		return out, nil
	}
	if err, ok := f.errors[k]; ok {
		return nil, err
	}
	return f.simulate(name, args...)
}

func (f *fakeRunner) simulate(name string, args ...string) ([]byte, error) {
	switch name {
	case "ip":
		if len(args) >= 6 && args[0] == "link" && args[1] == "add" && args[2] == "name" && args[4] == "type" && args[5] == "bridge" {
			f.mu.Lock()
			f.bridges[args[3]] = true
			f.mu.Unlock()
			return nil, nil
		}
		if len(args) >= 4 && args[0] == "tuntap" && args[1] == "add" && args[2] == "dev" {
			dev := args[3]
			f.mu.Lock()
			f.links[dev] = true
			f.mu.Unlock()
			return nil, nil
		}
		if len(args) >= 3 && args[0] == "link" && args[1] == "del" {
			dev := args[2]
			f.mu.Lock()
			delete(f.links, dev)
			delete(f.bridges, dev)
			f.mu.Unlock()
			return nil, nil
		}
		if len(args) >= 2 && args[0] == "link" && args[1] == "show" {
			if len(args) == 3 {
				dev := args[2]
				f.mu.Lock()
				defer f.mu.Unlock()
				if f.links[dev] || f.bridges[dev] {
					return []byte(dev + ": UP\n"), nil
				}
				return nil, fmt.Errorf("device %q does not exist", dev)
			}
			var b strings.Builder
			f.mu.Lock()
			defer f.mu.Unlock()
			for dev := range f.bridges {
				if f.bridges[dev] {
					fmt.Fprintf(&b, "%s: UP\n", dev)
				}
			}
			for dev := range f.links {
				if f.links[dev] {
					fmt.Fprintf(&b, "%s: UP\n", dev)
				}
			}
			return []byte(b.String()), nil
		}
		if len(args) >= 1 && args[0] == "-o" {
			var b strings.Builder
			f.mu.Lock()
			defer f.mu.Unlock()
			i := 0
			for dev := range f.links {
				if f.links[dev] {
					fmt.Fprintf(&b, "%d: %s: UP\n", i, dev)
					i++
				}
			}
			return []byte(b.String()), nil
		}
	case "bridge":
		return nil, nil
	case "iptables":
		return f.simulateIPTables(args...)
	case "test":
		if len(args) >= 3 && args[0] == "-d" && args[1] == "/sys/class/net" {
			dev := args[2]
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.links[dev] || f.bridges[dev] {
				return []byte("ok\n"), nil
			}
			return nil, fmt.Errorf("missing")
		}
	}
	return []byte{}, nil
}

func (f *fakeRunner) chainKey(table, chain string) string {
	return table + ":" + chain
}

func (f *fakeRunner) simulateIPTables(args ...string) ([]byte, error) {
	table := "filter"
	i := 0
	if len(args) >= 2 && args[0] == "-t" {
		table = args[1]
		i = 2
	}
	if i >= len(args) {
		return nil, nil
	}

	switch args[i] {
	case "-nL":
		if i+1 >= len(args) {
			return nil, fmt.Errorf("exit status 1")
		}
		chain := args[i+1]
		f.mu.Lock()
		exists := f.chains[f.chainKey(table, chain)]
		f.mu.Unlock()
		if !exists {
			return nil, fmt.Errorf("exit status 1")
		}
		return []byte("Chain " + chain + "\n"), nil
	case "-N":
		if i+1 >= len(args) {
			return nil, fmt.Errorf("exit status 1")
		}
		chain := args[i+1]
		key := f.chainKey(table, chain)
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.chains[key] {
			return nil, fmt.Errorf("exit status 1")
		}
		f.chains[key] = true
		return nil, nil
	default:
		return nil, nil
	}
}
