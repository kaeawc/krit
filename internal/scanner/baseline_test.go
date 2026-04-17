package scanner

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadBaseline_ValidXML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.xml")

	content := `<?xml version="1.0" encoding="UTF-8"?>
<SmellBaseline>
  <ManuallySuppressedIssues>
    <ID>MagicNumber:Foo.kt:fun bar()</ID>
    <ID>WildcardImport:Baz.kt:import foo.*</ID>
  </ManuallySuppressedIssues>
  <CurrentIssues>
    <ID>LongMethod:Main.kt:fun main()</ID>
    <ID>EmptyBlock:Util.kt:fun helper()</ID>
    <ID>UnusedImport:App.kt:import unused</ID>
  </CurrentIssues>
</SmellBaseline>`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	b, err := LoadBaseline(path)
	if err != nil {
		t.Fatalf("LoadBaseline failed: %v", err)
	}

	if len(b.ManuallySuppressed) != 2 {
		t.Errorf("expected 2 manually suppressed entries, got %d", len(b.ManuallySuppressed))
	}
	if len(b.CurrentIssues) != 3 {
		t.Errorf("expected 3 current issues, got %d", len(b.CurrentIssues))
	}

	if !b.ManuallySuppressed["MagicNumber:Foo.kt:fun bar()"] {
		t.Error("expected MagicNumber entry in ManuallySuppressed")
	}
	if !b.CurrentIssues["LongMethod:Main.kt:fun main()"] {
		t.Error("expected LongMethod entry in CurrentIssues")
	}
}

func TestLoadBaseline_MissingFile(t *testing.T) {
	_, err := LoadBaseline("/nonexistent/path/baseline.xml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadBaseline_MalformedXML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.xml")

	if err := os.WriteFile(path, []byte("<<<not xml at all>>>"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadBaseline(path)
	if err == nil {
		t.Fatal("expected error for malformed XML, got nil")
	}
}

func TestLoadBaseline_EmptyBaseline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.xml")

	content := `<?xml version="1.0" encoding="UTF-8"?>
<SmellBaseline>
  <ManuallySuppressedIssues></ManuallySuppressedIssues>
  <CurrentIssues></CurrentIssues>
</SmellBaseline>`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	b, err := LoadBaseline(path)
	if err != nil {
		t.Fatalf("LoadBaseline failed: %v", err)
	}
	if len(b.ManuallySuppressed) != 0 {
		t.Errorf("expected 0 manually suppressed, got %d", len(b.ManuallySuppressed))
	}
	if len(b.CurrentIssues) != 0 {
		t.Errorf("expected 0 current issues, got %d", len(b.CurrentIssues))
	}
}

func TestWriteBaseline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.xml")

	findings := []Finding{
		{File: "/project/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "avoid magic numbers"},
		{File: "/project/src/Bar.kt", Rule: "LongMethod", Line: 20, Message: "method too long"},
		{File: "/project/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "avoid magic numbers"}, // duplicate
	}

	if err := WriteBaseline(path, findings, ""); err != nil {
		t.Fatalf("WriteBaseline failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written baseline: %v", err)
	}

	// Verify it's valid XML by parsing it back
	var db DetektBaseline
	if err := xml.Unmarshal(data, &db); err != nil {
		t.Fatalf("written baseline is not valid XML: %v", err)
	}

	// Should have 2 unique IDs (duplicate finding deduplicated)
	if len(db.CurrentIssues.IDs) != 2 {
		t.Errorf("expected 2 current issue IDs, got %d", len(db.CurrentIssues.IDs))
	}

	// ManuallySuppressed should be empty
	if len(db.ManuallySuppressed.IDs) != 0 {
		t.Errorf("expected 0 manually suppressed IDs, got %d", len(db.ManuallySuppressed.IDs))
	}

	// Verify round-trip: load it back with LoadBaseline
	b, err := LoadBaseline(path)
	if err != nil {
		t.Fatalf("round-trip LoadBaseline failed: %v", err)
	}
	if len(b.CurrentIssues) != 2 {
		t.Errorf("round-trip expected 2 current issues, got %d", len(b.CurrentIssues))
	}
}

func TestWriteBaseline_WithBasePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.xml")

	findings := []Finding{
		{File: "/project/moduleA/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "msg"},
		{File: "/project/moduleB/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "msg"},
	}

	if err := WriteBaseline(path, findings, "/project"); err != nil {
		t.Fatalf("WriteBaseline failed: %v", err)
	}

	b, err := LoadBaseline(path)
	if err != nil {
		t.Fatalf("LoadBaseline failed: %v", err)
	}

	// With basePath, same-named files in different modules should produce different IDs
	if len(b.CurrentIssues) != 2 {
		t.Errorf("expected 2 distinct IDs with basePath, got %d", len(b.CurrentIssues))
	}
}

func TestWriteBaselineColumns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "columns.xml")

	columns := CollectFindings([]Finding{
		{File: "/project/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "avoid magic numbers"},
		{File: "/project/src/Bar.kt", Rule: "LongMethod", Line: 20, Message: "method too long"},
		{File: "/project/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "avoid magic numbers"},
	})

	if err := WriteBaselineColumns(path, &columns, ""); err != nil {
		t.Fatalf("WriteBaselineColumns failed: %v", err)
	}

	b, err := LoadBaseline(path)
	if err != nil {
		t.Fatalf("LoadBaseline failed: %v", err)
	}
	if len(b.CurrentIssues) != 2 {
		t.Errorf("expected 2 current issues, got %d", len(b.CurrentIssues))
	}
}

func TestWriteBaseline_MatchesWriteBaselineColumns(t *testing.T) {
	dir := t.TempDir()
	slicePath := filepath.Join(dir, "slice.xml")
	columnPath := filepath.Join(dir, "columns.xml")

	findings := []Finding{
		{File: "/project/moduleA/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "avoid magic numbers"},
		{File: "/project/moduleB/src/Foo.kt", Rule: "LongMethod", Line: 20, Message: "method too long"},
		{File: "/project/moduleA/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "avoid magic numbers"},
	}
	columns := CollectFindings(findings)

	if err := WriteBaseline(slicePath, findings, "/project"); err != nil {
		t.Fatalf("WriteBaseline failed: %v", err)
	}
	if err := WriteBaselineColumns(columnPath, &columns, "/project"); err != nil {
		t.Fatalf("WriteBaselineColumns failed: %v", err)
	}

	sliceData, err := os.ReadFile(slicePath)
	if err != nil {
		t.Fatalf("read slice baseline: %v", err)
	}
	columnData, err := os.ReadFile(columnPath)
	if err != nil {
		t.Fatalf("read column baseline: %v", err)
	}

	if string(sliceData) != string(columnData) {
		t.Fatalf("baseline XML mismatch:\nwant:\n%s\ngot:\n%s", string(columnData), string(sliceData))
	}
}

func TestFilterByBaseline(t *testing.T) {
	baseline := &Baseline{
		ManuallySuppressed: map[string]bool{
			"MagicNumber:Foo.kt:$MagicNumber$avoid magic numbers": true,
		},
		CurrentIssues: map[string]bool{
			"LongMethod:Bar.kt:$LongMethod$method too long": true,
		},
	}

	findings := []Finding{
		{File: "/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "avoid magic numbers"},
		{File: "/src/Bar.kt", Rule: "LongMethod", Line: 20, Message: "method too long"},
		{File: "/src/Baz.kt", Rule: "EmptyBlock", Line: 5, Message: "empty block"},
	}

	filtered := FilterByBaseline(findings, baseline, "")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 finding after filter, got %d", len(filtered))
	}
	if filtered[0].Rule != "EmptyBlock" {
		t.Errorf("expected EmptyBlock to pass through, got %s", filtered[0].Rule)
	}
}

func TestFilterByBaseline_NilBaseline(t *testing.T) {
	findings := []Finding{
		{File: "/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "msg"},
	}

	filtered := FilterByBaseline(findings, nil, "")
	if len(filtered) != len(findings) {
		t.Errorf("nil baseline should return all findings; got %d, want %d", len(filtered), len(findings))
	}
}

func TestFilterByBaseline_CompatFallback(t *testing.T) {
	// Baseline has filename-only ID (detekt style), filtering uses basePath
	baseline := &Baseline{
		ManuallySuppressed: map[string]bool{},
		CurrentIssues: map[string]bool{
			"MagicNumber:Foo.kt:$MagicNumber$msg": true,
		},
	}

	findings := []Finding{
		{File: "/project/module/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "msg"},
	}

	// With basePath, the relative-path ID won't match, but the compat (filename-only) ID should
	filtered := FilterByBaseline(findings, baseline, "/project")
	if len(filtered) != 0 {
		t.Errorf("expected compat fallback to filter finding, got %d remaining", len(filtered))
	}
}

func TestFilterColumnsByBaseline(t *testing.T) {
	baseline := &Baseline{
		ManuallySuppressed: map[string]bool{
			"MagicNumber:Foo.kt:$MagicNumber$avoid magic numbers": true,
		},
		CurrentIssues: map[string]bool{
			"LongMethod:Bar.kt:$LongMethod$method too long": true,
		},
	}

	source := CollectFindings([]Finding{
		{File: "/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "avoid magic numbers"},
		{File: "/src/Bar.kt", Rule: "LongMethod", Line: 20, Message: "method too long"},
		{
			File:    "/src/Baz.kt",
			Rule:    "EmptyBlock",
			Line:    5,
			Message: "empty block",
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("kept"),
			},
		},
	})

	filtered := FilterColumnsByBaseline(&source, baseline, "")
	if filtered.Len() != 1 {
		t.Fatalf("expected 1 finding after filter, got %d", filtered.Len())
	}
	if got := filtered.RuleAt(0); got != "EmptyBlock" {
		t.Fatalf("expected EmptyBlock to pass through, got %s", got)
	}

	source.BinaryFixPool[0].Content[0] = 'X'
	if string(filtered.BinaryFixPool[0].Content) != "kept" {
		t.Fatalf("filtered binary fix should be independent, got %q", string(filtered.BinaryFixPool[0].Content))
	}
}

func TestFilterColumnsByBaseline_CompatFallback(t *testing.T) {
	baseline := &Baseline{
		CurrentIssues: map[string]bool{
			"MagicNumber:Foo.kt:$MagicNumber$msg": true,
		},
	}

	source := CollectFindings([]Finding{
		{File: "/project/module/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "msg"},
	})

	filtered := FilterColumnsByBaseline(&source, baseline, "/project")
	if filtered.Len() != 0 {
		t.Errorf("expected compat fallback to filter finding, got %d remaining", filtered.Len())
	}
}

func TestFilterByBaseline_MatchesFilterColumnsByBaseline(t *testing.T) {
	baseline := &Baseline{
		ManuallySuppressed: map[string]bool{
			"MagicNumber:moduleA/src/Foo.kt:$MagicNumber$avoid magic numbers": true,
		},
		CurrentIssues: map[string]bool{
			"LongMethod:moduleB/src/Bar.kt:$LongMethod$method too long": true,
		},
	}

	findings := []Finding{
		{
			File:    "/project/moduleA/src/Foo.kt",
			Rule:    "MagicNumber",
			Line:    10,
			Message: "avoid magic numbers",
			Fix: &Fix{
				StartLine:   10,
				EndLine:     10,
				Replacement: "fixed()",
			},
		},
		{
			File:    "/project/moduleB/src/Bar.kt",
			Rule:    "LongMethod",
			Line:    20,
			Message: "method too long",
		},
		{
			File:    "/project/moduleC/src/Baz.kt",
			Rule:    "EmptyBlock",
			Line:    5,
			Message: "empty block",
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("kept"),
			},
		},
	}
	columns := CollectFindings(findings)

	got := FilterByBaseline(findings, baseline, "/project")
	wantColumns := FilterColumnsByBaseline(&columns, baseline, "/project")
	want := wantColumns.Findings()

	if len(got) != len(want) {
		t.Fatalf("filtered finding count mismatch: want %d, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i].BinaryFix != nil && want[i].BinaryFix != nil {
			if string(got[i].BinaryFix.Content) != string(want[i].BinaryFix.Content) {
				t.Fatalf("binary fix content mismatch at row %d: want %q, got %q", i, string(want[i].BinaryFix.Content), string(got[i].BinaryFix.Content))
			}
			got[i].BinaryFix.Content = nil
			want[i].BinaryFix.Content = nil
		}
	}
	if got[0].BinaryFix == nil || want[0].BinaryFix == nil {
		t.Fatal("expected surviving binary fix in both filtering paths")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered findings mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBaselineID_Deterministic(t *testing.T) {
	f := Finding{File: "/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "avoid magic"}

	id1 := BaselineID(f, "fun calculate()", "")
	id2 := BaselineID(f, "fun calculate()", "")

	if id1 != id2 {
		t.Errorf("BaselineID not deterministic: %q != %q", id1, id2)
	}

	expected := "MagicNumber:Foo.kt:fun calculate()"
	if id1 != expected {
		t.Errorf("BaselineID = %q, want %q", id1, expected)
	}
}

func TestBaselineID_DifferentFindings(t *testing.T) {
	f1 := Finding{File: "/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "msg1"}
	f2 := Finding{File: "/src/Bar.kt", Rule: "LongMethod", Line: 20, Message: "msg2"}

	id1 := BaselineID(f1, "", "")
	id2 := BaselineID(f2, "", "")

	if id1 == id2 {
		t.Errorf("different findings should produce different IDs: both got %q", id1)
	}
}

func TestBaselineID_WithBasePath(t *testing.T) {
	f := Finding{File: "/project/module/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "msg"}

	idNoBase := BaselineID(f, "sig", "")
	idWithBase := BaselineID(f, "sig", "/project")

	if idNoBase == idWithBase {
		t.Error("basePath should change the ID")
	}

	if idNoBase != "MagicNumber:Foo.kt:sig" {
		t.Errorf("without basePath got %q", idNoBase)
	}
	if idWithBase != "MagicNumber:module/src/Foo.kt:sig" {
		t.Errorf("with basePath got %q", idWithBase)
	}
}

func TestBaselineIDAt_MatchesFindingAPI(t *testing.T) {
	columns := CollectFindings([]Finding{
		{File: "/project/module/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "msg"},
	})

	got := BaselineIDAt(&columns, 0, "sig", "/project")
	want := BaselineID(Finding{
		File:    "/project/module/src/Foo.kt",
		Rule:    "MagicNumber",
		Message: "msg",
	}, "sig", "/project")

	if got != want {
		t.Fatalf("BaselineIDAt = %q, want %q", got, want)
	}
}

func TestBaselineID_EmptySignature(t *testing.T) {
	f := Finding{File: "/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "some message"}

	id := BaselineID(f, "", "")

	expected := "MagicNumber:Foo.kt:$MagicNumber$some message"
	if id != expected {
		t.Errorf("BaselineID with empty signature = %q, want %q", id, expected)
	}
}

func TestBaselineIDCompat(t *testing.T) {
	f := Finding{File: "/project/module/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "msg"}

	id := BaselineIDCompat(f, "fun calc()")
	expected := "MagicNumber:Foo.kt:fun calc()"
	if id != expected {
		t.Errorf("BaselineIDCompat = %q, want %q", id, expected)
	}
}

func TestBaselineIDCompatAt_MatchesFindingAPI(t *testing.T) {
	columns := CollectFindings([]Finding{
		{File: "/project/module/src/Foo.kt", Rule: "MagicNumber", Line: 10, Message: "msg"},
	})

	got := BaselineIDCompatAt(&columns, 0, "fun calc()")
	want := BaselineIDCompat(Finding{
		File:    "/project/module/src/Foo.kt",
		Rule:    "MagicNumber",
		Message: "msg",
	}, "fun calc()")

	if got != want {
		t.Fatalf("BaselineIDCompatAt = %q, want %q", got, want)
	}
}

func TestFindingSignature(t *testing.T) {
	f := Finding{File: "/src/Foo.kt", Rule: "MagicNumber", Line: 2}
	lines := []string{
		"package com.example",
		"fun calculate(x: Int) {",
		"    return x * 42",
		"}",
	}

	sig := FindingSignature(f, lines)
	if sig != "fun calculate(x: Int)" {
		t.Errorf("FindingSignature = %q, want %q", sig, "fun calculate(x: Int)")
	}
}

func TestFindingSignature_WithEquals(t *testing.T) {
	f := Finding{File: "/src/Foo.kt", Rule: "MagicNumber", Line: 1}
	lines := []string{
		"val x = 42",
	}

	sig := FindingSignature(f, lines)
	if sig != "val x" {
		t.Errorf("FindingSignature = %q, want %q", sig, "val x")
	}
}

func TestFindingSignature_OutOfBounds(t *testing.T) {
	f := Finding{File: "/src/Foo.kt", Rule: "MagicNumber", Line: 0}
	lines := []string{"val x = 1"}

	sig := FindingSignature(f, lines)
	if sig != "" {
		t.Errorf("expected empty signature for line 0, got %q", sig)
	}

	f.Line = 10
	sig = FindingSignature(f, lines)
	if sig != "" {
		t.Errorf("expected empty signature for line beyond file, got %q", sig)
	}
}

func TestBaseline_Contains(t *testing.T) {
	b := &Baseline{
		ManuallySuppressed: map[string]bool{
			"RuleA:Foo.kt:sig1": true,
		},
		CurrentIssues: map[string]bool{
			"RuleB:Bar.kt:sig2": true,
		},
	}

	tests := []struct {
		id   string
		want bool
	}{
		{"RuleA:Foo.kt:sig1", true},
		{"RuleB:Bar.kt:sig2", true},
		{"RuleC:Baz.kt:sig3", false},
		{"", false},
	}

	for _, tt := range tests {
		got := b.Contains(tt.id)
		if got != tt.want {
			t.Errorf("Contains(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestBaselineEntries(t *testing.T) {
	b := &Baseline{
		ManuallySuppressed: map[string]bool{
			"MagicNumber:src/Foo.kt:fun foo()": true,
		},
		CurrentIssues: map[string]bool{
			"LongMethod:src/Bar.kt:$LongMethod$msg": true,
		},
	}

	entries := b.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Section != "CurrentIssues" {
		t.Fatalf("expected CurrentIssues entry first, got %s", entries[0].Section)
	}
	if entries[0].Rule != "LongMethod" || entries[0].Path != "src/Bar.kt" || entries[0].Signature != "$LongMethod$msg" {
		t.Fatalf("unexpected parsed CurrentIssues entry: %+v", entries[0])
	}

	if entries[1].Section != "ManuallySuppressedIssues" {
		t.Fatalf("expected ManuallySuppressedIssues entry second, got %s", entries[1].Section)
	}
	if entries[1].Rule != "MagicNumber" || entries[1].Path != "src/Foo.kt" || entries[1].Signature != "fun foo()" {
		t.Fatalf("unexpected parsed ManuallySuppressed entry: %+v", entries[1])
	}
}
