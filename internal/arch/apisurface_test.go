package arch

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestExtractSurface_Empty(t *testing.T) {
	entries := ExtractSurface(nil)
	if len(entries) != 0 {
		t.Errorf("expected empty, got %d entries", len(entries))
	}
}

func TestExtractSurface_PublicOnly(t *testing.T) {
	symbols := []scanner.Symbol{
		{Name: "UserService", Kind: "class", Visibility: "public"},
		{Name: "helperFunc", Kind: "function", Visibility: "private"},
		{Name: "internalUtil", Kind: "function", Visibility: "internal"},
		{Name: "getUser", Kind: "function", Visibility: "public"},
		{Name: "BaseService", Kind: "class", Visibility: "protected"},
	}

	entries := ExtractSurface(symbols)
	if len(entries) != 3 {
		t.Fatalf("expected 3 public/protected entries, got %d", len(entries))
	}

	// Should be sorted by kind+name
	expected := []struct{ kind, name string }{
		{"class", "BaseService"},
		{"class", "UserService"},
		{"function", "getUser"},
	}
	for i, exp := range expected {
		if entries[i].Kind != exp.kind || entries[i].Name != exp.name {
			t.Errorf("entry[%d]: expected %s/%s, got %s/%s",
				i, exp.kind, exp.name, entries[i].Kind, entries[i].Name)
		}
	}
}

func TestExtractSurface_SkipsTestAndOverride(t *testing.T) {
	symbols := []scanner.Symbol{
		{Name: "UserService", Kind: "class", Visibility: "public"},
		{Name: "testHelper", Kind: "function", Visibility: "public", IsTest: true},
		{Name: "toString", Kind: "function", Visibility: "public", IsOverride: true},
	}

	entries := ExtractSurface(symbols)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (skip test+override), got %d", len(entries))
	}
	if entries[0].Name != "UserService" {
		t.Errorf("expected UserService, got %s", entries[0].Name)
	}
}

func TestExtractSurface_Deterministic(t *testing.T) {
	symbols1 := []scanner.Symbol{
		{Name: "B", Kind: "class", Visibility: "public"},
		{Name: "A", Kind: "class", Visibility: "public"},
		{Name: "C", Kind: "function", Visibility: "public"},
	}
	symbols2 := []scanner.Symbol{
		{Name: "C", Kind: "function", Visibility: "public"},
		{Name: "A", Kind: "class", Visibility: "public"},
		{Name: "B", Kind: "class", Visibility: "public"},
	}

	entries1 := ExtractSurface(symbols1)
	entries2 := ExtractSurface(symbols2)

	if len(entries1) != len(entries2) {
		t.Fatalf("lengths differ: %d vs %d", len(entries1), len(entries2))
	}
	for i := range entries1 {
		if entries1[i].Kind != entries2[i].Kind || entries1[i].Name != entries2[i].Name {
			t.Errorf("entry[%d] differs: %s/%s vs %s/%s",
				i, entries1[i].Kind, entries1[i].Name, entries2[i].Kind, entries2[i].Name)
		}
	}
}

func TestFormatAndParseSurface_RoundTrip(t *testing.T) {
	original := []APIEntry{
		{Kind: "class", Name: "UserService"},
		{Kind: "function", Name: "getUser"},
		{Kind: "interface", Name: "Repository"},
	}

	text := FormatSurface(original)
	parsed := ParseSurface(text)

	if len(parsed) != len(original) {
		t.Fatalf("expected %d entries after round-trip, got %d", len(original), len(parsed))
	}

	// FormatSurface sorts, so check sorted order
	expected := []struct{ kind, name string }{
		{"class", "UserService"},
		{"function", "getUser"},
		{"interface", "Repository"},
	}
	for i, exp := range expected {
		if parsed[i].Kind != exp.kind || parsed[i].Name != exp.name {
			t.Errorf("entry[%d]: expected %s/%s, got %s/%s",
				i, exp.kind, exp.name, parsed[i].Kind, parsed[i].Name)
		}
	}
}

func TestDiffSurfaces_NoChanges(t *testing.T) {
	surface := []APIEntry{
		{Kind: "class", Name: "A"},
		{Kind: "function", Name: "b"},
	}

	diffs := DiffSurfaces(surface, surface)
	if len(diffs) != 0 {
		t.Errorf("expected no diffs, got %d", len(diffs))
	}
}

func TestDiffSurfaces_Addition(t *testing.T) {
	old := []APIEntry{
		{Kind: "class", Name: "A"},
	}
	new := []APIEntry{
		{Kind: "class", Name: "A"},
		{Kind: "function", Name: "b"},
	}

	diffs := DiffSurfaces(old, new)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Change != "added" {
		t.Errorf("expected 'added', got %s", diffs[0].Change)
	}
	if diffs[0].Entry.Name != "b" {
		t.Errorf("expected name 'b', got %s", diffs[0].Entry.Name)
	}
}

func TestDiffSurfaces_Removal(t *testing.T) {
	old := []APIEntry{
		{Kind: "class", Name: "A"},
		{Kind: "function", Name: "b"},
	}
	new := []APIEntry{
		{Kind: "class", Name: "A"},
	}

	diffs := DiffSurfaces(old, new)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Change != "removed" {
		t.Errorf("expected 'removed', got %s", diffs[0].Change)
	}
	if diffs[0].Entry.Name != "b" {
		t.Errorf("expected name 'b', got %s", diffs[0].Entry.Name)
	}
}

func TestDiffSurfaces_Mixed(t *testing.T) {
	old := []APIEntry{
		{Kind: "class", Name: "A"},
		{Kind: "function", Name: "b"},
	}
	new := []APIEntry{
		{Kind: "class", Name: "A"},
		{Kind: "interface", Name: "C"},
	}

	diffs := DiffSurfaces(old, new)
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d", len(diffs))
	}

	// Sorted: "added" before "removed"
	if diffs[0].Change != "added" || diffs[0].Entry.Name != "C" {
		t.Errorf("diff[0]: expected added/C, got %s/%s", diffs[0].Change, diffs[0].Entry.Name)
	}
	if diffs[1].Change != "removed" || diffs[1].Entry.Name != "b" {
		t.Errorf("diff[1]: expected removed/b, got %s/%s", diffs[1].Change, diffs[1].Entry.Name)
	}
}
