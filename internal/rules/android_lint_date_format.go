package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// WeekBasedYearRule mirrors Lint's WeekBasedYear: a date pattern that uses `Y` (week-based year) next to a month or
// day field and without a calendar year field.
type WeekBasedYearRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the callee is matched by name and imports.
func (r *WeekBasedYearRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *WeekBasedYearRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || len(call.Args) == 0 {
		return
	}
	switch {
	case call.Name == "ofPattern" && lintTypeRefMatches(file, call.ReceiverText, "java.time.format.DateTimeFormatter"):
	case call.Name == "SimpleDateFormat" &&
		(lintTypeRefMatches(file, call.TypeText, "java.text.SimpleDateFormat") ||
			lintTypeRefMatches(file, call.TypeText, "android.icu.text.SimpleDateFormat")):
	default:
		return
	}
	arg := call.Args[0]
	pattern, ok := lintEvalString(file, arg)
	if !ok || !strings.Contains(pattern, "Y") {
		return
	}
	start, end, ok := weekBasedYearRun(pattern)
	if !ok {
		return
	}
	run := pattern[start:end]
	msg := "DateFormat character 'Y' in " + run + " is the week-era-year; did you mean 'y'?"
	// Lint points at the run itself only when the pattern is one literal.
	if file.FlatType(arg) == "string_literal" && !strings.Contains(file.FlatNodeText(arg), "$") {
		offset := int(file.FlatStartByte(arg)) + lintLiteralContentOffset(file, arg) + start
		// Lint's range for a Java literal starts at the opening quote, one column before the run.
		if file.Language == scanner.LangJava {
			offset--
		}
		ctx.Emit(lintFindingAtOffset(file, offset, offset+len(run), "", msg))
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, arg, "", msg))
}

// weekBasedYearRun finds the first unquoted run of `Y` in a pattern that has a month or day field and no `y` or `u`.
func weekBasedYearRun(pattern string) (start, end int, ok bool) {
	inQuote, hasMonthOrDay, hasYear := false, false, false
	first := -1
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\'':
			inQuote = !inQuote
		case 'M', 'L', 'd':
			hasMonthOrDay = true
		case 'y', 'u':
			hasYear = true
		case 'Y':
			if !inQuote && first == -1 {
				first = i
			}
		}
	}
	if first == -1 || !hasMonthOrDay || hasYear {
		return 0, 0, false
	}
	end = first + 1
	for end < len(pattern) && pattern[end] == 'Y' {
		end++
	}
	return first, end, true
}
