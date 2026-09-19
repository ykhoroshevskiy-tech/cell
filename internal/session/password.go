package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	GuestPasswordRel  = ".filter/opencode-server.pass"
	GuestServePortRel = ".filter/opencode-serve.port"
)

func ServerPasswordPath(sessionDir string) string {
	return filepath.Join(sessionDir, "opencode-server.pass")
}

func WriteServerPassword(sessionDir string) (string, error) {
	path := ServerPasswordPath(sessionDir)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", err
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	pw := hex.EncodeToString(buf)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			return ReadServerPassword(sessionDir)
		}
		return "", err
	}
	if _, err := f.Write([]byte(pw + "\n")); err != nil {
		_ = f.Close()
		_ = os.Remove(path)

		return "", err
	}
	if err := f.Close(); err != nil {
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

func WriteServePortToDiskRoot(diskRoot string, port int) error {
	dst := filepath.Join(diskRoot, GuestServePortRel)
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(dst, []byte(fmt.Sprintf("%d\n", port)), 0600); err != nil {
		return err
	}
	return os.Chmod(dst, 0600)
}

func CopyPasswordToDiskRoot(sessionDir, diskRoot string) error {
	pw, err := ReadServerPassword(sessionDir)
	if err != nil {
		return err
	}
	dst := filepath.Join(diskRoot, GuestPasswordRel)
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(dst, []byte(pw+"\n"), 0600); err != nil {
		return err
	}
	return os.Chmod(dst, 0600)
}
