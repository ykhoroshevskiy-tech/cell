package bootstrap

import (
	"fmt"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

const (
	specS3Base               = "https://s3.amazonaws.com/spec.ccfc.min"
	firecrackerLatestRelease = "https://github.com/firecracker-microvm/firecracker/releases/latest"
)

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
	SquashfsVersion    string
}

func resolveArtifacts(arch string, pins ArtifactPins) (Artifact, Artifact, Artifact, error) {
	if arch == "" {
		return Artifact{}, Artifact{}, Artifact{}, fmt.Errorf("arch is empty")
	}
	if pins.CIPrefix == "" || pins.KernelVersion == "" || pins.FirecrackerVersion == "" || pins.SquashfsVersion == "" {
		return Artifact{}, Artifact{}, Artifact{}, fmt.Errorf("incomplete artifact pins: ci_prefix=%q kernel=%q firecracker=%q squashfs=%q",
			pins.CIPrefix, pins.KernelVersion, pins.FirecrackerVersion, pins.SquashfsVersion)
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
	squashfs := Artifact{
		Name:    "ubuntu-squashfs",
		Version: pins.SquashfsVersion,
		URL:     fmt.Sprintf("%s/%s%s/ubuntu-%s.squashfs", specS3Base, prefix, arch, pins.SquashfsVersion),
	}
	return kernel, firecracker, squashfs, nil
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

func selectLatestCIPrefix(prefixes []string) (string, error) {
	dateRe := regexp.MustCompile(`^firecracker-ci/\d{8}-`)
	var dated []string
	for _, prefix := range prefixes {
		if dateRe.MatchString(prefix) {
			dated = append(dated, prefix)
		}
	}
	if len(dated) > 0 {
		slices.Sort(dated)
		return dated[len(dated)-1], nil
	}
	return selectLatestStablePrefix(prefixes)
}

func selectLatestStablePrefix(prefixes []string) (string, error) {
	re := regexp.MustCompile(`^firecracker-ci/v1\.(\d+)/$`)
	bestMinor := -1
	best := ""
	for _, prefix := range prefixes {
		m := re.FindStringSubmatch(prefix)
		if m == nil {
			continue
		}
		minor, _ := strconv.Atoi(m[1])
		if minor > bestMinor {
			bestMinor = minor
			best = prefix
		}
	}
	if best == "" {
		return "", fmt.Errorf("no stable firecracker-ci prefixes found")
	}
	return best, nil
}
