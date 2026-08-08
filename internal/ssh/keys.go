package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	gossh "golang.org/x/crypto/ssh"
)

type KeyPair struct {
	PrivateKeyPath string
	PublicKeyPath  string
	PublicKeyLine  string
}

func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, string, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, "", err
	}
	pubKey, err := gossh.NewPublicKey(pub)
	if err != nil {
		return nil, nil, "", err
	}
	return pub, priv, string(gossh.MarshalAuthorizedKey(pubKey)), nil
}

func WriteKeyPair(dir string) (*KeyPair, error) {
	pub, priv, pubLine, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate keypair: %w", err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	privPath := filepath.Join(dir, "id_ed25519")
	pubPath := filepath.Join(dir, "id_ed25519.pub")

	privPEM, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(privPEM), 0600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(pubPath, []byte(pubLine), 0644); err != nil {
		return nil, err
	}
	_ = pub
	return &KeyPair{
		PrivateKeyPath: privPath,
		PublicKeyPath:  pubPath,
		PublicKeyLine:  pubLine,
	}, nil
}
