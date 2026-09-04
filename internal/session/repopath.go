package session

import (
	"fmt"
	"os"
	"path/filepath"
)

func ResolveRepoSource(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("repo path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	fi, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("repo path is not a directory: %s", abs)
	}
	return abs, nil
}
