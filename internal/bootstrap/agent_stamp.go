package bootstrap

import (
	"fmt"
	"os"
	"strings"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

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

// CheckRootfsAgentKind verifies that the rootfs at rootfsPath was built for
// the requested agent kind. A missing or agent-less stamp is treated as a
// legacy opencode rootfs; "none" needs no agent binary and always passes.
func CheckRootfsAgentKind(rootfsPath, want string) error {
	want = models.NormalizeAgentKind(want)
	if want == "" {
		return fmt.Errorf("invalid agent kind")
	}
	if want == models.AgentKindNone {
		return nil
	}
	built, ok := ReadRootfsAgentKind(rootfsPath)
	if ok && built == want {
		return nil
	}
	if !ok && want == models.AgentKindOpenCode {
		return nil
	}
	builtLabel := built
	if !ok {
		builtLabel = "unknown"
	}
	return fmt.Errorf("rootfs built for agent %q; run: CELL_AGENT=%s sudo cell bootstrap", builtLabel, want)
}
