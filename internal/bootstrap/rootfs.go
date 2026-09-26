package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ykhoroshevskiy-tech/cell/guestinit"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

func rootfsBuildStampPath(rootfsPath string) string {
	return rootfsPath + ".build-stamp"
}

func rootfsBuildStamp(cfg *config.CellConfig) string {
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
	sp := "off"
	if cfg.InstallSuperpowers {
		sp = "on"
	}
	return "debootstrap:noble+apt+node:" + node + "+uv:" + uv + "+py:" + py + "+sp:" + sp + "+layout:cell+agent:" + AgentStampAll
}

func needsRootfsRebuild(rootfsPath, expectedStamp string, rebuildRequested bool) (bool, error) {
	if rebuildRequested {
		return true, nil
	}
	if !artifactReady(rootfsPath) {
		return true, nil
	}
	data, err := os.ReadFile(rootfsBuildStampPath(rootfsPath))
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return strings.TrimSpace(string(data)) != expectedStamp, nil
}

func writeRootfsBuildStamp(rootfsPath, stamp string) error {
	return os.WriteFile(rootfsBuildStampPath(rootfsPath), []byte(stamp+"\n"), 0644)
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
	defer func() { _ = os.RemoveAll(workDir) }()

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
AuthorizedKeysFile /project/.cell/authorized_keys
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
	if cfg.InstallSuperpowers {
		fmt.Println("chroot: npm install superpowers…")
		if err := runChroot(root, "/bin/bash", "-c", guestSuperpowersInstallScript()); err != nil {
			return fmt.Errorf("chroot npm superpowers: %w", err)
		}
	}
	if err := writeAgentSudoers(root); err != nil {
		return err
	}

	customize := guestCustomizeScript(cfg)
	fmt.Println("chroot: customize (user, verify binaries)…")
	if err := runChroot(root, "/bin/bash", "-c", customize); err != nil {
		return fmt.Errorf("chroot customize: %w", err)
	}

	installAgentsHostSide(root, imagesDir, cfg)

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

func guestCustomizeScript(cfg *config.CellConfig) string {
	script := `
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
`
	if cfg.InstallSuperpowers {
		script += `test -d /opt/opencode-plugins/node_modules/superpowers
`
	}
	script += `test -x /opt/guest-init/guest-entry.sh
if ldd /usr/bin/rsync 2>/dev/null | grep -q 'not found'; then
  echo "rsync has unresolved shared libraries:" >&2
  ldd /usr/bin/rsync >&2 || true
  exit 1
fi
`
	return script
}
