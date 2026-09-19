package bootstrap

import (
	"fmt"
	"io"
	"os"
)

func ValidateKernel(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("kernel missing: %w", err)
	}
	defer func() { _ = f.Close() }()
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return fmt.Errorf("read kernel header: %w", err)
	}
	if string(hdr) != "\x7fELF" {
		return fmt.Errorf("kernel is not a valid ELF binary: %s", path)
	}
	return nil
}
