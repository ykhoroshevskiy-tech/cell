package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

// agentInstall describes one agent to place into a fresh rootfs.
type agentInstall struct {
	kind string
	url  string
	bin  string
}

// agentInstalls returns every agent to install into a fresh rootfs: opencode
// (skipped when CELL_AGENT_URL is set empty) plus claude, so any per-session
// --agent works without rebuilding the rootfs.
func agentInstalls(cfg *config.CellConfig) []agentInstall {
	var installs []agentInstall
	if strings.TrimSpace(cfg.AgentURL) != "" {
		bin := cfg.AgentBin
		if bin == "" {
			bin = "opencode"
		}
		installs = append(installs, agentInstall{kind: models.AgentKindOpenCode, url: cfg.AgentURL, bin: bin})
	}
	return append(installs, agentInstall{kind: models.AgentKindClaude, url: config.DefaultClaudeAgentURL(), bin: "claude"})
}

// installAgentsHostSide downloads and installs every agent; a failed download
// leaves that agent's stub in place so the rootfs stays bootable.
func installAgentsHostSide(root, imagesDir string, cfg *config.CellConfig) {
	for _, ai := range agentInstalls(cfg) {
		fmt.Printf("installing agent %q (host download)…\n", ai.bin)
		if err := installAgentHostSide(root, imagesDir, ai.url, ai.bin, ai.kind); err != nil {
			fmt.Printf("  agent download failed: %v; installing stub\n", err)
			writeAgentStub(root, ai.bin)
		}
	}
}

func agentDownloadTarget() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "linux-x64-baseline", nil
	case "arm64":
		return "linux-arm64", nil
	default:
		return "", fmt.Errorf("unsupported arch %s", runtime.GOARCH)
	}
}

func installAgentHostSide(root, imagesDir, urlTemplate, binName, kind string) error {
	if binName == "" {
		return fmt.Errorf("agent_bin is empty")
	}
	target, err := agentDownloadTarget()
	if err != nil {
		return err
	}
	url := strings.ReplaceAll(urlTemplate, "{target}", target)
	cache := filepath.Join(imagesDir, binName+"-"+target+".tar.gz")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return err
	}
	if st, err := os.Stat(cache); err != nil || st.Size() == 0 {
		fmt.Printf("  fetching %s\n", url)
		if err := curlDownloadRetry(cache, url, 3); err != nil {
			return err
		}
	} else {
		fmt.Printf("  using cached %s (%s)\n", cache, humanSize(st.Size()))
	}

	tmp, err := os.MkdirTemp("", "cell-agent-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := runCmd("tar", "-xzf", cache, "-C", tmp); err != nil {
		_ = os.Remove(cache) // corrupt cache → refetch next time
		return fmt.Errorf("extract agent: %w", err)
	}
	if models.NormalizeAgentKind(kind) == models.AgentKindClaude {
		return installClaudeFromExtract(root, tmp)
	}
	return installOpencodeFromExtract(root, tmp, binName)
}

// installOpencodeFromExtract installs the opencode tarball layout: a single
// binary named binName at the tarball root, exposed at /opt/agent/bin and
// symlinked from /usr/local/bin.
func installOpencodeFromExtract(root, tmpDir, binName string) error {
	src, err := findAgentBinary(tmpDir, binName)
	if err != nil {
		return err
	}
	binDir := filepath.Join(root, "opt", "agent", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "usr", "local", "bin"), 0755); err != nil {
		return err
	}
	dst := filepath.Join(binDir, binName)
	if err := runCmd("install", "-m", "755", src, dst); err != nil {
		return fmt.Errorf("install agent binary: %w", err)
	}
	link := filepath.Join(root, "usr", "local", "bin", binName)
	_ = os.Remove(link)
	if err := os.Symlink("/opt/agent/bin/"+binName, link); err != nil {
		return err
	}
	fmt.Printf("  agent %q installed\n", binName)
	return nil
}

// installClaudeFromExtract installs the npm tarball layout used by
// @anthropic-ai/claude-code: the package tree lands under
// /opt/agent/claude-code and /usr/local/bin/claude becomes a node wrapper.
func installClaudeFromExtract(root, tmpDir string) error {
	pkg := filepath.Join(tmpDir, "package")
	cli := filepath.Join(pkg, "cli.js")
	if st, err := os.Stat(cli); err != nil || st.IsDir() {
		return fmt.Errorf("claude tarball missing package/cli.js")
	}
	dst := filepath.Join(root, "opt", "agent", "claude-code")
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if err := runCmd("cp", "-a", pkg, dst); err != nil {
		return fmt.Errorf("install claude package: %w", err)
	}
	binDir := filepath.Join(root, "usr", "local", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}
	wrapper := filepath.Join(binDir, "claude")
	_ = os.Remove(wrapper)
	body := "#!/bin/sh\nexec node /opt/agent/claude-code/cli.js \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(body), 0755); err != nil {
		return err
	}
	if err := os.Chmod(wrapper, 0755); err != nil {
		return err
	}
	fmt.Printf("  agent %q installed\n", "claude")
	return nil
}

// findAgentBinary locates the agent binary inside an extracted tarball.
// Opencode tarballs carry the binary at the tarball root; npm tarballs
// (e.g. @anthropic-ai/claude-code) extract under a "package/" prefix.
func findAgentBinary(tmpDir, binName string) (string, error) {
	for _, rel := range []string{binName, filepath.Join("package", binName)} {
		src := filepath.Join(tmpDir, rel)
		if st, err := os.Stat(src); err == nil && !st.IsDir() {
			return src, nil
		}
	}
	var found string
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || found != "" {
			return nil
		}
		if filepath.Base(path) == binName {
			found = path
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("agent binary %q missing in tarball", binName)
	}
	return found, nil
}
func writeAgentStub(root, binName string) {
	if binName == "" {
		binName = "agent"
	}
	stub := filepath.Join(root, "usr", "local", "bin", binName)
	_ = os.MkdirAll(filepath.Dir(stub), 0755)
	body := "#!/bin/sh\necho \"[agent stub] install failed during bootstrap; run: cell bootstrap --force\" >&2\nexec /bin/zsh\n"
	_ = os.WriteFile(stub, []byte(body), 0755)
}

func nodeDownloadTarget() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "linux-x64", nil
	case "arm64":
		return "linux-arm64", nil
	default:
		return "", fmt.Errorf("unsupported arch %s", runtime.GOARCH)
	}
}

func nodeTarballURL(version, target string) string {
	return fmt.Sprintf("https://nodejs.org/dist/%s/node-%s-%s.tar.xz", version, version, target)
}

func installNodeHostSide(root, imagesDir, version string) error {
	if version == "" {
		version = "v24.20.0"
	}
	target, err := nodeDownloadTarget()
	if err != nil {
		return err
	}
	url := nodeTarballURL(version, target)
	cache := filepath.Join(imagesDir, "node-"+version+"-"+target+".tar.xz")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return err
	}
	if st, err := os.Stat(cache); err != nil || st.Size() == 0 {
		fmt.Printf("  fetching %s\n", url)
		if err := curlDownloadRetry(cache, url, 3); err != nil {
			return err
		}
	} else {
		fmt.Printf("  using cached %s (%s)\n", cache, humanSize(st.Size()))
	}
	usrLocal := filepath.Join(root, "usr", "local")
	if err := os.MkdirAll(usrLocal, 0755); err != nil {
		return err
	}
	fmt.Println("  extracting node into /usr/local…")
	if err := runCmd("tar", "-xJf", cache, "-C", usrLocal, "--strip-components=1"); err != nil {
		_ = os.Remove(cache)
		return fmt.Errorf("extract node: %w", err)
	}
	if !guestPathExists(root, "usr/local/bin/node") {
		return fmt.Errorf("node missing at /usr/local/bin/node after extract")
	}
	fmt.Printf("  node %s installed\n", version)
	return nil
}

func uvDownloadTarget() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "aarch64", nil
	default:
		return "", fmt.Errorf("unsupported arch %s", runtime.GOARCH)
	}
}

func uvTarballURL(version, arch string) string {
	return fmt.Sprintf("https://github.com/astral-sh/uv/releases/download/%s/uv-%s-unknown-linux-gnu.tar.gz", version, arch)
}

func installUvHostSide(root, imagesDir, version string) error {
	if version == "" {
		version = "0.12.7"
	}
	arch, err := uvDownloadTarget()
	if err != nil {
		return err
	}
	url := uvTarballURL(version, arch)
	cache := filepath.Join(imagesDir, "uv-"+version+"-"+arch+".tar.gz")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return err
	}
	if st, err := os.Stat(cache); err != nil || st.Size() == 0 {
		fmt.Printf("  fetching %s\n", url)
		if err := curlDownloadRetry(cache, url, 3); err != nil {
			return err
		}
	} else {
		fmt.Printf("  using cached %s (%s)\n", cache, humanSize(st.Size()))
	}

	tmp, err := os.MkdirTemp("", "cell-uv-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := runCmd("tar", "-xzf", cache, "-C", tmp); err != nil {
		_ = os.Remove(cache)
		return fmt.Errorf("extract uv: %w", err)
	}
	var src string
	_ = filepath.Walk(tmp, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Base(path) == "uv" && src == "" {
			src = path
		}
		return nil
	})
	if src == "" {
		return fmt.Errorf("uv binary missing in tarball")
	}
	binDir := filepath.Join(root, "usr", "local", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}
	dst := filepath.Join(binDir, "uv")
	if err := runCmd("install", "-m", "755", src, dst); err != nil {
		return fmt.Errorf("install uv binary: %w", err)
	}
	fmt.Printf("  uv %s installed\n", version)
	return nil
}
