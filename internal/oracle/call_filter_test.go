package oracle

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestFinalizeCallTargetFilter_DerivesAndDedupsCallees(t *testing.T) {
	got := FinalizeCallTargetFilter(CallTargetFilterSummary{
		Enabled:     true,
		CalleeNames: []string{"delay", "await", "delay"},
		TargetFQNs:  []string{"kotlinx.coroutines.delay", "com.example.Foo#bar"},
		LexicalHintsByCallee: map[string][]string{
			"delay": {"kotlinx.coroutines", "kotlinx.coroutines"},
		},
		LexicalSkipByCallee: map[string][]string{
			"w": {"Log", "Log"},
		},
	})

	want := []string{"await", "bar", "delay"}
	if len(got.CalleeNames) != len(want) {
		t.Fatalf("callee names = %v, want %v", got.CalleeNames, want)
	}
	for i := range want {
		if got.CalleeNames[i] != want[i] {
			t.Fatalf("callee names = %v, want %v", got.CalleeNames, want)
		}
	}
	if got.Fingerprint == "" {
		t.Fatal("fingerprint is empty")
	}
	if hints := got.LexicalHintsByCallee["bar"]; len(hints) == 0 {
		t.Fatalf("lexical hints for derived target missing: %+v", got.LexicalHintsByCallee)
	}
	if hints := got.LexicalHintsByCallee["delay"]; len(hints) != 1 || hints[0] != "kotlinx.coroutines" {
		t.Fatalf("lexical hints for delay = %v, want [kotlinx.coroutines]", hints)
	}
	if skips := got.LexicalSkipByCallee["w"]; len(skips) != 1 || skips[0] != "Log" {
		t.Fatalf("lexical skips for w = %v, want [Log]", skips)
	}
}

func TestWriteCallTargetFilterFile(t *testing.T) {
	dir := t.TempDir()
	summary := FinalizeCallTargetFilter(CallTargetFilterSummary{
		Enabled:     true,
		CalleeNames: []string{"delay"},
		TargetFQNs:  []string{"kotlinx.coroutines.delay"},
		LexicalHintsByCallee: map[string][]string{
			"delay": {"kotlinx.coroutines"},
		},
		LexicalSkipByCallee: map[string][]string{
			"w": {"Log"},
		},
		RuleProfiles: []CallTargetRuleProfile{{
			RuleID:               "SuspendRule",
			CalleeNames:          []string{"delay"},
			TargetFQNs:           []string{"kotlinx.coroutines.delay"},
			LexicalHintsByCallee: map[string][]string{"delay": {"kotlinx.coroutines"}},
			LexicalSkipByCallee:  map[string][]string{"w": {"Log"}},
			AnnotatedIdentifiers: []string{"Deprecated"},
			DerivedCalleeNames:   []string{"oldCall"},
		}},
	})

	path, err := WriteCallTargetFilterFile(summary, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var payload callTargetFilterJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Version != 1 || payload.Mode != "calleeNames" {
		t.Fatalf("payload header = %+v", payload)
	}
	if len(payload.CalleeNames) != 1 || payload.CalleeNames[0] != "delay" {
		t.Fatalf("calleeNames = %v, want [delay]", payload.CalleeNames)
	}
	if len(payload.RuleProfiles) != 1 || payload.RuleProfiles[0].RuleID != "SuspendRule" {
		t.Fatalf("ruleProfiles = %+v, want SuspendRule profile", payload.RuleProfiles)
	}
	if hints := payload.LexicalHintsByCallee["delay"]; len(hints) != 1 || hints[0] != "kotlinx.coroutines" {
		t.Fatalf("lexical hints = %+v, want delay -> [kotlinx.coroutines]", payload.LexicalHintsByCallee)
	}
	if hints := payload.RuleProfiles[0].LexicalHintsByCallee["delay"]; len(hints) != 1 || hints[0] != "kotlinx.coroutines" {
		t.Fatalf("profile lexical hints = %+v, want delay -> [kotlinx.coroutines]", payload.RuleProfiles[0].LexicalHintsByCallee)
	}
	if skips := payload.LexicalSkipByCallee["w"]; len(skips) != 1 || skips[0] != "Log" {
		t.Fatalf("lexical skips = %+v, want w -> [Log]", payload.LexicalSkipByCallee)
	}
	if skips := payload.RuleProfiles[0].LexicalSkipByCallee["w"]; len(skips) != 1 || skips[0] != "Log" {
		t.Fatalf("profile lexical skips = %+v, want w -> [Log]", payload.RuleProfiles[0].LexicalSkipByCallee)
	}
	if len(payload.RuleProfiles[0].DerivedCalleeNames) != 1 || payload.RuleProfiles[0].DerivedCalleeNames[0] != "oldCall" {
		t.Fatalf("derived callee names = %+v, want [oldCall]", payload.RuleProfiles[0].DerivedCalleeNames)
	}
}

func TestDaemonParamsIncludeRuleProfilesJSON(t *testing.T) {
	// Verify that a summary with rule profiles serializes correctly so the
	// Kotlin daemon's extractJsonObjectArray can find "callFilterRuleProfiles".
	summary := FinalizeCallTargetFilter(CallTargetFilterSummary{
		Enabled:     true,
		CalleeNames: []string{"launch"},
		RuleProfiles: []CallTargetRuleProfile{{
			RuleID:      "CoroutineSuspend",
			AllCalls:    false,
			CalleeNames: []string{"launch"},
			TargetFQNs:  []string{"kotlinx.coroutines.launch"},
		}},
	})

	params := map[string]interface{}{
		"callFilterCalleeNames": summary.CalleeNames,
		"callFilterRuleProfiles": summary.RuleProfiles,
	}
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"callFilterRuleProfiles"`) {
		t.Fatalf("serialized params missing callFilterRuleProfiles key: %s", s)
	}
	if !strings.Contains(s, `"CoroutineSuspend"`) {
		t.Fatalf("serialized params missing ruleID: %s", s)
	}
}

func TestWriteCallTargetFilterFile_DisabledReturnsEmpty(t *testing.T) {
	path, err := WriteCallTargetFilterFile(CallTargetFilterSummary{Enabled: false}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatalf("path = %q, want empty", path)
	}
}
