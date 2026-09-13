package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
)

func TestEnsureAuthorizedKeysExistingSkipped(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "authorized_keys"), []byte("ssh-ed25519 AAA test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ensureAuthorizedKeys(&config.CellConfig{}, dir); err != nil {
		t.Fatalf("err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "id_ed25519")); !os.IsNotExist(err) {
		t.Fatal("id_ed25519 must not be created when authorized_keys exists")
	}
}

func TestEnsureAuthorizedKeysRebuildsFromExistingPub(t *testing.T) {
	dir := t.TempDir()
	pub := "ssh-ed25519 AAABBB test@host\n"
	if err := os.WriteFile(filepath.Join(dir, "id_ed25519.pub"), []byte(pub), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "id_ed25519"), []byte("PRIVATE"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ensureAuthorizedKeys(&config.CellConfig{}, dir); err != nil {
		t.Fatalf("err=%v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "authorized_keys"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != pub {
		t.Fatalf("authorized_keys=%q want %q", data, pub)
	}
	priv, err := os.ReadFile(filepath.Join(dir, "id_ed25519"))
	if err != nil || string(priv) != "PRIVATE" {
		t.Fatal("existing private key must not be touched")
	}
}

func TestEnsureAuthorizedKeysGeneratesFresh(t *testing.T) {
	dir := t.TempDir()
	if err := ensureAuthorizedKeys(&config.CellConfig{}, dir); err != nil {
		t.Fatalf("err=%v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "authorized_keys"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "ssh-ed25519 ") {
		t.Fatalf("authorized_keys=%q", data)
	}
}

func TestEnsureAuthorizedKeysConfiguredKey(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.CellConfig{SSHPublicKey: "ssh-ed25519 AAA configured"}
	if err := ensureAuthorizedKeys(cfg, dir); err != nil {
		t.Fatalf("err=%v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "authorized_keys"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ssh-ed25519 AAA configured\n" {
		t.Fatalf("authorized_keys=%q", data)
	}
}
