package rules

import (
	"strconv"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// TextDirectionLiteralInStringRule flags string literals that embed
// Unicode BIDI control characters (LRM, RLM, LRE/RLE/PDF/LRO/RLO,
// LRI/RLI/FSI/PDI). Hard-coding these characters bakes a particular
// directional override into the data and tends to break in mixed-locale
// rendering. Use BidiFormatter (or platform equivalents) to wrap text
// with the right embedding for the runtime locale.
type TextDirectionLiteralInStringRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-3 (high) base confidence. The BIDI control
// codepoints have no other meaning in a string literal, so a literal
// match is unambiguous.
func (r *TextDirectionLiteralInStringRule) Confidence() float64 { return 0.9 }

func (r *TextDirectionLiteralInStringRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	if file.FlatType(idx) != "string_literal" {
		return
	}
	if !stringLiteralHasBidiControl(file, idx) {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"String literal embeds a Unicode BIDI control character. Wrap user-visible text with BidiFormatter (or equivalent) instead of hard-coding direction overrides.")
}

// stringLiteralHasBidiControl reports whether any string_content child
// of a string_literal node contains a BIDI control character, either as
// a raw codepoint or as a Kotlin `\uXXXX` escape. Interpolated segments
// are skipped — only fragments of literal text are scanned.
func stringLiteralHasBidiControl(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "string_content", "string_fragment":
			text := file.FlatNodeText(child)
			if containsBidiControl(text) || containsBidiEscape(text) {
				return true
			}
		case "character_escape_seq":
			if containsBidiEscape(file.FlatNodeText(child)) {
				return true
			}
		}
	}
	return false
}

// containsBidiControl reports whether s contains a BIDI control rune.
func containsBidiControl(s string) bool {
	for _, r := range s {
		if isBidiControlRune(r) {
			return true
		}
	}
	return false
}

func isBidiControlRune(r rune) bool {
	switch {
	case r == 0x200E, r == 0x200F:
		return true
	case r >= 0x202A && r <= 0x202E:
		return true
	case r >= 0x2066 && r <= 0x2069:
		return true
	}
	return false
}

// containsBidiEscape reports whether s contains a Kotlin/Java `\uXXXX`
// escape sequence whose codepoint is a BIDI control character. An odd
// number of leading backslashes means the `u` is unescaped; an even
// number means the backslash itself is escaped and the sequence is
// literal text.
func containsBidiEscape(s string) bool {
	i := 0
	for {
		j := strings.Index(s[i:], `\u`)
		if j < 0 || i+j+6 > len(s) {
			return false
		}
		j += i
		bs := 0
		for k := j; k >= 0 && s[k] == '\\'; k-- {
			bs++
		}
		if bs%2 == 1 {
			if r, err := strconv.ParseUint(s[j+2:j+6], 16, 32); err == nil && isBidiControlRune(rune(r)) {
				return true
			}
		}
		i = j + 2
	}
}
