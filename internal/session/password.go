package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const GuestPasswordRel = ".filter/opencode-server.pass"

func ServerPasswordPath(sessionDir string) string {
	return filepath.Join(sessionDir, "opencode-server.pass")
}

func WriteServerPassword(sessionDir string) (string, error) {
	path := ServerPasswordPath(sessionDir)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			return ReadServerPassword(sessionDir)
		}
		return "", err
	}
	defer f.Close()
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	pw := hex.EncodeToString(buf)
	if _, err := f.Write([]byte(pw + "\n")); err != nil {
		return "", err
	}
	return pw, nil
}

func ReadServerPassword(sessionDir string) (string, error) {
	b, err := os.ReadFile(ServerPasswordPath(sessionDir))
	if err != nil {
		return "", fmt.Errorf("read serve password: %w", err)
	}
	pw := strings.TrimSpace(string(b))
	if pw == "" {
		return "", fmt.Errorf("serve password file empty")
	}
	return pw, nil
}
