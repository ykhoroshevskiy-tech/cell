package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const GuestAgentConfigRel = ".cell/opencode.json"

func ResolveAgentConfig(path string) (string, string, error) {
	if path == "" {
		return "", "", fmt.Errorf("agent config path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	abs = filepath.Clean(abs)
	fi, err := os.Stat(abs)
	if err != nil {
		return "", "", err
	}
	if !fi.Mode().IsRegular() {
		return "", "", fmt.Errorf("agent config is not a file: %s", abs)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", "", err
	}
	warn := ""
	if !json.Valid(data) {
		warn = fmt.Sprintf("warning: --config is not valid JSON: %s", abs)
	}
	return abs, warn, nil
}

func CopyAgentConfigToDiskRoot(src, diskRoot string) error {
	if src == "" {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	dst := filepath.Join(diskRoot, GuestAgentConfigRel)
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0600); err != nil {
		return err
	}
	return os.Chmod(dst, 0600)
}
