package privilege

import "golang.org/x/sys/unix"

// RaiseAmbientNetCaps copies cap_net_admin and cap_net_raw into the ambient
// set so children (ip, iptables, firecracker) keep them across exec.
// cap_dac_override stays on the cell process only (file +ep, not inheritable).
// No-op when the process lacks those capabilities (tests, uninstalled binary).
func RaiseAmbientNetCaps() {
	for _, cap := range []uintptr{unix.CAP_NET_ADMIN, unix.CAP_NET_RAW} {
		_ = unix.Prctl(unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_RAISE, cap, 0, 0)
	}
}
