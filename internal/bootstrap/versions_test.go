package bootstrap

import "testing"

func TestResolveArtifacts(t *testing.T) {
	kernel, fc, squash, err := resolveArtifacts("x86_64")
	if err != nil {
		t.Fatalf("resolveArtifacts: %v", err)
	}
	t.Logf("prefix resolution: kernel=%s (%s)", kernel.Version, kernel.URL)
	t.Logf("firecracker=%s (%s)", fc.Version, fc.URL)
	t.Logf("squashfs=%s (%s)", squash.Version, squash.URL)
	if kernel.Version == "" || fc.Version == "" || squash.Version == "" {
		t.Fatal("empty version in resolved artifacts")
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
