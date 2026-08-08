package bootstrap

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
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
	kernelArt, fcArt, sqArt, err := resolveArtifacts(arch, pins)
	if err != nil {
		return err
	}

	kernelVersioned := filepath.Join(cfg.ImagesDir, "vmlinux-"+kernelArt.Version)
	fcVersioned := filepath.Join(cfg.ImagesDir, "bin", "firecracker-"+fcArt.Version)
	sqVersioned := filepath.Join(cfg.ImagesDir, "ubuntu-"+sqArt.Version+".squashfs")
	fcTgz := filepath.Join(cfg.ImagesDir, "firecracker-"+fcArt.Version+".tgz")

	if force {
		_ = os.Remove(kernelVersioned)
		_ = os.Remove(fcVersioned)
		_ = os.Remove(sqVersioned)
		_ = os.Remove(fcTgz)
		_ = os.Remove(cfg.RootfsPath)
	}

	if !force && artifactReady(cfg.KernelPath) {
		st, _ := os.Stat(cfg.KernelPath)
		fmt.Printf("✓ kernel cached (%s)\n", humanSize(st.Size()))
	} else {
		if err := download(kernelVersioned, kernelArt); err != nil {
			return fmt.Errorf("download %s failed (ci_prefix=%s kernel=%s firecracker=%s squashfs=%s): %w",
				kernelArt.Name, pins.CIPrefix, pins.KernelVersion, pins.FirecrackerVersion, pins.SquashfsVersion, err)
		}
		if err := ValidateKernel(kernelVersioned); err != nil {
			return err
		}
		_ = ensureSymlink(cfg.KernelPath, kernelVersioned)
	}

	if !force && artifactReady(cfg.SquashfsPath) {
		st, _ := os.Stat(cfg.SquashfsPath)
		fmt.Printf("✓ squashfs cached (%s)\n", humanSize(st.Size()))
	} else {
		if err := download(sqVersioned, sqArt); err != nil {
			return fmt.Errorf("download %s failed (ci_prefix=%s kernel=%s firecracker=%s squashfs=%s): %w",
				sqArt.Name, pins.CIPrefix, pins.KernelVersion, pins.FirecrackerVersion, pins.SquashfsVersion, err)
		}
		cfg.SquashfsPath = sqVersioned
	}

	if !force && artifactReady(cfg.FirecrackerBin) {
		st, _ := os.Stat(cfg.FirecrackerBin)
		fmt.Printf("✓ firecracker cached (%s)\n", humanSize(st.Size()))
	} else {
		if err := download(fcTgz, fcArt); err != nil {
			return fmt.Errorf("download %s failed (ci_prefix=%s kernel=%s firecracker=%s squashfs=%s): %w",
				fcArt.Name, pins.CIPrefix, pins.KernelVersion, pins.FirecrackerVersion, pins.SquashfsVersion, err)
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

	needsRootfs := rebuildRootfs || cfg.RebuildRootfs
	if st, err := os.Stat(cfg.RootfsPath); err != nil || st.Size() == 0 {
		needsRootfs = true
	}
	if needsRootfs {
		fmt.Printf("Building rootfs from %s…\n", cfg.SquashfsPath)
		if err := buildRootfs(cfg.SquashfsPath, cfg.RootfsPath, initScriptsDir, cfg.ImagesDir, rootfsSizeMB(cfg)); err != nil {
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
	n := cfg.ProjectDiskSizeMB * 4
	if n < 4096 {
		n = 4096
	}
	return n
}

func buildRootfs(squashfsPath, rootfsPath, initScriptsDir, imagesDir string, sizeMB int) error {
	workDir, err := os.MkdirTemp("", "cell-rootfs-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	root := filepath.Join(workDir, "root")
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	if err := runCmd("unsquashfs", "-d", root, squashfsPath); err != nil {
		return fmt.Errorf("unsquashfs: %w", err)
	}

	authKey := filepath.Join(initScriptsDir, "authorized_keys")
	data, err := os.ReadFile(authKey)
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		return fmt.Errorf("authorized_keys missing or empty at %s", authKey)
	}

	// DNS kept for any future in-chroot network need; current apt-free path
	// fetches debs and OpenCode on the host.
	if resolv, err := os.ReadFile("/etc/resolv.conf"); err == nil {
		_ = os.MkdirAll(filepath.Join(root, "etc"), 0755)
		_ = os.WriteFile(filepath.Join(root, "etc", "resolv.conf"), resolv, 0644)
	}

	for _, d := range []string{"project", "run/sshd", "tmp", "opt/guest-init"} {
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
	for _, name := range []string{"guest-entry.sh", "tmux-attach-opencode.sh"} {
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

	// Firecracker CI squashfs has no dpkg status — install missing pkgs via
	// jammy .deb extract (host may be a different Ubuntu release).
	// jammy zsh.deb installs to /bin/zsh (not /usr/bin); rsync to /usr/bin/rsync
	// rsync needs libpopt.so.0 (libpopt0) or guest pull fails at runtime
	if err := ensureJammyDebs(root, []debPkg{
		{Name: "zsh-common", Binary: ""}, // supporting files for zsh
		{Name: "zsh", Binary: "/bin/zsh"},
		{Name: "libpopt0", Binary: "/usr/lib/x86_64-linux-gnu/libpopt.so.0"},
		{Name: "rsync", Binary: "/usr/bin/rsync"},
	}); err != nil {
		return err
	}

	customize := `
set -eux
id agent >/dev/null 2>&1 || useradd -m -s /bin/zsh agent
mkdir -p /run/sshd
command -v tmux
command -v rsync
command -v sshd
command -v zsh
test -x /opt/guest-init/guest-entry.sh
if ldd /usr/bin/rsync 2>/dev/null | grep -q 'not found'; then
  echo "rsync has unresolved shared libraries:" >&2
  ldd /usr/bin/rsync >&2 || true
  exit 1
fi
`
	fmt.Println("chroot: customize (user, verify binaries)…")
	if err := runChroot(root, "/bin/bash", "-c", customize); err != nil {
		return fmt.Errorf("chroot customize: %w", err)
	}

	fmt.Println("installing opencode (host download)…")
	if err := installOpenCodeHostSide(root, imagesDir); err != nil {
		fmt.Printf("  opencode download failed: %v; installing stub\n", err)
		writeOpenCodeStub(root)
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
	// mkfs.ext4 -d (matches Python launcher; proven fast). By default filter the
	// harmless "__populate_fs: symlink increased in size" stderr flood; with -v
	// stream raw mkfs output so the operator sees live progress.
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

func installOpenCodeHostSide(root, imagesDir string) error {
	target := ""
	switch runtime.GOARCH {
	case "amd64":
		target = "linux-x64-baseline"
	case "arm64":
		target = "linux-arm64-baseline"
	}
	if target == "" {
		return fmt.Errorf("unsupported arch %s", runtime.GOARCH)
	}
	url := "https://github.com/anomalyco/opencode/releases/latest/download/opencode-" + target + ".tar.gz"
	cache := filepath.Join(imagesDir, "opencode-"+target+".tar.gz")
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

	tmp, err := os.MkdirTemp("", "cell-opencode-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := runCmd("tar", "-xzf", cache, "-C", tmp); err != nil {
		_ = os.Remove(cache) // corrupt cache → refetch next time
		return fmt.Errorf("extract opencode: %w", err)
	}
	src := filepath.Join(tmp, "opencode")
	if st, err := os.Stat(src); err != nil || st.IsDir() {
		return fmt.Errorf("opencode binary missing in tarball")
	}
	binDir := filepath.Join(root, "opt", "opencode", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "usr", "local", "bin"), 0755); err != nil {
		return err
	}
	if err := runCmd("install", "-m", "755", src, filepath.Join(binDir, "opencode")); err != nil {
		return fmt.Errorf("install opencode binary: %w", err)
	}
	link := filepath.Join(root, "usr", "local", "bin", "opencode")
	_ = os.Remove(link)
	if err := os.Symlink("/opt/opencode/bin/opencode", link); err != nil {
		return err
	}
	fmt.Println("  opencode installed")
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

func writeOpenCodeStub(root string) {
	stub := filepath.Join(root, "usr", "local", "bin", "opencode")
	_ = os.MkdirAll(filepath.Dir(stub), 0755)
	_ = os.WriteFile(stub, []byte("#!/bin/sh\necho \"[opencode stub] install failed during bootstrap; run: cell bootstrap --force\" >&2\nexec /bin/zsh\n"), 0755)
}

// debPkg is a jammy package to extract into the guest rootfs without apt/dpkg DB.
type debPkg struct {
	Name   string
	Binary string // guest path to check; empty = always extract (dependency)
}

// ensureJammyDebs downloads Ubuntu jammy .debs and extracts them with dpkg-deb -x.
// Host apt-get download is NOT used — host may be a newer release (ABI mismatch).
func ensureJammyDebs(root string, pkgs []debPkg) error {
	need := false
	for _, p := range pkgs {
		if p.Binary == "" {
			// dependency-only package: extract when any binary package is missing
			continue
		}
		if !guestPathExists(root, p.Binary) {
			need = true
			break
		}
	}
	if !need {
		// still extract empty-Binary deps if a later package will be extracted
		return nil
	}

	tmp, err := os.MkdirTemp("", "cell-deb-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	index, err := fetchJammyPackageIndex()
	if err != nil {
		return err
	}

	for _, p := range pkgs {
		if p.Binary != "" && guestPathExists(root, p.Binary) {
			continue
		}
		// empty Binary (e.g. zsh-common): extract when we're in the need path
		file, ok := index[p.Name]
		if !ok {
			return fmt.Errorf("package %s not found in jammy Packages index", p.Name)
		}
		url := "http://archive.ubuntu.com/ubuntu/" + file
		debPath := filepath.Join(tmp, filepath.Base(file))
		fmt.Printf("installing %s via jammy deb extract…\n", p.Name)
		if err := downloadFile(debPath, url); err != nil {
			return fmt.Errorf("download %s: %w", p.Name, err)
		}
		if err := runCmd("dpkg-deb", "-x", debPath, root); err != nil {
			return fmt.Errorf("dpkg-deb -x %s: %w", p.Name, err)
		}
		// dpkg-deb -x replaces /bin→usr/bin symlink with a real dir; restore usrmerge
		if err := repairUsrmerge(root); err != nil {
			return fmt.Errorf("usrmerge repair after %s: %w", p.Name, err)
		}
		if p.Binary != "" && !guestPathExists(root, p.Binary) {
			return fmt.Errorf("%s still missing at %s after deb extract", p.Name, p.Binary)
		}
	}
	return nil
}

func guestPathExists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, rel))
	return err == nil
}

// repairUsrmerge restores /bin,/sbin,/lib,/lib64 as symlinks into usr/
// after dpkg-deb -x materializes them as real directories (breaks chroot /bin/bash).
func repairUsrmerge(root string) error {
	for _, d := range []string{"bin", "sbin", "lib", "lib64"} {
		p := filepath.Join(root, d)
		fi, err := os.Lstat(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if !fi.IsDir() {
			continue
		}
		usr := filepath.Join(root, "usr", d)
		if err := os.MkdirAll(usr, 0755); err != nil {
			return err
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return err
		}
		for _, e := range entries {
			src := filepath.Join(p, e.Name())
			dst := filepath.Join(usr, e.Name())
			if _, err := os.Lstat(dst); err == nil {
				_ = os.RemoveAll(dst)
			}
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("merge %s → %s: %w", src, dst, err)
			}
		}
		if err := os.RemoveAll(p); err != nil {
			return err
		}
		if err := os.Symlink("usr/"+d, p); err != nil {
			return err
		}
	}
	return nil
}

func fetchJammyPackageIndex() (map[string]string, error) {
	url := "http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/Packages.gz"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d fetching jammy Packages.gz", resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	data, err := io.ReadAll(gz)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	var pkg, file string
	flush := func() {
		if pkg != "" && file != "" {
			if _, exists := out[pkg]; !exists {
				out[pkg] = file
			}
		}
		pkg, file = "", ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Package: ") {
			flush()
			pkg = strings.TrimPrefix(line, "Package: ")
		} else if strings.HasPrefix(line, "Filename: ") {
			file = strings.TrimPrefix(line, "Filename: ")
		} else if line == "" {
			flush()
		}
	}
	flush()
	return out, nil
}

func downloadFile(dst, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
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
