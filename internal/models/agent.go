package models

const (
	AgentKindOpenCode = "opencode"
	AgentKindClaude   = "claude"
	AgentKindNone     = "none"
)

// NormalizeAgentKind maps an agent kind string onto a supported kind.
// Empty (legacy sessions, unset config) defaults to opencode; unknown values
// return "" so callers can reject them.
func NormalizeAgentKind(kind string) string {
	switch kind {
	case "":
		return AgentKindOpenCode
	case AgentKindOpenCode, AgentKindClaude, AgentKindNone:
		return kind
	default:
		return ""
	}
}

// EffectiveAgent returns the session's agent kind with legacy sessions
// (empty Agent field) treated as opencode.
func (s *SessionRecord) EffectiveAgent() string {
	return NormalizeAgentKind(s.Agent)
}
