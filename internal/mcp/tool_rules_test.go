package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// assertCaveat asserts that the MCP rules.explain payload for ruleID
// includes a caveat bullet matching substr. Used by tests covering the
// KnownLimitations -> caveats wire-up.
func assertCaveat(t *testing.T, ruleID, substr string) {
	t.Helper()
	s := &Server{}
	res := s.rulesExplain(rulesArgs{Rule: ruleID})
	if res.IsError {
		t.Fatalf("rulesExplain(%s) returned error: %s", ruleID, res.Content[0].Text)
	}
	if len(res.Content) == 0 {
		t.Fatalf("rulesExplain(%s) returned no content", ruleID)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(res.Content[0].Text), &payload); err != nil {
		t.Fatalf("unmarshal explain payload: %v", err)
	}
	raw, ok := payload["caveats"]
	if !ok {
		t.Fatalf("rule %s explain payload missing 'caveats' key: %v", ruleID, payload)
	}
	bullets, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("rule %s 'caveats' is not a list: %T", ruleID, raw)
	}
	for _, b := range bullets {
		if s, ok := b.(string); ok && strings.Contains(s, substr) {
			return
		}
	}
	t.Fatalf("rule %s caveats %v missing bullet containing %q", ruleID, bullets, substr)
}

// TestRulesExplain_SurfacesCaveats verifies that MCP rules.explain
// renders KnownLimitations under a top-level "caveats" key and that the
// noisiness tier is included.
func TestRulesExplain_SurfacesCaveats(t *testing.T) {
	assertCaveat(t, "MagicNumber", "Heuristic literal scan")
}

// TestRulesExplain_NoisinessAlwaysPresent verifies the explain payload
// always carries a 'noisiness' tier (never empty / unset).
func TestRulesExplain_NoisinessAlwaysPresent(t *testing.T) {
	s := &Server{}
	res := s.rulesExplain(rulesArgs{Rule: "MagicNumber"})
	if res.IsError {
		t.Fatalf("rulesExplain returned error: %s", res.Content[0].Text)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(res.Content[0].Text), &payload); err != nil {
		t.Fatalf("unmarshal explain payload: %v", err)
	}
	n, ok := payload["noisiness"].(string)
	if !ok {
		t.Fatalf("explain payload missing string 'noisiness': %v", payload)
	}
	if n == "" || n == "unset" {
		t.Fatalf("noisiness tier should be populated, got %q", n)
	}
}
