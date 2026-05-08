package scanner

import (
	"reflect"
	"testing"
)

func TestFindingsBundleStore_RoundTrip(t *testing.T) {
	root := t.TempDir()
	fp := RunFingerprint{
		Version:      "v1",
		Rules:        "rules",
		Config:       "config",
		SourceSet:    "sources",
		CrossFile:    "cross",
		Android:      "android",
		LibraryFacts: "libs",
	}
	cols := CollectFindings([]Finding{{
		File:     "/repo/A.kt",
		Line:     1,
		Col:      2,
		Rule:     "RuleA",
		Severity: "warning",
		Message:  "message",
	}})
	store := DiskFindingsBundleStore{}
	if err := store.Save(root, fp, &cols); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, ok := store.Load(root, fp)
	if !ok {
		t.Fatal("Load returned ok=false")
	}
	if !reflect.DeepEqual(got.Findings(), cols.Findings()) {
		t.Fatalf("findings = %#v, want %#v", got.Findings(), cols.Findings())
	}
}

func TestFindingsBundleStore_FingerprintChangeMisses(t *testing.T) {
	root := t.TempDir()
	store := DiskFindingsBundleStore{}
	fp := RunFingerprint{Version: "v1", Rules: "rules"}
	cols := CollectFindings([]Finding{{File: "/repo/A.kt", Rule: "RuleA"}})
	if err := store.Save(root, fp, &cols); err != nil {
		t.Fatalf("Save: %v", err)
	}
	fp.Rules = "other-rules"
	if _, ok := store.Load(root, fp); ok {
		t.Fatal("Load returned hit after fingerprint changed")
	}
}

func TestConservativeDeltaPlanner_AllowsSingleFileWhenGlobalInputsStable(t *testing.T) {
	prev := RunFingerprint{Version: "v1", Rules: "rules", Config: "config", CrossFile: "cross", Android: "android", LibraryFacts: "libs"}
	cur := prev
	cur.SourceSet = "sources-after-edit"
	plan := (ConservativeDeltaPlanner{}).Plan(prev, cur, []string{"/repo/A.kt"})
	if !plan.ReusePrevious {
		t.Fatal("expected reusable delta plan")
	}
	if !reflect.DeepEqual(plan.AffectedPaths, []string{"/repo/A.kt"}) {
		t.Fatalf("AffectedPaths = %v", plan.AffectedPaths)
	}
}

func TestConservativeDeltaPlanner_GlobalFingerprintChangeDisablesDelta(t *testing.T) {
	prev := RunFingerprint{Version: "v1", Rules: "rules", Config: "config"}
	cur := prev
	cur.Config = "changed"
	plan := (ConservativeDeltaPlanner{}).Plan(prev, cur, []string{"/repo/A.kt"})
	if plan.ReusePrevious {
		t.Fatal("expected config change to disable delta")
	}
}

func TestApplyDelta_ReplacesAffectedRowsAndKeepsProjectRows(t *testing.T) {
	prev := CollectFindings([]Finding{
		{File: "/repo/A.kt", Line: 2, Rule: "RuleA", Message: "old"},
		{File: "/repo/B.kt", Line: 1, Rule: "RuleA", Message: "keep"},
		{File: "", Line: 1, Rule: "ProjectRule", Message: "project"},
	})
	replacement := CollectFindings([]Finding{
		{File: "/repo/A.kt", Line: 1, Rule: "RuleA", Message: "new"},
	})
	got := ApplyDelta(&prev, &replacement, []string{"/repo/A.kt"})
	messages := make([]string, 0, got.Len())
	for i := 0; i < got.Len(); i++ {
		messages = append(messages, got.MessageAt(i))
	}
	want := []string{"project", "new", "keep"}
	if !reflect.DeepEqual(messages, want) {
		t.Fatalf("messages = %v, want %v", messages, want)
	}
}
