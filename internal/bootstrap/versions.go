package bootstrap

import (
	"fmt"
	"runtime"
	"strings"
)

const specS3Base = "https://s3.amazonaws.com/spec.ccfc.min"

type Artifact struct {
	Name    string
	Version string
	URL     string
	SHA256  string
}

type ArtifactPins struct {
	CIPrefix           string
	KernelVersion      string
	FirecrackerVersion string
}

func resolveArtifacts(arch string, pins ArtifactPins) (Artifact, Artifact, error) {
	if arch == "" {
		return Artifact{}, Artifact{}, fmt.Errorf("arch is empty")
	}
	if pins.CIPrefix == "" || pins.KernelVersion == "" || pins.FirecrackerVersion == "" {
		return Artifact{}, Artifact{}, fmt.Errorf("incomplete artifact pins: ci_prefix=%q kernel=%q firecracker=%q",
			pins.CIPrefix, pins.KernelVersion, pins.FirecrackerVersion)
	}
	prefix := pins.CIPrefix
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	kernel := Artifact{
		Name:    "kernel",
		Version: pins.KernelVersion,
		URL:     fmt.Sprintf("%s/%s%s/vmlinux-%s", specS3Base, prefix, arch, pins.KernelVersion),
	}
	firecracker := Artifact{
		Name:    "firecracker",
		Version: pins.FirecrackerVersion,
		URL: fmt.Sprintf("https://github.com/firecracker-microvm/firecracker/releases/download/%s/firecracker-%s-%s.tgz",
			pins.FirecrackerVersion, pins.FirecrackerVersion, arch),
	}
	return kernel, firecracker, nil
}

func firecrackerArch() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "aarch64", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
}
