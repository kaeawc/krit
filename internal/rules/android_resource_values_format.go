package rules

import (
	"fmt"
	"strconv"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// StringFormatInvalidResource
// ---------------------------------------------------------------------------

// StringFormatInvalidResourceRule detects invalid format specifiers in string
// resources. Checks for bare `%` at end of string, `%` followed by invalid
// conversion characters, etc. Valid: %s, %d, %f, %1$s, %%, %n, %x, %o, %e, %g.
type StringFormatInvalidResourceRule struct {
	ValuesStringsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *StringFormatInvalidResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StringFormatInvalidResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for name, val := range idx.Strings {
		// Skip strings with formatted="false". These are NOT format
		// strings — the `%{var}` / `%word` sequences are literal text
		// (typically for gettext/phrase-style translation placeholders).
		if idx.StringsNonFormatted != nil && idx.StringsNonFormatted[name] {
			continue
		}
		msg := checkInvalidFormatString(val)
		if msg == "" {
			continue
		}
		// Use the real file path and line from the resource index. The
		// previous hardcoded `res/values/strings.xml` at line 0 both
		// mislocated findings (emitting a CWD-relative path that could
		// even reference other repos if krit was run from a parent dir)
		// and collapsed multiple findings into one key.
		filePath := "res/values/strings.xml"
		line := 0
		if loc, ok := idx.StringsLocation[name]; ok {
			if loc.FilePath != "" {
				filePath = loc.FilePath
			}
			if loc.Line > 0 {
				line = loc.Line
			}
		}
		ctx.Emit(baseFinding(filePath, line, r.BaseRule,
			fmt.Sprintf("String `%s` has invalid format specifier: %s", name, msg)))
	}
}

// checkInvalidFormatString checks a string value for invalid % sequences.
// Returns an error description or "" if valid.
func checkInvalidFormatString(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+1 >= len(s) {
			return "bare `%` at end of string"
		}
		next := s[i+1]
		// %% is a literal percent
		if next == '%' {
			i++
			continue
		}
		// Skip optional argument index (digits followed by $)
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j < len(s) && s[j] == '$' {
			j++
		}
		// Skip flags
		for j < len(s) && (s[j] == '-' || s[j] == '+' || s[j] == ' ' || s[j] == '0' || s[j] == '#') {
			j++
		}
		// Skip width
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		// Skip precision
		if j < len(s) && s[j] == '.' {
			j++
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
		}
		if j >= len(s) {
			return "bare `%` at end of string (incomplete format)"
		}
		c := s[j]
		if !validConversions[c] {
			return fmt.Sprintf("invalid conversion character `%c` after `%%`", c)
		}
		i = j
	}
	return ""
}

// ---------------------------------------------------------------------------
// StringFormatCountResource
// ---------------------------------------------------------------------------

// StringFormatCountResourceRule detects format strings where positional argument
// indices are inconsistent. For example, `%1$s and %3$s` is suspicious because
// %2$ is missing.
type StringFormatCountResourceRule struct {
	ValuesStringsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *StringFormatCountResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StringFormatCountResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for name, val := range idx.Strings {
		if msg := checkFormatArgCount(val); msg != "" {
			ctx.Emit(baseFinding("res/values/strings.xml", 0, r.BaseRule,
				fmt.Sprintf("String `%s`: %s", name, msg)))
		}
	}
}

// checkFormatArgCount checks for gaps in positional argument indices.
func checkFormatArgCount(s string) string {
	indices := make(map[int]bool)
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+1 >= len(s) {
			continue
		}
		if s[i+1] == '%' {
			i++
			continue
		}
		// Look for digits followed by $
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j > i+1 && j < len(s) && s[j] == '$' {
			idx, _ := strconv.Atoi(s[i+1 : j])
			if idx > 0 {
				indices[idx] = true
			}
			i = j
		}
	}
	if len(indices) < 2 {
		return ""
	}
	// Find max index and check for gaps
	maxIdx := 0
	for idx := range indices {
		if idx > maxIdx {
			maxIdx = idx
		}
	}
	var missing []int
	for i := 1; i <= maxIdx; i++ {
		if !indices[i] {
			missing = append(missing, i)
		}
	}
	if len(missing) > 0 {
		parts := make([]string, len(missing))
		for i, m := range missing {
			parts[i] = fmt.Sprintf("%%%d$", m)
		}
		return fmt.Sprintf("positional arguments have gap: missing %s (max is %%%d$)",
			strings.Join(parts, ", "), maxIdx)
	}
	return ""
}

// ---------------------------------------------------------------------------
// StringFormatMatchesResource
// ---------------------------------------------------------------------------

// StringFormatMatchesResourceRule detects format specifier type mismatches across
// plural quantities. If a plural has `%d` in "one" but `%s` in "other", the
// types are inconsistent and will likely cause a runtime crash.
type StringFormatMatchesResourceRule struct {
	ValuesPluralsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *StringFormatMatchesResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StringFormatMatchesResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for name, quantities := range idx.Plurals {
		if len(quantities) < 2 {
			continue
		}
		// Extract format specifier types for each quantity
		typesByQty := make(map[string][]byte)
		for qty, val := range quantities {
			types := extractFormatTypes(val)
			if len(types) > 0 {
				typesByQty[qty] = types
			}
		}
		if len(typesByQty) < 2 {
			continue
		}
		// Compare all quantities against the first one found
		var refQty string
		var refTypes []byte
		for qty, types := range typesByQty {
			refQty = qty
			refTypes = types
			break
		}
		for qty, types := range typesByQty {
			if qty == refQty {
				continue
			}
			if len(types) != len(refTypes) {
				ctx.Emit(baseFinding("res/values/strings.xml", 0, r.BaseRule,
					fmt.Sprintf("Plural `%s`: quantity `%s` has %d format args but `%s` has %d",
						name, qty, len(types), refQty, len(refTypes))))
				continue
			}
			for i := range types {
				if types[i] != refTypes[i] {
					ctx.Emit(baseFinding("res/values/strings.xml", 0, r.BaseRule,
						fmt.Sprintf("Plural `%s`: format type mismatch at arg %d — `%s` uses `%%%c` but `%s` uses `%%%c`",
							name, i+1, qty, types[i], refQty, refTypes[i])))
					break
				}
			}
		}
	}
}

// extractFormatTypes returns the conversion characters from format specifiers.
func extractFormatTypes(s string) []byte {
	var types []byte
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+1 >= len(s) {
			continue
		}
		if s[i+1] == '%' {
			i++
			continue
		}
		j := i + 1
		// Skip argument index
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j < len(s) && s[j] == '$' {
			j++
		}
		// Skip flags
		for j < len(s) && (s[j] == '-' || s[j] == '+' || s[j] == ' ' || s[j] == '0' || s[j] == '#') {
			j++
		}
		// Skip width
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		// Skip precision
		if j < len(s) && s[j] == '.' {
			j++
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
		}
		if j < len(s) && validConversions[s[j]] {
			types = append(types, s[j])
			i = j
		}
	}
	return types
}

// ---------------------------------------------------------------------------
// StringFormatTrivialResource
// ---------------------------------------------------------------------------

// StringFormatTrivialResourceRule detects string resources whose whole value is
// a single string placeholder.
type StringFormatTrivialResourceRule struct {
	ValuesStringsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *StringFormatTrivialResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StringFormatTrivialResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for name, val := range idx.Strings {
		if isTrivialSingleStringFormatResource(val) {
			filePath := "res/values/strings.xml"
			line := 0
			if loc, ok := idx.StringsLocation[name]; ok {
				if loc.FilePath != "" {
					filePath = loc.FilePath
				}
				if loc.Line > 0 {
					line = loc.Line
				}
			}
			ctx.Emit(baseFinding(filePath, line, r.BaseRule,
				fmt.Sprintf("String `%s` is only a string format placeholder. "+
					"Use the source string directly instead of formatting this resource.", name)))
		}
	}
}

func isTrivialSingleStringFormatResource(val string) bool {
	trimmed := strings.TrimSpace(val)
	if countFormatSpecifiers(trimmed) != 1 {
		return false
	}
	if j, ok := formatSpecifierEnd(trimmed, 1); ok && j == len(trimmed)-1 {
		return trimmed[j] == 's'
	}
	return false
}

// countFormatSpecifiers counts the number of format specifiers in a string.
// It recognizes patterns like %s, %d, %f, %1$s, %2$d, etc.
func countFormatSpecifiers(s string) int {
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+1 >= len(s) {
			continue
		}
		if s[i+1] == '%' {
			i++
			continue
		}
		if j, ok := formatSpecifierEnd(s, i+1); ok {
			count++
			i = j
		}
	}
	return count
}

func formatSpecifierEnd(s string, j int) (int, bool) {
	j = skipArgIndex(s, j)
	j = skipFlagsAndWidth(s, j)
	j = skipPrecision(s, j)
	if j < len(s) && isFormatConversionChar(s[j]) {
		return j, true
	}
	return j, false
}

func skipArgIndex(s string, j int) int {
	start := j
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if j < len(s) && s[j] == '$' {
		return j + 1
	}
	return start
}

func skipFlagsAndWidth(s string, j int) int {
	for j < len(s) && (s[j] == '-' || s[j] == '+' || s[j] == ' ' || s[j] == '0' || s[j] == '#') {
		j++
	}
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	return j
}

func skipPrecision(s string, j int) int {
	if j < len(s) && s[j] == '.' {
		j++
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
	}
	return j
}

func isFormatConversionChar(c byte) bool {
	switch c {
	case 's', 'd', 'f', 'x', 'o', 'e', 'g', 'S', 'D':
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// StringResourceMissingPositional
// ---------------------------------------------------------------------------

// StringResourceMissingPositionalRule detects translatable <string> resources
// containing two or more non-positional format specifiers (`%s`, `%d`, ...)
// without any positional indices. Languages that reorder arguments need
// `%1$s %2$s` form; bare `%s %s` cannot be reordered by translators.
type StringResourceMissingPositionalRule struct {
	ValuesStringsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *StringResourceMissingPositionalRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *StringResourceMissingPositionalRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for name, val := range idx.Strings {
		if idx.StringsNonTranslate[name] {
			continue
		}
		if idx.StringsNonFormatted != nil && idx.StringsNonFormatted[name] {
			continue
		}
		nonPos, pos := classifyFormatSpecifiers(val)
		if nonPos < 2 || pos > 0 {
			continue
		}
		filePath := "res/values/strings.xml"
		line := 0
		if loc, ok := idx.StringsLocation[name]; ok {
			if loc.FilePath != "" {
				filePath = loc.FilePath
			}
			if loc.Line > 0 {
				line = loc.Line
			}
		}
		ctx.Emit(baseFinding(filePath, line, r.BaseRule,
			fmt.Sprintf("String `%s` has multiple non-positional format specifiers. "+
				"Use positional syntax like `%%1$s %%2$s` so translators can reorder arguments.", name)))
	}
}

// classifyFormatSpecifiers counts non-positional and positional format
// specifiers in s. `%%` is ignored. Positional specifiers use the `%N$X`
// form; non-positional specifiers are bare `%s`, `%d`, etc.
func classifyFormatSpecifiers(s string) (nonPositional, positional int) {
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+1 >= len(s) {
			continue
		}
		if s[i+1] == '%' {
			i++
			continue
		}
		j := i + 1
		isPositional := false
		// Optional argument index: digits followed by $
		k := j
		for k < len(s) && s[k] >= '0' && s[k] <= '9' {
			k++
		}
		if k > j && k < len(s) && s[k] == '$' {
			isPositional = true
			j = k + 1
		}
		// Skip flags
		for j < len(s) && (s[j] == '-' || s[j] == '+' || s[j] == ' ' || s[j] == '0' || s[j] == '#') {
			j++
		}
		// Skip width
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		// Skip precision
		if j < len(s) && s[j] == '.' {
			j++
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
		}
		if j >= len(s) {
			continue
		}
		if !validConversions[s[j]] {
			continue
		}
		if isPositional {
			positional++
		} else {
			nonPositional++
		}
		i = j
	}
	return nonPositional, positional
}
