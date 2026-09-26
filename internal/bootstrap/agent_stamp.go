package bootstrap

import (
	"fmt"
	"os"
	"strings"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

// AgentStampAll marks a rootfs built with every supported agent installed;
// it satisfies any per-session --agent choice except validation itself.
const AgentStampAll = "all"

// ParseRootfsAgentKind extracts the agent kind from a rootfs build stamp such
// as "debootstrap:noble+...+agent:claude". ok is false when the stamp carries
// no usable +agent: token (legacy rootfs).
func ParseRootfsAgentKind(stamp string) (string, bool) {
	for _, token := range strings.Split(stamp, "+") {
		kind, found := strings.CutPrefix(token, "agent:")
		if !found {
			continue
		}
		kind = strings.TrimSpace(kind)
		if kind == "" {
			return "", false
		}
		if kind == AgentStampAll {
			return AgentStampAll, true
		}
		kind = models.NormalizeAgentKind(kind)
		if kind == "" {
			return "", false
		}
		return kind, true
	}
	return "", false
}

// ReadRootfsAgentKind reads <rootfsPath>.build-stamp and returns the agent
// kind the rootfs was built for.
func ReadRootfsAgentKind(rootfsPath string) (string, bool) {
	data, err := os.ReadFile(rootfsBuildStampPath(rootfsPath))
	if err != nil {
		return "", false
	}
	return ParseRootfsAgentKind(string(data))
}

// CheckRootfsAgentKind verifies that the rootfs at rootfsPath supports the
// requested agent kind. A rootfs stamped "all" carries every agent; a missing
// or agent-less stamp is treated as a legacy opencode rootfs; "none" needs no
// agent binary and always passes.
func CheckRootfsAgentKind(rootfsPath, want string) error {
	want = models.NormalizeAgentKind(want)
	if want == "" {
		return fmt.Errorf("invalid agent kind")
	}
	if want == models.AgentKindNone {
		return nil
	}
	built, ok := ReadRootfsAgentKind(rootfsPath)
	if ok && (built == want || built == AgentStampAll) {
		return nil
	}
	if !ok && want == models.AgentKindOpenCode {
		return nil
	}
	builtLabel := built
	if !ok {
		builtLabel = "unknown"
	}
	return fmt.Errorf("rootfs built for agent %q; run: sudo cell bootstrap --rebuild-rootfs", builtLabel)
}
