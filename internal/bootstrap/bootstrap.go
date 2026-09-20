package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	}
	kernelArt, fcArt, err := resolveArtifacts(arch, pins)
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
		_ = os.Remove(rootfsBuildStampPath(cfg.RootfsPath))
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

	needsRootfs, err := needsRootfsRebuild(cfg.RootfsPath, rootfsBuildStamp(cfg), rebuildRootfs || cfg.RebuildRootfs)
	if err != nil {
		return err
	}
	if needsRootfs {
		fmt.Println("Building rootfs via debootstrap noble…")
		if err := buildRootfs(cfg, rootfsSizeMB(cfg)); err != nil {
			return err
		}
		if err := writeRootfsBuildStamp(cfg.RootfsPath, rootfsBuildStamp(cfg)); err != nil {
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
	defer func() { _ = os.RemoveAll(extractDir) }()

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
