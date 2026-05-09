package fixer

import (
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestCanonicalSortByteFixes_StableTotalOrder asserts the byte-mode
// canonical comparator yields the same total order regardless of
// input permutation, including ties on StartByte. Regression for #26:
// `sort.Slice` keyed only on StartByte had undefined tie-breaking
// behavior, causing autofix overlap resolution to flap run-to-run.
func TestCanonicalSortByteFixes_StableTotalOrder(t *testing.T) {
	canonical := []textFixRow{
		{rule: "rule-a", fix: scanner.Fix{StartByte: 30, EndByte: 35, ByteMode: true}},
		// Same StartByte as below: longer span wins (EndByte desc tiebreaker).
		{rule: "rule-z", fix: scanner.Fix{StartByte: 20, EndByte: 30, ByteMode: true}},
		{rule: "rule-a", fix: scanner.Fix{StartByte: 20, EndByte: 22, ByteMode: true}},
		{rule: "rule-b", fix: scanner.Fix{StartByte: 20, EndByte: 22, ByteMode: true}},
		// Same StartByte AND EndByte: rule asc tiebreaker.
		{rule: "rule-a", fix: scanner.Fix{StartByte: 10, EndByte: 11, ByteMode: true}},
		{rule: "rule-b", fix: scanner.Fix{StartByte: 10, EndByte: 11, ByteMode: true}},
		{rule: "rule-c", fix: scanner.Fix{StartByte: 5, EndByte: 6, ByteMode: true}},
	}

	permutations := [][]int{
		{6, 5, 4, 3, 2, 1, 0},
		{0, 6, 1, 5, 2, 4, 3},
		{4, 1, 6, 3, 0, 5, 2},
		{2, 0, 4, 6, 1, 3, 5},
	}
	for k, perm := range permutations {
		got := make([]textFixRow, len(perm))
		for i, p := range perm {
			got[i] = canonical[p]
		}
		canonicalSortByteFixes(got)
		if !reflect.DeepEqual(got, canonical) {
			t.Fatalf("perm %d:\n  got:  %#v\n  want: %#v", k, got, canonical)
		}
	}
}

// TestCanonicalSortLineFixes_StableTotalOrder is the line-mode
// counterpart.
func TestCanonicalSortLineFixes_StableTotalOrder(t *testing.T) {
	canonical := []textFixRow{
		{rule: "rule-a", fix: scanner.Fix{StartLine: 50, EndLine: 55}},
		{rule: "rule-z", fix: scanner.Fix{StartLine: 20, EndLine: 30}},
		{rule: "rule-a", fix: scanner.Fix{StartLine: 20, EndLine: 22}},
		{rule: "rule-b", fix: scanner.Fix{StartLine: 20, EndLine: 22}},
		{rule: "rule-x", fix: scanner.Fix{StartLine: 5, EndLine: 6}},
	}
	permutations := [][]int{
		{4, 3, 2, 1, 0},
		{0, 4, 1, 3, 2},
		{2, 0, 3, 4, 1},
	}
	for k, perm := range permutations {
		got := make([]textFixRow, len(perm))
		for i, p := range perm {
			got[i] = canonical[p]
		}
		canonicalSortLineFixes(got)
		if !reflect.DeepEqual(got, canonical) {
			t.Fatalf("perm %d:\n  got:  %#v\n  want: %#v", k, got, canonical)
		}
	}
}

// TestApplyByteFixes_OverlapResolutionIsDeterministic asserts the
// end-to-end fix application: when two rules collide at the same
// start position, the same rule's fix wins every run, and the same
// rule appears in DroppedFixes every run.
func TestApplyByteFixes_OverlapResolutionIsDeterministic(t *testing.T) {
	const original = "abcdefghij"

	// Two rules, both start at byte 2, different end positions.
	// Tiebreaker: longer span wins (EndByte desc).
	makeFixes := func() []textFixRow {
		return []textFixRow{
			{rule: "rule-z-shorter", fix: scanner.Fix{
				StartByte:   2,
				EndByte:     5,
				Replacement: "Z",
				ByteMode:    true,
			}},
			{rule: "rule-a-longer", fix: scanner.Fix{
				StartByte:   2,
				EndByte:     8,
				Replacement: "A",
				ByteMode:    true,
			}},
		}
	}

	// Run many times — without canonical tiebreakers, sort.Slice can
	// pick either order. With canonical tiebreakers we deterministically
	// keep the longer span and drop the shorter one.
	var refResult string
	var refDropped []DroppedFix
	for i := 0; i < 200; i++ {
		got, dropped := applyByteFixes(original, makeFixes(), "/test.kt")
		if i == 0 {
			refResult = got
			refDropped = dropped
			continue
		}
		if got != refResult {
			t.Fatalf("iter %d: result diverged: got %q, want %q", i, got, refResult)
		}
		if !reflect.DeepEqual(dropped, refDropped) {
			t.Fatalf("iter %d: DroppedFixes diverged:\n  got:  %#v\n  want: %#v", i, dropped, refDropped)
		}
	}

	// Spot-check the canonical winner: longer span ("rule-a-longer")
	// is kept; shorter span ("rule-z-shorter") is dropped.
	if len(refDropped) != 1 || refDropped[0].Rule != "rule-z-shorter" {
		t.Fatalf("expected rule-z-shorter dropped, got %#v", refDropped)
	}
	if refResult != "abAij" {
		t.Fatalf("expected longer fix applied, got %q", refResult)
	}
}

// TestApplyByteFixes_RuleTiebreakerIsDeterministic covers the case
// where two rules collide on identical (StartByte, EndByte): rule
// name asc decides the winner.
func TestApplyByteFixes_RuleTiebreakerIsDeterministic(t *testing.T) {
	const original = "abcdefghij"
	makeFixes := func() []textFixRow {
		return []textFixRow{
			{rule: "zzz-rule", fix: scanner.Fix{
				StartByte: 2, EndByte: 5, Replacement: "Z", ByteMode: true,
			}},
			{rule: "aaa-rule", fix: scanner.Fix{
				StartByte: 2, EndByte: 5, Replacement: "A", ByteMode: true,
			}},
		}
	}
	for i := 0; i < 200; i++ {
		got, dropped := applyByteFixes(original, makeFixes(), "/test.kt")
		if got != "abAfghij" {
			t.Fatalf("iter %d: expected lexically-smaller rule to win, got %q", i, got)
		}
		if len(dropped) != 1 || dropped[0].Rule != "zzz-rule" {
			t.Fatalf("iter %d: expected zzz-rule dropped, got %#v", i, dropped)
		}
	}
}
