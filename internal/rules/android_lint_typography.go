package rules

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ValuesTypographyResourceBase marks rules that read the text nodes of translatable strings.
type ValuesTypographyResourceBase struct{ ResourceBase }

// AndroidDependencies declares the typography text scan.
func (ValuesTypographyResourceBase) AndroidDependencies() AndroidDataDependency {
	return AndroidDepValuesTypography
}

// TypographyDashesRule mirrors Lint's TypographyDashes: hyphens that should be en or em dashes.
type TypographyDashesRule struct {
	ValuesTypographyResourceBase
	AndroidRule
}

// TypographyEllipsisRule mirrors Lint's TypographyEllipsis: "..." that should be an ellipsis character.
type TypographyEllipsisRule struct {
	ValuesTypographyResourceBase
	AndroidRule
}

// TypographyFractionsRule mirrors Lint's TypographyFractions: "1/2" style fractions.
type TypographyFractionsRule struct {
	ValuesTypographyResourceBase
	AndroidRule
}

// TypographyOtherRule mirrors Lint's TypographyOther: "(c)" that should be the copyright symbol.
type TypographyOtherRule struct {
	ValuesTypographyResourceBase
	AndroidRule
}

// TypographyQuotesRule mirrors Lint's TypographyQuotes: straight quotes and apostrophes.
type TypographyQuotesRule struct {
	ValuesTypographyResourceBase
	AndroidRule
}

// Confidence reports a high base confidence: the checks read the literal string text.
func (r *TypographyDashesRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read the literal string text.
func (r *TypographyEllipsisRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read the literal string text.
func (r *TypographyFractionsRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read the literal string text.
func (r *TypographyOtherRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read the literal string text.
func (r *TypographyQuotesRule) Confidence() float64 { return api.ConfidenceHigh }

var (
	typographyHyphenRange = regexp.MustCompile(`^.*(\d+\s*)-(\s*\d+).*$`)
	typographyGrave       = regexp.MustCompile("^(?:(?:[^`]*`[^'`]+'[^']*)|(?:[^`]*``[^'`]+''[^']*))$")
	typographyFraction    = regexp.MustCompile(`^.*\b([13])\s*/\s*([234])\b.*$`)
	// Lint compiles this with UNICODE_CHARACTER_CLASS, so \W is anything but a Unicode letter, digit, or underscore.
	typographySingleQuote = regexp.MustCompile(`^.*[^\p{L}\p{N}_]*'[^']+'(?:[^\p{L}\p{N}_].*)?$`)
)

// typographyHit is one finding: a message and the 1-based position it is reported at.
type typographyHit struct {
	message   string
	line, col int
}

func typographyTrimStart(s string) string { return strings.TrimLeftFunc(s, unicode.IsSpace) }

func typographyWhole(t android.TypographyText, msg string) []typographyHit {
	return []typographyHit{{msg, t.Line, t.Col}}
}

func typographyEllipsis(t android.TypographyText) []typographyHit {
	trimmed := typographyTrimStart(t.Text)
	if i := strings.Index(trimmed, "..."); i != -1 && !strings.HasPrefix(trimmed[i+3:], ".") {
		return typographyWhole(t, `Replace "..." with ellipsis character (…, &#8230;) ?`)
	}
	return nil
}

func typographyDashes(t android.TypographyText) []typographyHit {
	trimmed := typographyTrimStart(t.Text)
	var hits []typographyHit
	if strings.Contains(trimmed, "-") {
		if m := typographyHyphenRange.FindStringSubmatch(trimmed); m != nil {
			g1, g2 := m[1], m[2]
			first, _ := utf8.DecodeRuneInString(g2)
			last, _ := utf8.DecodeLastRuneInString(g1)
			negative := !unicode.IsSpace(first) && unicode.IsSpace(last)
			if !negative && t.Name != "ga_trackingId" {
				hits = append(hits, typographyWhole(t, `Replace "-" with an "en dash" character (–, &#8211;) ?`)...)
			}
		}
	}
	if i := strings.Index(trimmed, "--"); i > 1 && !strings.HasPrefix(trimmed[i+2:], "-") {
		hits = append(hits, typographyWhole(t, `Replace "--" with an "em dash" character (—, &#8212;) ?`)...)
	}
	return hits
}

func typographyFractions(t android.TypographyText) []typographyHit {
	trimmed := typographyTrimStart(t.Text)
	if !strings.Contains(trimmed, "/") {
		return nil
	}
	m := typographyFraction.FindStringSubmatch(trimmed)
	if m == nil {
		return nil
	}
	fractions := map[string]struct {
		char   rune
		entity string
	}{
		"1/2": {'½', "&#189;"}, "1/4": {'¼', "&#188;"}, "3/4": {'¾', "&#190;"}, "1/3": {'⅓', "&#8531;"}, "2/3": {'⅔', "&#8532;"},
	}
	key := m[1] + "/" + m[2]
	f, ok := fractions[key]
	if !ok {
		return nil
	}
	return typographyWhole(t, fmt.Sprintf("Use fraction character %c (%s) instead of %s?", f.char, f.entity, key))
}

func typographyOther(t android.TypographyText) []typographyHit {
	trimmed := typographyTrimStart(t.Text)
	if strings.Contains(trimmed, "(") && (strings.Contains(trimmed, "(c)") || strings.Contains(trimmed, "(C)")) {
		return typographyWhole(t, "Replace (c) with copyright symbol © (&#169;) ?")
	}
	return nil
}

// typographyQuotes follows Lint's quote checks in order, stopping at the first finding.
func typographyQuotes(t android.TypographyText) []typographyHit {
	trimmed := typographyTrimStart(t.Text)
	if quoteStart := strings.IndexByte(trimmed, '\''); quoteStart != -1 {
		quoteEnd := -1
		if i := strings.IndexByte(trimmed[quoteStart+1:], '\''); i != -1 {
			quoteEnd = quoteStart + 1 + i
		}
		if quoteEnd != -1 && quoteEnd > quoteStart+1 && (quoteEnd < len(trimmed)-1 || quoteStart > 0) && typographySingleQuote.MatchString(t.Text) {
			return typographyWhole(t, "Replace straight quotes ('') with directional quotes (‘’, &#8216; and &#8217;) ?")
		}
		if quoteEnd != quoteStart+1 && quoteStart > 0 && typographyApostropheContext(trimmed, quoteStart) {
			return typographyApostrophe(t, trimmed, quoteStart)
		}
	}
	if quoteStart := strings.IndexByte(trimmed, '"'); quoteStart != -1 {
		if i := strings.IndexByte(trimmed[quoteStart+1:], '"'); i != -1 {
			quoteEnd := quoteStart + 1 + i
			if quoteEnd > quoteStart+1 && (quoteEnd < len(trimmed)-1 || quoteStart > 0) {
				return typographyWhole(t, `Replace straight quotes (") with directional quotes (“”, &#8220; and &#8221;) ?`)
			}
		}
	}
	if strings.IndexByte(trimmed, '`') != -1 && typographyGrave.MatchString(trimmed) {
		return typographyWhole(t, "Avoid quoting with grave accents; use apostrophes or better yet directional quotes instead")
	}
	return nil
}

func typographyApostropheContext(trimmed string, quoteStart int) bool {
	prev, _ := utf8.DecodeLastRuneInString(trimmed[:quoteStart])
	if unicode.IsLetter(prev) || unicode.IsDigit(prev) {
		return true
	}
	if quoteStart > 1 && trimmed[quoteStart-1] == '\\' {
		before, _ := utf8.DecodeLastRuneInString(trimmed[:quoteStart-1])
		return unicode.IsLetter(before) || unicode.IsDigit(before)
	}
	return false
}

// typographyApostrophe reports at the apostrophe itself when it is the only one, and at the whole text otherwise.
func typographyApostrophe(t android.TypographyText, trimmed string, quoteStart int) []typographyHit {
	const msg = "Replace apostrophe (') with typographic apostrophe (’, &#8217;) ?"
	if strings.Count(trimmed, "'") > 1 {
		return typographyWhole(t, msg)
	}
	prefix := trimmed[:quoteStart]
	lineBreaks := strings.Count(prefix, "\n")
	col := t.Col + utf8.RuneCountInString(prefix)
	if lineBreaks > 0 {
		col = utf8.RuneCountInString(prefix[strings.LastIndexByte(prefix, '\n')+1:]) + 1
	}
	return []typographyHit{{msg, t.Line + lineBreaks, col}}
}

func typographyEmit(ctx *api.Context, check func(android.TypographyText) []typographyHit) {
	for _, text := range ctx.ResourceIndex.TypographyTexts {
		for _, hit := range check(text) {
			ctx.Emit(scanner.Finding{File: text.FilePath, Line: hit.line, Col: hit.col, Message: hit.message})
		}
	}
}

func (r *TypographyDashesRule) check(ctx *api.Context)    { typographyEmit(ctx, typographyDashes) }
func (r *TypographyEllipsisRule) check(ctx *api.Context)  { typographyEmit(ctx, typographyEllipsis) }
func (r *TypographyFractionsRule) check(ctx *api.Context) { typographyEmit(ctx, typographyFractions) }
func (r *TypographyOtherRule) check(ctx *api.Context)     { typographyEmit(ctx, typographyOther) }
func (r *TypographyQuotesRule) check(ctx *api.Context)    { typographyEmit(ctx, typographyQuotes) }
