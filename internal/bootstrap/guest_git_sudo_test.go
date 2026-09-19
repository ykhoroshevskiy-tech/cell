package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
)

func TestGuestAptPackagesIncludesGitAndSudo(t *testing.T) {
	pkgs := guestAptPackages()
	want := []string{"git", "sudo", "openssh-server", "ca-certificates"}
	for _, name := range want {
		found := false
		for _, p := range pkgs {
			if p == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("guestAptPackages() missing %q", name)
		}
	}
}

func TestWriteAgentSudoers(t *testing.T) {
	root := t.TempDir()
	if err := writeAgentSudoers(root); err != nil {
		t.Fatalf("writeAgentSudoers() error = %v", err)
	}
	path := filepath.Join(root, "etc", "sudoers.d", "agent")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sudoers file: %v", err)
	}
	want := "agent ALL=(ALL) NOPASSWD:ALL\nDefaults:agent !requiretty\n"
	if got := string(data); got != want {
		t.Fatalf("sudoers content:\n got %q\nwant %q", got, want)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat sudoers file: %v", err)
	}
	if fi.Mode().Perm() != 0440 {
		t.Fatalf("sudoers mode = %o, want 0440", fi.Mode().Perm())
	}
}

func TestGuestCustomizeScriptChecksGitSudoNode(t *testing.T) {
	script := guestCustomizeScript(&config.CellConfig{InstallSuperpowers: true})
	for _, want := range []string{
		"command -v git",
		"command -v sudo",
		"command -v node",
		"command -v npm",
		"command -v uv",
		"command -v python3",
		"su - agent -c 'sudo -n true'",
		"/opt/opencode-plugins/node_modules/superpowers",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("guestCustomizeScript() missing %q", want)
		}
	}
}

func TestGuestCustomizeScriptOmitsSuperpowersWhenDisabled(t *testing.T) {
	script := guestCustomizeScript(&config.CellConfig{InstallSuperpowers: false})
	if strings.Contains(script, "superpowers") {
		t.Fatalf("disabled install must not verify superpowers: %q", script)
	}
	for _, want := range []string{
		"command -v git",
		"test -x /opt/guest-init/guest-entry.sh",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("guestCustomizeScript() missing %q", want)
		}
	}
}

func TestNodeTarballURL(t *testing.T) {
	got := nodeTarballURL("v24.20.0", "linux-x64")
	want := "https://nodejs.org/dist/v24.20.0/node-v24.20.0-linux-x64.tar.xz"
	if got != want {
		t.Fatalf("nodeTarballURL = %q, want %q", got, want)
	}
}

func TestRootfsSizeMBIndependentOfProjectDisk(t *testing.T) {
	cfg := &config.CellConfig{ProjectDiskSizeMB: 3072, RootfsSizeMB: 4096}
	if got := rootfsSizeMB(cfg); got != 4096 {
		t.Fatalf("rootfsSizeMB() = %d, want 4096", got)
	}
}

func TestSquashfsBuildStampIncludesNodeVersion(t *testing.T) {
	cfg := &config.CellConfig{NodeVersion: "v24.20.0"}
	want := "debootstrap:noble+apt+node:v24.20.0+uv:0.12.7+py:3.13+sp:off+agent:opencode"
	if got := squashfsBuildStamp(cfg); got != want {
		t.Fatalf("squashfsBuildStamp() = %q, want %q", got, want)
	}
}

func TestSquashfsBuildStampSuperpowersToggle(t *testing.T) {
	off := squashfsBuildStamp(&config.CellConfig{InstallSuperpowers: false})
	on := squashfsBuildStamp(&config.CellConfig{InstallSuperpowers: true})
	if on == off {
		t.Fatal("stamp must differ between superpowers on/off (forces rebuild)")
	}
	if !strings.Contains(on, "+sp:on") || !strings.Contains(off, "+sp:off") {
		t.Fatalf("stamps: on=%q off=%q", on, off)
	}
}

func TestUvTarballURL(t *testing.T) {
	got := uvTarballURL("0.12.7", "x86_64")
	want := "https://github.com/astral-sh/uv/releases/download/0.12.7/uv-x86_64-unknown-linux-gnu.tar.gz"
	if got != want {
		t.Fatalf("uvTarballURL = %q, want %q", got, want)
	}
}

func TestGuestPythonInstallScript(t *testing.T) {
	script := guestPythonInstallScript("3.13")
	for _, want := range []string{
		"UV_PYTHON_INSTALL_DIR=/usr/local/share/uv/python",
		"UV_PYTHON_BIN_DIR=/usr/local/bin",
		"uv python install --default 3.13",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("guestPythonInstallScript() missing %q", want)
		}
	}
}

func TestGuestSuperpowersInstallUsesGitSpec(t *testing.T) {
	script := guestSuperpowersInstallScript()
	if !strings.Contains(script, superpowersNPMSpec) {
		t.Fatalf("guestSuperpowersInstallScript() missing %q", superpowersNPMSpec)
	}
	if !strings.Contains(script, "/opt/opencode-plugins") {
		t.Fatal("guestSuperpowersInstallScript() missing install prefix")
	}
}

func TestWriteNobleSourcesList(t *testing.T) {
	root := t.TempDir()
	if err := writeNobleSourcesList(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "etc", "apt", "sources.list"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "noble main universe") || !strings.Contains(got, "noble-security") {
		t.Fatalf("sources.list = %q", got)
	}
}
