package privilege

import (
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// capVersion3 selects 64-bit capability sets (two 32-bit words per set).
const capVersion3 = 0x20080522

// capHeader and capData mirror __user_cap_header_struct and
// __user_cap_data_struct used by the capget/capset syscalls.
type capHeader struct {
	version uint32
	pid     int32
}

type capData struct {
	effective   uint32
	permitted   uint32
	inheritable uint32
}

type capSets [2]capData

func capGet() (capSets, error) {
	hdr := capHeader{version: capVersion3}
	var sets capSets
	_, _, errno := unix.Syscall(unix.SYS_CAPGET, uintptr(unsafe.Pointer(&hdr)), uintptr(unsafe.Pointer(&sets[0])), 0)
	if errno != 0 {
		return sets, errno
	}
	return sets, nil
}

func capSet(sets capSets) error {
	hdr := capHeader{version: capVersion3}
	_, _, errno := unix.Syscall(unix.SYS_CAPSET, uintptr(unsafe.Pointer(&hdr)), uintptr(unsafe.Pointer(&sets[0])), 0)
	if errno != 0 {
		return errno
	}
	return nil
}

// RaiseAmbientNetCaps copies cap_net_admin and cap_net_raw into the ambient
// set so children (ip, iptables, bridge, firecracker) keep them across exec.
// PR_CAP_AMBIENT_RAISE requires the capability to be present in the permitted
// AND the inheritable set; exec preserves the (usually empty) inheritable set,
// so the net capabilities are first copied from permitted into inheritable via
// capset. cap_dac_override stays on the cell process only (file +ep, not
// inheritable). No-op when the process lacks the capabilities (tests,
// uninstalled binary) or the kernel does not support ambient capabilities.
func RaiseAmbientNetCaps() {
	sets, err := capGet()
	if err != nil {
		return
	}
	const netCaps = uint32(1<<unix.CAP_NET_ADMIN | 1<<unix.CAP_NET_RAW)
	changed := false
	for i := range sets {
		want := sets[i].permitted & netCaps
		if sets[i].inheritable&want != want {
			sets[i].inheritable |= want
			changed = true
		}
	}
	if changed {
		if err := capSet(sets); err != nil {
			return
		}
	}
	for _, capValue := range []uintptr{unix.CAP_NET_ADMIN, unix.CAP_NET_RAW} {
		_ = unix.Prctl(unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_RAISE, capValue, 0, 0)
	}
}

var ambientAttrOnce sync.Once
var ambientAttr *syscall.SysProcAttr

// AmbientSysProcAttr returns a SysProcAttr that raises cap_net_admin and
// cap_net_raw in the fork child before execve, or nil when the process lacks
// those capabilities. This must be set per exec.Command: the parent's ambient
// set is thread-local, and goroutines migrate between OS threads, so children
// would otherwise inherit ambient capabilities unreliably. Returns nil when
// the capabilities are absent (tests, uninstalled binary) so child execution
// is not disturbed.
func AmbientSysProcAttr() *syscall.SysProcAttr {
	ambientAttrOnce.Do(func() {
		sets, err := capGet()
		if err != nil {
			return
		}
		const netCaps = uint32(1<<unix.CAP_NET_ADMIN | 1<<unix.CAP_NET_RAW)
		if sets[0].permitted&netCaps == netCaps {
			ambientAttr = &syscall.SysProcAttr{
				AmbientCaps: []uintptr{unix.CAP_NET_ADMIN, unix.CAP_NET_RAW},
			}
		}
	})
	return ambientAttr
}
