package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

// aliasIndex builds a ResourceIndex containing the given alias items.
func aliasIndex(items ...android.AliasItem) *android.ResourceIndex {
	idx := emptyIndex()
	idx.AliasItems = append(idx.AliasItems, items...)
	return idx
}

func TestReferenceType(t *testing.T) {
	r := findResourceRule(t, "ReferenceType")

	t.Run("type mismatch triggers on item line", func(t *testing.T) {
		idx := aliasIndex(android.AliasItem{
			Name:     "invalid1",
			Type:     "string",
			Value:    "@layout/other",
			FilePath: "res/values/aliases.xml",
			Line:     3,
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 3 {
			t.Fatalf("expected finding on line 3, got %d", findings[0].Line)
		}
		if !strings.Contains(findings[0].Message, "invalid1") {
			t.Fatalf("message missing alias name: %q", findings[0].Message)
		}
	})

	t.Run("matching type is clean", func(t *testing.T) {
		idx := aliasIndex(android.AliasItem{
			Name: "ok", Type: "string", Value: "@string/indirect", Line: 4,
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d (%v)", len(findings), findings)
		}
	})

	t.Run("plain text value is clean", func(t *testing.T) {
		idx := aliasIndex(android.AliasItem{
			Name: "string1", Type: "string", Value: "Plain String", Line: 5,
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for plain text, got %d (%v)", len(findings), findings)
		}
	})

	t.Run("id item with no reference is clean", func(t *testing.T) {
		// <item type="id" name="x"/> has no reference value: not a mismatch.
		idx := aliasIndex(android.AliasItem{
			Name: "generated", Type: "id", Value: "", Line: 6,
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for empty id item, got %d (%v)", len(findings), findings)
		}
	})

	t.Run("framework namespace reference matches on type", func(t *testing.T) {
		// @android:string/ok references a string; declared type string -> clean.
		idx := aliasIndex(android.AliasItem{
			Name: "fw", Type: "string", Value: "@android:string/ok", Line: 7,
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for matching framework ref, got %d (%v)", len(findings), findings)
		}
	})

	t.Run("framework namespace reference still detects mismatch", func(t *testing.T) {
		idx := aliasIndex(android.AliasItem{
			Name: "fw2", Type: "string", Value: "@android:drawable/ic", Line: 8,
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for framework type mismatch, got %d", len(findings))
		}
	})

	t.Run("multiple aliases each evaluated", func(t *testing.T) {
		idx := aliasIndex(
			android.AliasItem{Name: "bad1", Type: "string", Value: "@layout/a", Line: 3},
			android.AliasItem{Name: "good", Type: "color", Value: "@color/c", Line: 4},
			android.AliasItem{Name: "bad2", Type: "drawable", Value: "@string/s", Line: 5},
		)
		findings := runResourceRule(r, idx)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d (%v)", len(findings), findings)
		}
	})
}
