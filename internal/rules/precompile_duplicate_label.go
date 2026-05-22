package rules

import (
	"fmt"
	"strconv"
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

func (r *PrecompileDuplicateLabelRule) Confidence() float64 { return api.ConfidenceVeryHigh }

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
//
// Integer literals are normalized (underscore separators stripped,
// hex/binary forms canonicalized to decimal) so kotlinc-equivalent forms
// like `0x01` and `1` dedupe. String, char, and real literals are compared
// textually, so escape-sequence-equivalent forms (e.g. "a" vs "a")
// are not deduped — false negatives, not false positives.
func whenConditionDuplicateKey(file *scanner.File, cond uint32) string {
	// `null` is emitted as a sole non-named token child of when_condition.
	first := file.FlatFirstChild(cond)
	if first != 0 && file.FlatNextSib(first) == 0 && file.FlatType(first) == "null" {
		return "null"
	}
	var named uint32
	namedCount := 0
	for child := first; child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		namedCount++
		named = child
	}
	if namedCount != 1 {
		return ""
	}
	switch file.FlatType(named) {
	case "integer_literal", "hex_literal", "bin_literal", "long_literal",
		"unsigned_literal":
		return normalizeIntegerLiteral(file.FlatNodeText(named))
	case "real_literal", "boolean_literal", "character_literal",
		"null_literal":
		return file.FlatNodeText(named)
	case "string_literal", "line_string_literal":
		if stringLiteralHasInterpolation(file, named) {
			return ""
		}
		return file.FlatNodeText(named)
	}
	return ""
}

// normalizeIntegerLiteral canonicalizes a Kotlin integer literal so
// equivalent forms (underscore separators, hex/binary prefixes) produce
// the same key. The L/u/uL suffix is preserved as a type marker because
// `1L` and `1` are distinct constants to kotlinc.
func normalizeIntegerLiteral(raw string) string {
	if raw == "" {
		return ""
	}
	s := raw
	suffix := ""
	for len(s) > 0 {
		last := s[len(s)-1]
		if last == 'L' || last == 'u' || last == 'U' {
			suffix = string(last) + suffix
			s = s[:len(s)-1]
			continue
		}
		break
	}
	if strings.Contains(s, "_") {
		s = strings.ReplaceAll(s, "_", "")
	}
	if s == "" {
		return raw
	}
	var (
		val uint64
		err error
	)
	switch {
	case len(s) > 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X'):
		val, err = strconv.ParseUint(s[2:], 16, 64)
	case len(s) > 2 && s[0] == '0' && (s[1] == 'b' || s[1] == 'B'):
		val, err = strconv.ParseUint(s[2:], 2, 64)
	default:
		val, err = strconv.ParseUint(s, 10, 64)
	}
	if err != nil {
		return raw
	}
	return strconv.FormatUint(val, 10) + suffix
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
