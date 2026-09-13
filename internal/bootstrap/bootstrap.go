package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ykhoroshevskiy-tech/cell/guestinit"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/ssh"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

func artifactReady(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() && st.Size() > 0
}

func isCustomArtifactPath(path, managedDefault string) bool {
	return filepath.Clean(path) != filepath.Clean(managedDefault)
}

func shouldSkipManagedArtifactDownload(configuredPath, managedDefault string, force bool) bool {
	return !force && isCustomArtifactPath(configuredPath, managedDefault) && artifactReady(configuredPath)
}

func managedKernelPath(cfg *config.CellConfig) string {
	return filepath.Join(cfg.ImagesDir, "vmlinux")
}

func managedFirecrackerPath(cfg *config.CellConfig) string {
	return filepath.Join(cfg.ImagesDir, "bin", "firecracker")
}

func rootfsSquashfsStampPath(rootfsPath string) string {
	return rootfsPath + ".squashfs-version"
}

func squashfsBuildStamp(cfg *config.CellConfig) string {
	node := cfg.NodeVersion
	if node == "" {
		node = "v24.20.0"
	}
	uv := cfg.UvVersion
	if uv == "" {
		uv = "0.12.7"
	}
	py := cfg.PythonVersion
	if py == "" {
		py = "3.13"
	}
	return "debootstrap:noble+apt+node:" + node + "+uv:" + uv + "+py:" + py
}

func needsRootfsRebuild(rootfsPath, expectedStamp string, rebuildRequested bool) (bool, error) {
	if rebuildRequested {
		return true, nil
	}
	if !artifactReady(rootfsPath) {
		return true, nil
	}
	data, err := os.ReadFile(rootfsSquashfsStampPath(rootfsPath))
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return strings.TrimSpace(string(data)) != expectedStamp, nil
}

func writeRootfsSquashfsStamp(rootfsPath, stamp string) error {
	return os.WriteFile(rootfsSquashfsStampPath(rootfsPath), []byte(stamp+"\n"), 0644)
}

func Ensure(cfg *config.CellConfig, force, rebuildRootfs bool) error {
	if err := os.MkdirAll(cfg.ImagesDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(cfg.ImagesDir, "bin"), 0755); err != nil {
		return err
	}
	initScriptsDir := filepath.Join(cfg.DataDir, "init-scripts")
	if err := os.MkdirAll(initScriptsDir, 0755); err != nil {
		return err
	}
	if err := ensureAuthorizedKeys(cfg, initScriptsDir); err != nil {
		return err
	}

	arch, err := firecrackerArch()
	if err != nil {
		return err
	}
	pins := ArtifactPins{
		CIPrefix:           cfg.CIPrefix,
		KernelVersion:      cfg.KernelVersion,
		FirecrackerVersion: cfg.FirecrackerVersion,
		SquashfsVersion:    cfg.SquashfsVersion,
	}
	kernelArt, fcArt, _, err := resolveArtifacts(arch, pins)
	if err != nil {
		return err
	}

	kernelVersioned := filepath.Join(cfg.ImagesDir, "vmlinux-"+kernelArt.Version)
	fcVersioned := filepath.Join(cfg.ImagesDir, "bin", "firecracker-"+fcArt.Version)
	fcTgz := filepath.Join(cfg.ImagesDir, "firecracker-"+fcArt.Version+".tgz")

	if force {
		_ = os.Remove(kernelVersioned)
		_ = os.Remove(fcVersioned)
		_ = os.Remove(fcTgz)
		_ = os.Remove(cfg.RootfsPath)
		_ = os.Remove(rootfsSquashfsStampPath(cfg.RootfsPath))
	}

	if shouldSkipManagedArtifactDownload(cfg.KernelPath, managedKernelPath(cfg), force) {
		st, _ := os.Stat(cfg.KernelPath)
		fmt.Printf("✓ kernel cached (%s)\n", humanSize(st.Size()))
	} else {
		if err := download(kernelVersioned, kernelArt); err != nil {
			return fmt.Errorf("download %s failed (ci_prefix=%s kernel=%s firecracker=%s): %w",
				kernelArt.Name, pins.CIPrefix, pins.KernelVersion, pins.FirecrackerVersion, err)
		}
		if err := ValidateKernel(kernelVersioned); err != nil {
			return err
		}
		_ = ensureSymlink(cfg.KernelPath, kernelVersioned)
	}

	if shouldSkipManagedArtifactDownload(cfg.FirecrackerBin, managedFirecrackerPath(cfg), force) {
		st, _ := os.Stat(cfg.FirecrackerBin)
		fmt.Printf("✓ firecracker cached (%s)\n", humanSize(st.Size()))
	} else {
		if err := download(fcTgz, fcArt); err != nil {
			return fmt.Errorf("download %s failed (ci_prefix=%s kernel=%s firecracker=%s): %w",
				fcArt.Name, pins.CIPrefix, pins.KernelVersion, pins.FirecrackerVersion, err)
		}
		if st, err := os.Stat(fcVersioned); err == nil && st.Size() > 0 {
			fmt.Printf("✓ firecracker binary cached (%s)\n", humanSize(st.Size()))
		} else {
			fmt.Printf("extracting firecracker…\n")
			if err := extractFirecracker(fcTgz, fcVersioned); err != nil {
				return err
			}
			if err := os.Chmod(fcVersioned, 0755); err != nil {
				return err
			}
			fcStat, _ := os.Stat(fcVersioned)
			fmt.Printf("✓ firecracker %s\n", humanSize(fcStat.Size()))
		}
		_ = ensureSymlink(cfg.FirecrackerBin, fcVersioned)
	}

	needsRootfs, err := needsRootfsRebuild(cfg.RootfsPath, squashfsBuildStamp(cfg), rebuildRootfs || cfg.RebuildRootfs)
	if err != nil {
		return err
	}
	if needsRootfs {
		fmt.Println("Building rootfs via debootstrap noble…")
		if err := buildRootfs(cfg, rootfsSizeMB(cfg)); err != nil {
			return err
		}
		if err := writeRootfsSquashfsStamp(cfg.RootfsPath, squashfsBuildStamp(cfg)); err != nil {
			return err
		}
		rStat, _ := os.Stat(cfg.RootfsPath)
		fmt.Printf("✓ rootfs %s\n", humanSize(rStat.Size()))
	} else {
		st, _ := os.Stat(cfg.RootfsPath)
		fmt.Printf("✓ rootfs cached (%s)\n", humanSize(st.Size()))
	}
	return nil
}

func ensureAuthorizedKeys(cfg *config.CellConfig, dir string) error {
	path := filepath.Join(dir, "authorized_keys")
	if cfg.SSHPublicKey != "" {
		if strings.TrimSpace(cfg.SSHPublicKey) == "" {
			return fmt.Errorf("authorized_keys would be empty")
		}
		return os.WriteFile(path, []byte(strings.TrimSpace(cfg.SSHPublicKey)+"\n"), 0600)
	}
	if data, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		return nil
	}
	kp, err := ssh.WriteKeyPair(dir)
	if err != nil {
		return err
	}
	pub, err := os.ReadFile(kp.PublicKeyPath)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(pub))) == 0 {
		return fmt.Errorf("authorized_keys would be empty after key generation")
	}
	return os.WriteFile(path, pub, 0600)
}

func rootfsSizeMB(cfg *config.CellConfig) int {
	if cfg.RootfsSizeMB > 0 {
		return cfg.RootfsSizeMB
	}
	return 4096
}

func buildRootfs(cfg *config.CellConfig, sizeMB int) error {
	rootfsPath := cfg.RootfsPath
	imagesDir := cfg.ImagesDir
	initScriptsDir := filepath.Join(cfg.DataDir, "init-scripts")
	workDir, err := os.MkdirTemp("", "cell-rootfs-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	root := filepath.Join(workDir, "root")
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	if _, err := exec.LookPath("debootstrap"); err != nil {
		return fmt.Errorf("debootstrap not on PATH (install the debootstrap package): %w", err)
	}
	fmt.Println("debootstrap noble (minbase)…")
	deboot := exec.Command("debootstrap", "--variant=minbase", "noble", root, "http://archive.ubuntu.com/ubuntu")
	deboot.Stdout = os.Stdout
	deboot.Stderr = os.Stderr
	if err := deboot.Run(); err != nil {
		return fmt.Errorf("debootstrap: %w", err)
	}
	if err := writeNobleSourcesList(root); err != nil {
		return err
	}

	authKey := filepath.Join(initScriptsDir, "authorized_keys")
	data, err := os.ReadFile(authKey)
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		return fmt.Errorf("authorized_keys missing or empty at %s", authKey)
	}

	if resolv, err := os.ReadFile("/etc/resolv.conf"); err == nil {
		_ = os.MkdirAll(filepath.Join(root, "etc"), 0755)
		_ = os.WriteFile(filepath.Join(root, "etc", "resolv.conf"), resolv, 0644)
	}

	for _, d := range []string{"project", "run/sshd", "tmp", "opt/guest-init", "opt/opencode-plugins"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			return err
		}
	}

	type mnt struct {
		args []string
		dst  string
	}
	mounts := []mnt{
		{[]string{"-t", "proc", "proc", filepath.Join(root, "proc")}, filepath.Join(root, "proc")},
		{[]string{"-t", "sysfs", "sysfs", filepath.Join(root, "sys")}, filepath.Join(root, "sys")},
		{[]string{"--bind", "/dev", filepath.Join(root, "dev")}, filepath.Join(root, "dev")},
		{[]string{"-t", "tmpfs", "tmpfs", filepath.Join(root, "tmp")}, filepath.Join(root, "tmp")},
	}
	var mountDsts []string
	for _, m := range mounts {
		_ = os.MkdirAll(m.dst, 0755)
		if err := runCmd("mount", m.args...); err != nil {
			return fmt.Errorf("mount %s: %w", m.dst, err)
		}
		mountDsts = append(mountDsts, m.dst)
		defer func(dst string) { _ = exec.Command("umount", dst).Run() }(m.dst)
	}

	guestInitDir := filepath.Join(root, "opt", "guest-init")
	for _, name := range []string{"guest-entry.sh", "tmux-attach.sh"} {
		content, err := guestinit.Scripts.ReadFile(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(guestInitDir, name), content, 0755); err != nil {
			return err
		}
	}

	sshdCfg := `Port 22
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes
AuthorizedKeysFile /project/.filter/authorized_keys
UsePAM yes
PidFile /run/sshd/sshd.pid
Subsystem sftp /usr/lib/openssh/sftp-server
`
	if err := os.MkdirAll(filepath.Join(root, "etc", "ssh"), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "etc", "ssh", "sshd_config"), []byte(sshdCfg), 0644); err != nil {
		return err
	}

	fmt.Println("chroot: apt-get install guest packages…")
	if err := runChroot(root, "/bin/bash", "-c", guestAptInstallScript()); err != nil {
		return fmt.Errorf("chroot apt-get: %w", err)
	}
	if err := installNodeHostSide(root, imagesDir, cfg.NodeVersion); err != nil {
		return err
	}
	if err := installUvHostSide(root, imagesDir, cfg.UvVersion); err != nil {
		return err
	}
	fmt.Println("chroot: uv python install…")
	if err := runChroot(root, "/bin/bash", "-c", guestPythonInstallScript(cfg.PythonVersion)); err != nil {
		return fmt.Errorf("chroot uv python: %w", err)
	}
	fmt.Println("chroot: npm install superpowers…")
	if err := runChroot(root, "/bin/bash", "-c", guestSuperpowersInstallScript()); err != nil {
		return fmt.Errorf("chroot npm superpowers: %w", err)
	}
	if err := writeAgentSudoers(root); err != nil {
		return err
	}

	customize := guestCustomizeScript()
	fmt.Println("chroot: customize (user, verify binaries)…")
	if err := runChroot(root, "/bin/bash", "-c", customize); err != nil {
		return fmt.Errorf("chroot customize: %w", err)
	}

	if strings.TrimSpace(cfg.AgentURL) == "" {
		fmt.Println("agent install skipped (CELL_AGENT_URL empty)")
	} else {
		fmt.Printf("installing agent %q (host download)…\n", cfg.AgentBin)
		if err := installAgentHostSide(root, imagesDir, cfg.AgentURL, cfg.AgentBin); err != nil {
			fmt.Printf("  agent download failed: %v; installing stub\n", err)
			writeAgentStub(root, cfg.AgentBin)
		}
	}

	// Unmount before mkfs.ext4 -d — populate must not walk mounted /proc,/sys,/dev,/tmp.
	for i := len(mountDsts) - 1; i >= 0; i-- {
		if err := runCmd("umount", mountDsts[i]); err != nil {
			return fmt.Errorf("umount %s before mkfs: %w", mountDsts[i], err)
		}
	}

	_ = os.Remove(rootfsPath)
	fmt.Printf("building rootfs.ext4 (%d MiB)…\n", sizeMB)
	if err := runCmd("truncate", "-s", fmt.Sprintf("%dM", sizeMB), rootfsPath); err != nil {
		return err
	}
	mkfs := exec.Command("mkfs.ext4", "-F", "-d", root, rootfsPath)
	if verbose.Enabled() {
		mkfs.Stdout = os.Stdout
		mkfs.Stderr = os.Stderr
		if err := mkfs.Run(); err != nil {
			return fmt.Errorf("mkfs.ext4 rootfs: %w", err)
		}
	} else {
		mkfsOut, err := mkfs.CombinedOutput()
		if err != nil {
			return fmt.Errorf("mkfs.ext4 rootfs: %w\n%s", err, mkfsOut)
		}
		for _, line := range strings.Split(string(mkfsOut), "\n") {
			if strings.Contains(line, "__populate_fs: symlink increased in size") || line == "" {
				continue
			}
			fmt.Println(line)
		}
	}
	fmt.Println("rootfs built")
	return nil
}


func agentDownloadTarget() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "linux-x64-baseline", nil
	case "arm64":
		return "linux-arm64-baseline", nil
	default:
		return "", fmt.Errorf("unsupported arch %s", runtime.GOARCH)
	}
}

func installAgentHostSide(root, imagesDir, urlTemplate, binName string) error {
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
	defer os.RemoveAll(tmp)
	if err := runCmd("tar", "-xzf", cache, "-C", tmp); err != nil {
		_ = os.Remove(cache) // corrupt cache → refetch next time
		return fmt.Errorf("extract agent: %w", err)
	}
	src := filepath.Join(tmp, binName)
	if st, err := os.Stat(src); err != nil || st.IsDir() {
		return fmt.Errorf("agent binary %q missing in tarball", binName)
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

// curlDownloadRetry fetches url into dst via curl (atomic rename), up to attempts times.
func curlDownloadRetry(dst, url string, attempts int) error {
	backoffs := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	var lastErr error
	tmp := dst + ".tmp"
	for i := 0; i < attempts; i++ {
		if i > 0 {
			fmt.Printf("  retry %d/%d after %v…\n", i+1, attempts, backoffs[i-1])
			time.Sleep(backoffs[i-1])
		}
		_ = os.Remove(tmp)
		cmd := exec.Command("curl", "-fsSL", "--connect-timeout", "15", "--max-time", "120",
			"-o", tmp, url)
		out, err := cmd.CombinedOutput()
		if err != nil {
			lastErr = fmt.Errorf("curl: %w\n%s", err, strings.TrimSpace(string(out)))
			_ = os.Remove(tmp)
			continue
		}
		if st, err := os.Stat(tmp); err != nil || st.Size() == 0 {
			lastErr = fmt.Errorf("curl wrote empty file")
			_ = os.Remove(tmp)
			continue
		}
		if err := os.Rename(tmp, dst); err != nil {
			return err
		}
		st, _ := os.Stat(dst)
		fmt.Printf("  downloaded %s (%s)\n", dst, humanSize(st.Size()))
		return nil
	}
	return fmt.Errorf("failed after %d attempts: %w", attempts, lastErr)
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
	defer os.RemoveAll(tmp)
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

func guestPythonInstallScript(pythonVersion string) string {
	if pythonVersion == "" {
		pythonVersion = "3.13"
	}
	return `set -eux
export PATH=/usr/local/bin:/usr/bin:/bin
export UV_PYTHON_INSTALL_DIR=/usr/local/share/uv/python
export UV_PYTHON_BIN_DIR=/usr/local/bin
uv python install --default ` + pythonVersion + `
command -v python3
`
}

const superpowersNPMSpec = "superpowers@git+https://github.com/obra/superpowers.git"

func nobleSourcesList() string {
	mirror := "http://archive.ubuntu.com/ubuntu"
	if runtime.GOARCH == "arm64" {
		mirror = "http://ports.ubuntu.com/ubuntu-ports"
	}
	return fmt.Sprintf(`deb %s noble main universe
deb %s noble-updates main universe
deb %s noble-security main universe
`, mirror, mirror, mirror)
}

func writeNobleSourcesList(root string) error {
	dir := filepath.Join(root, "etc", "apt")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "sources.list"), []byte(nobleSourcesList()), 0644)
}

func guestAptInstallScript() string {
	return `set -eux
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends \
  openssh-server tmux zsh rsync curl git sudo ca-certificates
ssh-keygen -A
`
}

func guestSuperpowersInstallScript() string {
	return `set -eux
export PATH=/usr/local/bin:/usr/bin:/bin
npm install --prefix /opt/opencode-plugins ` + superpowersNPMSpec + `
test -d /opt/opencode-plugins/node_modules/superpowers
`
}

func guestAptPackages() []string {
	return []string{"openssh-server", "tmux", "zsh", "rsync", "curl", "git", "sudo", "ca-certificates"}
}

func agentSudoersBody() string {
	return "agent ALL=(ALL) NOPASSWD:ALL\nDefaults:agent !requiretty\n"
}

func writeAgentSudoers(root string) error {
	dir := filepath.Join(root, "etc", "sudoers.d")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "agent")
	return os.WriteFile(path, []byte(agentSudoersBody()), 0440)
}

func guestCustomizeScript() string {
	return `
set -eux
id agent >/dev/null 2>&1 || useradd -m -s /bin/zsh agent
mkdir -p /run/sshd
command -v tmux
command -v rsync
command -v curl
command -v sshd
command -v zsh
command -v git
command -v sudo
command -v node
command -v npm
command -v uv
command -v python3
su - agent -c 'sudo -n true'
test -d /opt/opencode-plugins/node_modules/superpowers
test -x /opt/guest-init/guest-entry.sh
if ldd /usr/bin/rsync 2>/dev/null | grep -q 'not found'; then
  echo "rsync has unresolved shared libraries:" >&2
  ldd /usr/bin/rsync >&2 || true
  exit 1
fi
`
}

func guestPathExists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, rel))
	return err == nil
}

func runChroot(root string, args ...string) error {
	cmd := exec.Command("chroot", append([]string{root}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if verbose.Enabled() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", name, err, out)
	}
	return nil
}

func extractFirecracker(tgzPath, dstBin string) error {
	extractDir := filepath.Join(filepath.Dir(dstBin), "extract-tmp")
	_ = os.RemoveAll(extractDir)
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(extractDir)

	cmd := exec.Command("tar", "-xzf", tgzPath, "-C", extractDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract firecracker tgz: %w\n%s", err, out)
	}

	var candidates []string
	var found string
	_ = filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if !strings.HasPrefix(base, "firecracker-") {
			return nil
		}
		candidates = append(candidates, path)
		if strings.HasSuffix(base, ".debug") {
			return nil
		}
		if found == "" {
			found = path
		}
		return nil
	})
	if found == "" {
		return fmt.Errorf("firecracker binary not found after extract; candidates: %v", candidates)
	}

	if err := os.MkdirAll(filepath.Dir(dstBin), 0755); err != nil {
		return err
	}
	_ = os.Remove(dstBin)
	data, err := os.ReadFile(found)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dstBin, data, 0755); err != nil {
		return err
	}
	return nil
}
