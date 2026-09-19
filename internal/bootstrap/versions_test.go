package bootstrap

import (
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
)

func TestResolveArtifactsFromPins(t *testing.T) {
	pins := ArtifactPins{
		CIPrefix:           "firecracker-ci/20260708-f11c230ed107-0/",
		KernelVersion:      "6.1.176",
		FirecrackerVersion: "v1.16.1",
	}
	kernel, fc, err := resolveArtifacts("x86_64", pins)
	if err != nil {
		t.Fatalf("resolveArtifacts: %v", err)
	}
	wantKernel := "https://s3.amazonaws.com/spec.ccfc.min/firecracker-ci/20260708-f11c230ed107-0/x86_64/vmlinux-6.1.176"
	wantFC := "https://github.com/firecracker-microvm/firecracker/releases/download/v1.16.1/firecracker-v1.16.1-x86_64.tgz"
	if kernel.URL != wantKernel || kernel.Version != "6.1.176" {
		t.Fatalf("kernel = %+v", kernel)
	}
	if fc.URL != wantFC || fc.Version != "v1.16.1" {
		t.Fatalf("firecracker = %+v", fc)
	}
	if kernel.SHA256 != artifactSHA256s["kernel"]["x86_64"] {
		t.Fatalf("kernel.SHA256 = %q want pinned sum", kernel.SHA256)
	}
	if fc.SHA256 != artifactSHA256s["firecracker"]["x86_64"] {
		t.Fatalf("firecracker.SHA256 = %q want pinned sum", fc.SHA256)
	}
}

func TestResolveArtifactsCustomPinsResolveWithoutKnownSHA256(t *testing.T) {
	pins := ArtifactPins{
		CIPrefix:           "firecracker-ci/other/",
		KernelVersion:      "6.1.100",
		FirecrackerVersion: "v1.16.0",
	}
	kernel, fc, err := resolveArtifacts("aarch64", pins)
	if err != nil {
		t.Fatalf("resolveArtifacts: %v", err)
	}
	if kernel.SHA256 != "" || fc.SHA256 != "" {
		t.Fatalf("custom pins must not carry another version's sum: kernel=%q fc=%q", kernel.SHA256, fc.SHA256)
	}
}

func TestArtifactPinsMatchConfigDefaults(t *testing.T) {
	def := config.Default()
	if builtinCIPrefix != def.CIPrefix || builtinKernelVersion != def.KernelVersion ||
		builtinFirecrackerVersion != def.FirecrackerVersion {
		t.Fatalf("builtin pins %s/%s/%s diverge from config defaults %s/%s/%s",
			builtinCIPrefix, builtinKernelVersion, builtinFirecrackerVersion,
			def.CIPrefix, def.KernelVersion, def.FirecrackerVersion)
	}
}

func TestResolveArtifactsRejectsEmptyPin(t *testing.T) {
	_, _, err := resolveArtifacts("x86_64", ArtifactPins{
		CIPrefix: "firecracker-ci/x/", KernelVersion: "6.1.176",
	})
	if err == nil {
		t.Fatal("expected error for empty pin")
	}
}
