package models

import (
	"encoding/json"
	"testing"
)

func TestNormalizeAgentKind(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", AgentKindOpenCode},
		{"opencode", AgentKindOpenCode},
		{"claude", AgentKindClaude},
		{"none", AgentKindNone},
		{"gpt", ""},
	}
	for _, c := range cases {
		if got := NormalizeAgentKind(c.in); got != c.want {
			t.Fatalf("NormalizeAgentKind(%q) = %q want %q", c.in, got, c.want)
		}
	}
}

func TestSessionRecordAgentRoundTrip(t *testing.T) {
	cases := []struct {
		agent string
		want  string
	}{
		{"", AgentKindOpenCode},
		{AgentKindOpenCode, AgentKindOpenCode},
		{AgentKindClaude, AgentKindClaude},
		{AgentKindNone, AgentKindNone},
	}
	for _, c := range cases {
		s := &SessionRecord{SessionID: "a1b2c3", Agent: c.agent}
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		var rt SessionRecord
		if err := json.Unmarshal(data, &rt); err != nil {
			t.Fatal(err)
		}
		if got := rt.EffectiveAgent(); got != c.want {
			t.Fatalf("round trip agent=%q EffectiveAgent=%q want %q", c.agent, got, c.want)
		}
	}
}

func TestSessionRecordLegacyJSONWithoutAgentField(t *testing.T) {
	doc := []byte(`{"session_id":"legacy","repo_source":".","state":"stopped"}`)
	var s SessionRecord
	if err := json.Unmarshal(doc, &s); err != nil {
		t.Fatal(err)
	}
	if got := s.EffectiveAgent(); got != AgentKindOpenCode {
		t.Fatalf("legacy session EffectiveAgent=%q want opencode", got)
	}
}
