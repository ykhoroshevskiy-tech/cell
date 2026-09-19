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

// artifactSHA256s holds known SHA256 sums of the pinned artifacts, per arch,
// from the artifact publisher (GitHub release digests for firecracker,
// computed sums for the CI kernel). Verified at download time.
//
// Keep in sync with the default pins in internal/config (guarded by a test).
var artifactSHA256s = map[string]map[string]string{
	"kernel": {
		"x86_64":  "489f209ae3542925043f09e75787cbffaf5a448fe211ae27b80faefd4a9cd38f",
		"aarch64": "423d0bb6dae467445ea203c9ef74027bb49f5e71a1da658fab2a369b736b83e1",
	},
	"firecracker": {
		"x86_64":  "382a02a869e4d6d5cb14c40577f9545e8458021ea8b0b2d3fc10ec14d9c242e6",
		"aarch64": "8d0e69f6d6f9a1724551f607f18504052c16c1828ee3d4d7b6e6c73380871e0e",
	},
}

const (
	builtinCIPrefix           = "firecracker-ci/20260708-f11c230ed107-0/"
	builtinKernelVersion      = "6.1.176"
	builtinFirecrackerVersion = "v1.16.1"
)

// knownArtifactSHA256 returns the embedded hash when the pins match the
// shipped defaults. Env-overridden pins point at different files, so their
// sums are unknown and verification is skipped (see download()).
func knownArtifactSHA256(name, arch string, pins ArtifactPins) string {
	if pins.CIPrefix != builtinCIPrefix || pins.KernelVersion != builtinKernelVersion ||
		pins.FirecrackerVersion != builtinFirecrackerVersion {
		return ""
	}
	return artifactSHA256s[name][arch]
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
		SHA256:  knownArtifactSHA256("kernel", arch, pins),
	}
	firecracker := Artifact{
		Name:    "firecracker",
		Version: pins.FirecrackerVersion,
		URL: fmt.Sprintf("https://github.com/firecracker-microvm/firecracker/releases/download/%s/firecracker-%s-%s.tgz",
			pins.FirecrackerVersion, pins.FirecrackerVersion, arch),
		SHA256: knownArtifactSHA256("firecracker", arch, pins),
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
