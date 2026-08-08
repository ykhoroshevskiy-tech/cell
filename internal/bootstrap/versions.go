package bootstrap

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
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

type s3ListBucketResult struct {
	CommonPrefixes []struct {
		Prefix string `xml:"Prefix"`
	} `xml:"CommonPrefixes"`
	Contents []struct {
		Key string `xml:"Key"`
	} `xml:"Contents"`
}

func resolveArtifacts(arch string) (Artifact, Artifact, Artifact, error) {
	prefixes, err := listS3Prefixes("firecracker-ci/")
	if err != nil {
		return Artifact{}, Artifact{}, Artifact{}, err
	}
	prefix, err := selectLatestCIPrefix(prefixes)
	if err != nil {
		return Artifact{}, Artifact{}, Artifact{}, err
	}
	keys, err := listS3Keys(prefix + arch + "/")
	if err != nil {
		return Artifact{}, Artifact{}, Artifact{}, err
	}
	kernelKey, kernelVersion, err := selectLatestKernelKey(keys)
	if err != nil {
		return Artifact{}, Artifact{}, Artifact{}, err
	}
	squashKey, squashVersion, err := selectLatestSquashfsKey(keys)
	if err != nil {
		return Artifact{}, Artifact{}, Artifact{}, err
	}
	fcVersion, err := latestFirecrackerReleaseTag()
	if err != nil {
		return Artifact{}, Artifact{}, Artifact{}, err
	}
	kernel := Artifact{
		Name:    "kernel",
		Version: kernelVersion,
		URL:     specS3Base + "/" + kernelKey,
	}
	firecracker := Artifact{
		Name:    "firecracker",
		Version: fcVersion,
		URL:     fmt.Sprintf("https://github.com/firecracker-microvm/firecracker/releases/download/%s/firecracker-%s-%s.tgz", fcVersion, fcVersion, arch),
	}
	squashfs := Artifact{
		Name:    "ubuntu-squashfs",
		Version: squashVersion,
		URL:     specS3Base + "/" + squashKey,
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

func listS3Prefixes(prefix string) ([]string, error) {
	result, err := fetchS3List(prefix, true)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(result.CommonPrefixes))
	for _, p := range result.CommonPrefixes {
		if p.Prefix != "" {
			out = append(out, p.Prefix)
		}
	}
	return out, nil
}

func listS3Keys(prefix string) ([]string, error) {
	result, err := fetchS3List(prefix, false)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(result.Contents))
	for _, c := range result.Contents {
		if c.Key != "" {
			out = append(out, c.Key)
		}
	}
	return out, nil
}

func fetchS3List(prefix string, delimiter bool) (*s3ListBucketResult, error) {
	url := fmt.Sprintf("%s?list-type=2&prefix=%s", specS3Base, prefix)
	if delimiter {
		url += "&delimiter=/"
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("s3 list %s: HTTP %d", prefix, resp.StatusCode)
	}
	var result s3ListBucketResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
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

func selectLatestKernelKey(keys []string) (string, string, error) {
	re := regexp.MustCompile(`vmlinux-(\d+)\.(\d+)\.(\d+)$`)
	best, version := selectLatestVersionedKey(keys, re)
	if best == "" {
		return "", "", fmt.Errorf("no kernel artifact found")
	}
	return best, version, nil
}

func selectLatestSquashfsKey(keys []string) (string, string, error) {
	re := regexp.MustCompile(`ubuntu-(\d+)\.(\d+)\.squashfs$`)
	best, version := selectLatestVersionedKey(keys, re)
	if best == "" {
		return "", "", fmt.Errorf("no ubuntu squashfs artifact found")
	}
	return best, version, nil
}

func selectLatestVersionedKey(keys []string, re *regexp.Regexp) (string, string) {
	type candidate struct {
		key     string
		version string
		parts   []int
	}
	var candidates []candidate
	for _, key := range keys {
		m := re.FindStringSubmatch(key)
		if m == nil {
			continue
		}
		parts := make([]int, 0, len(m)-1)
		for _, piece := range m[1:] {
			n, _ := strconv.Atoi(piece)
			parts = append(parts, n)
		}
		candidates = append(candidates, candidate{
			key:     key,
			version: strings.Join(m[1:], "."),
			parts:   parts,
		})
	}
	if len(candidates) == 0 {
		return "", ""
	}
	slices.SortFunc(candidates, func(a, b candidate) int {
		for i := 0; i < len(a.parts) && i < len(b.parts); i++ {
			if a.parts[i] != b.parts[i] {
				return a.parts[i] - b.parts[i]
			}
		}
		return len(a.parts) - len(b.parts)
	})
	best := candidates[len(candidates)-1]
	return best.key, best.version
}

func latestFirecrackerReleaseTag() (string, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(firecrackerLatestRelease)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("firecracker latest release: HTTP %d", resp.StatusCode)
	}
	path := strings.TrimSuffix(resp.Request.URL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("cannot parse firecracker release tag from %s", resp.Request.URL.String())
	}
	tag := parts[len(parts)-1]
	if !strings.HasPrefix(tag, "v") {
		return "", fmt.Errorf("unexpected firecracker release tag %q", tag)
	}
	return tag, nil
}
