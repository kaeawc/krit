package rules

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// PluralsCandidateResourceRule mirrors the PluralsCandidate check in Lint's
// StringFormatDetector: an English string such as "%d items" that probably
// should be a plurals resource.
type PluralsCandidateResourceRule struct {
	ValuesStringsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *PluralsCandidateResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *PluralsCandidateResourceRule) check(ctx *api.Context) {
	for _, d := range lintStringDefinitions(ctx.ResourceIndex) {
		if !isEnglishValuesFolder(d.FilePath) {
			continue
		}
		if word := lintPluralCandidateWord(lintStripQuotes(d.Text), d.Formatted); word != "" {
			ctx.Emit(baseFinding(d.FilePath, d.Line, r.BaseRule,
				fmt.Sprintf("Formatting %%d followed by words (%q): This should probably be a plural rather than a string", word)))
		}
	}
}

// isEnglishValuesFolder is Lint.isEnglishResource with an unqualified folder counted as English.
func isEnglishValuesFolder(path string) bool {
	folder := filepath.Base(filepath.Dir(filepath.FromSlash(strings.ReplaceAll(path, "\\", "/"))))
	loc, ok := parseFolderLocale(folder)
	return !ok || loc.language == "en"
}

// javaWhitespace is Character.isWhitespace, which excludes non-breaking spaces.
func javaWhitespace(r rune) bool {
	return unicode.IsSpace(r) && r != 0xA0 && r != 0x2007 && r != 0x202F
}

// lintPluralCandidateWord is StringFormatDetector.checkTextNode reduced to its
// PluralsCandidate side check. It returns the word that follows the first integer
// conversion, or "" when Lint reports nothing.
func lintPluralCandidateWord(text, formattedAttr string) string {
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '\\' {
			i++
		}
		if c != '%' {
			continue
		}
		if formattedAttr != "" && !strings.EqualFold(formattedAttr, "true") {
			return ""
		}
		loc := lintFormatRegexp.FindStringSubmatchIndex(text[i:])
		if loc == nil {
			return ""
		}
		conv := text[i+loc[12] : i+loc[13]]
		class := lintConversionClass(conv[0])
		if class == 0 || loc[10] >= 0 {
			return ""
		}
		if class == 3 {
			if word := pluralWordAfter(text, i+loc[1]); word != "" {
				return word
			}
		}
		i++
	}
	return ""
}

// pluralWordAfter is checkPotentialPlural: the first non-space character at or after
// from must start a lowercase word of three or more letters that contains a vowel.
func pluralWordAfter(text string, from int) string {
	pos := from
	for pos < len(text) {
		r, size := utf8.DecodeRuneInString(text[pos:])
		if javaWhitespace(r) {
			pos += size
			continue
		}
		if !unicode.IsLetter(r) {
			return ""
		}
		break
	}
	if pos >= len(text) {
		return ""
	}
	end, letters, vowel := pos, 0, false
	for end < len(text) {
		r, size := utf8.DecodeRuneInString(text[end:])
		if !unicode.IsLetter(r) {
			break
		}
		if letters > 0 && unicode.IsUpper(r) {
			return ""
		}
		vowel = vowel || strings.ContainsRune("aeiouy", r)
		letters++
		end += size
	}
	word := text[pos:end]
	if letters <= 2 || !vowel || word == "min" {
		return ""
	}
	return word
}
