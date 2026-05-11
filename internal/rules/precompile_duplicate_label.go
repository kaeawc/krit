package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// PrecompileDuplicateLabelRule flags repeated constant labels in a
// subject-bearing `when` expression. Mirrors kotlinc's
// DUPLICATE_LABEL_IN_WHEN: when (x) { 1 -> a; 1 -> b } — the second `1`
// branch is unreachable.
//
// Conservative by construction: only literal labels (numeric, boolean,
// char, null, and string literals without interpolation) form a key.
// Identifiers, qualified references, range tests, and type tests are
// skipped because they need resolver context to deduplicate safely.
// Subject-less `when` (free boolean conditions) is also skipped — those
// are not labels.
type PrecompileDuplicateLabelRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *PrecompileDuplicateLabelRule) Confidence() float64 { return 0.95 }

func (r *PrecompileDuplicateLabelRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if _, ok := file.FlatFindChild(idx, "when_subject"); !ok {
		return
	}
	seen := map[string]uint32{}
	for entry := file.FlatFirstChild(idx); entry != 0; entry = file.FlatNextSib(entry) {
		if file.FlatType(entry) != "when_entry" {
			continue
		}
		for cond := file.FlatFirstChild(entry); cond != 0; cond = file.FlatNextSib(cond) {
			if file.FlatType(cond) != "when_condition" {
				continue
			}
			key := whenConditionDuplicateKey(file, cond)
			if key == "" {
				continue
			}
			if first, dup := seen[key]; dup {
				ctx.EmitAt(file.FlatRow(cond)+1, file.FlatCol(cond)+1,
					fmt.Sprintf("Duplicate `when` label `%s`; already covered on line %d.",
						key, file.FlatRow(first)+1))
				continue
			}
			seen[key] = cond
		}
	}
}

// whenConditionDuplicateKey returns a stable key when cond is a constant
// label safe to deduplicate without resolver context. Returns "" for
// non-literal conditions so the rule fails closed.
func whenConditionDuplicateKey(file *scanner.File, cond uint32) string {
	var named uint32
	count := 0
	for child := file.FlatFirstChild(cond); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		count++
		named = child
	}
	if count != 1 {
		return ""
	}
	switch file.FlatType(named) {
	case "integer_literal", "hex_literal", "bin_literal", "real_literal",
		"boolean_literal", "character_literal", "long_literal",
		"unsigned_literal", "null_literal", "null":
		return strings.TrimSpace(file.FlatNodeText(named))
	case "string_literal", "line_string_literal":
		if stringLiteralHasInterpolation(file, named) {
			return ""
		}
		return strings.TrimSpace(file.FlatNodeText(named))
	}
	return ""
}

func stringLiteralHasInterpolation(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "interpolated_identifier", "interpolated_expression",
			"line_str_ref", "line_string_expression",
			"multi_line_str_ref", "multi_line_string_expression":
			return true
		}
	}
	return false
}
