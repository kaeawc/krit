package oracle

import (
	"encoding/json"
	"os"
	"testing"
)

func TestFinalizeCallTargetFilter_DerivesAndDedupsCallees(t *testing.T) {
	got := FinalizeCallTargetFilter(CallTargetFilterSummary{
		Enabled:     true,
		CalleeNames: []string{"delay", "await", "delay"},
		TargetFQNs:  []string{"kotlinx.coroutines.delay", "com.example.Foo#bar"},
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
}

func TestWriteCallTargetFilterFile(t *testing.T) {
	dir := t.TempDir()
	summary := FinalizeCallTargetFilter(CallTargetFilterSummary{
		Enabled:     true,
		CalleeNames: []string{"delay"},
		TargetFQNs:  []string{"kotlinx.coroutines.delay"},
		RuleProfiles: []CallTargetRuleProfile{{
			RuleID:               "SuspendRule",
			CalleeNames:          []string{"delay"},
			TargetFQNs:           []string{"kotlinx.coroutines.delay"},
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
	if len(payload.RuleProfiles[0].DerivedCalleeNames) != 1 || payload.RuleProfiles[0].DerivedCalleeNames[0] != "oldCall" {
		t.Fatalf("derived callee names = %+v, want [oldCall]", payload.RuleProfiles[0].DerivedCalleeNames)
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
