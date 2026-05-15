package serve

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestAnalyzeProject_ShowPerfEmitsPerfTiming verifies that when the
// client requests ShowPerf=true, the daemon constructs a real
// perf.Tracker and the JSON envelope OutputPhase writes contains a
// non-empty "perfTiming" key. The pre-fix code path always ran the
// noop tracker, so "perfTiming" was silently absent.
func TestAnalyzeProject_ShowPerfEmitsPerfTiming(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "A.kt",
		"package demo\n\nclass A {\n    fun a() {}\n}\n")

	// First call warms the bundle so the second can take the warm
	// path; both should still emit perfTiming because the tracker is
	// gated on ShowPerf, not on bundle-hit.
	var warmup daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{ShowPerf: true}, &warmup); err != nil {
		t.Fatalf("warmup call: %v", err)
	}
	requirePerfTiming(t, "warmup", warmup.Findings)

	var second daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{ShowPerf: true}, &second); err != nil {
		t.Fatalf("second call: %v", err)
	}
	requirePerfTiming(t, "bundle-hit", second.Findings)
}

// TestAnalyzeProject_ShowPerfFalseOmitsPerfTiming is the negative
// case: with ShowPerf=false the tracker is a noop, perfTiming must
// not appear (otherwise non-perf callers pay a cost they didn't ask
// for and JSON-schema-driven consumers see surprise fields).
func TestAnalyzeProject_ShowPerfFalseOmitsPerfTiming(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "A.kt",
		"package demo\n\nclass A {\n    fun a() {}\n}\n")

	var res daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &res); err != nil {
		t.Fatalf("call: %v", err)
	}
	if strings.Contains(string(res.Findings), `"perfTiming"`) {
		t.Errorf("ShowPerf=false: expected no perfTiming in JSON, got body containing it")
	}
}

func requirePerfTiming(t *testing.T, label string, body []byte) {
	t.Helper()
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("%s: decode envelope: %v", label, err)
	}
	raw, ok := envelope["perfTiming"]
	if !ok {
		t.Fatalf("%s: JSON envelope missing perfTiming key; got %v", label, mapKeys(envelope))
	}
	var entries []map[string]any
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("%s: decode perfTiming: %v", label, err)
	}
	if len(entries) == 0 {
		t.Errorf("%s: perfTiming is an empty array — tracker was wired but never populated", label)
	}
}

func mapKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
