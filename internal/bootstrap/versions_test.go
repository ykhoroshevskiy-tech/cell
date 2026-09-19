package bootstrap

import "testing"

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
}

func TestResolveArtifactsRejectsEmptyPin(t *testing.T) {
	_, _, err := resolveArtifacts("x86_64", ArtifactPins{
		CIPrefix: "firecracker-ci/x/", KernelVersion: "6.1.176",
	})
	if err == nil {
		t.Fatal("expected error for empty pin")
	}
}
