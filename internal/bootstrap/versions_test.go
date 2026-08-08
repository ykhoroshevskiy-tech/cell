package bootstrap

import "testing"

func TestResolveArtifactsFromPins(t *testing.T) {
	pins := ArtifactPins{
		CIPrefix:           "firecracker-ci/20260708-f11c230ed107-0/",
		KernelVersion:      "6.1.176",
		FirecrackerVersion: "v1.16.1",
		SquashfsVersion:    "24.04",
	}
	kernel, fc, squash, err := resolveArtifacts("x86_64", pins)
	if err != nil {
		t.Fatalf("resolveArtifacts: %v", err)
	}
	wantKernel := "https://s3.amazonaws.com/spec.ccfc.min/firecracker-ci/20260708-f11c230ed107-0/x86_64/vmlinux-6.1.176"
	wantFC := "https://github.com/firecracker-microvm/firecracker/releases/download/v1.16.1/firecracker-v1.16.1-x86_64.tgz"
	wantSQ := "https://s3.amazonaws.com/spec.ccfc.min/firecracker-ci/20260708-f11c230ed107-0/x86_64/ubuntu-24.04.squashfs"
	if kernel.URL != wantKernel || kernel.Version != "6.1.176" {
		t.Fatalf("kernel = %+v", kernel)
	}
	if fc.URL != wantFC || fc.Version != "v1.16.1" {
		t.Fatalf("firecracker = %+v", fc)
	}
	if squash.URL != wantSQ || squash.Version != "24.04" {
		t.Fatalf("squashfs = %+v", squash)
	}
}

func TestResolveArtifactsRejectsEmptyPin(t *testing.T) {
	_, _, _, err := resolveArtifacts("x86_64", ArtifactPins{
		CIPrefix: "firecracker-ci/x/", KernelVersion: "6.1.176",
		FirecrackerVersion: "v1.16.1", // SquashfsVersion empty
	})
	if err == nil {
		t.Fatal("expected error for empty squashfs pin")
	}
}

func TestSelectLatestCIPrefix(t *testing.T) {
	prefixes := []string{
		"firecracker-ci/v1.9/",
		"firecracker-ci/v1.15/",
		"firecracker-ci/v1.15-vmclock/",
		"firecracker-ci/20260624-ce269725504a-0/",
		"firecracker-ci/20260708-f11c230ed107-0/",
	}
	got, err := selectLatestCIPrefix(prefixes)
	if err != nil {
		t.Fatal(err)
	}
	want := "firecracker-ci/20260708-f11c230ed107-0/"
	if got != want {
		t.Fatalf("selectLatestCIPrefix=%q want %q", got, want)
	}
}

func TestSelectLatestStablePrefixFallback(t *testing.T) {
	prefixes := []string{
		"firecracker-ci/v1.9/",
		"firecracker-ci/v1.15/",
		"firecracker-ci/v1.15-vmclock/",
		"firecracker-ci/20260624-ce269725504a-0/",
	}
	got, err := selectLatestStablePrefix(prefixes)
	if err != nil {
		t.Fatal(err)
	}
	if got != "firecracker-ci/v1.15/" {
		t.Fatalf("selectLatestStablePrefix=%q want firecracker-ci/v1.15/", got)
	}
}
