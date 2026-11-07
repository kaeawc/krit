package rules

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// StringFormatInvalidResource
// ---------------------------------------------------------------------------

// StringFormatInvalidResourceRule mirrors Lint's StringFormatInvalid check on string
// resources: a space-flagged conversion such as the `% s` in "100% sure" is probably a
// literal percent sign that was not doubled.
type StringFormatInvalidResourceRule struct {
	ValuesStringsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *StringFormatInvalidResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StringFormatInvalidResourceRule) check(ctx *api.Context) {
	emitStringFormatFindings(ctx, r.BaseRule, "StringFormatInvalid")
}

// emitStringFormatFindings reports the Lint StringFormatDetector findings of one issue id.
func emitStringFormatFindings(ctx *api.Context, rule BaseRule, issue string) {
	for _, f := range analyzeStringFormats(ctx.ResourceIndex) {
		if f.issue == issue {
			ctx.Emit(baseFinding(f.path, f.line, rule, f.msg))
		}
	}
}

// ---------------------------------------------------------------------------
// StringFormatCountResource
// ---------------------------------------------------------------------------

// StringFormatCountResourceRule mirrors Lint's StringFormatCount check: a variant of a
// string whose number of format arguments differs from the default string, and a
// string that skips a numbered argument such as the 2 in `%1$s ... %3$s`.
type StringFormatCountResourceRule struct {
	ValuesStringsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *StringFormatCountResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StringFormatCountResourceRule) check(ctx *api.Context) {
	emitStringFormatFindings(ctx, r.BaseRule, "StringFormatCount")
}

// ---------------------------------------------------------------------------
// StringFormatMatchesResource
// ---------------------------------------------------------------------------

// StringFormatMatchesResourceRule mirrors Lint's StringFormatMatches check on string
// resources: a variant of a string that uses an incompatible conversion for a numbered
// argument. It also compares the conversions of plural quantities, which Lint does not.
type StringFormatMatchesResourceRule struct {
	ValuesPluralsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *StringFormatMatchesResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StringFormatMatchesResourceRule) check(ctx *api.Context) {
	emitStringFormatFindings(ctx, r.BaseRule, "StringFormatMatches")
	// Lint does not compare plural quantities; this is a krit extension.
	idx := ctx.ResourceIndex
	for name, quantities := range idx.Plurals {
		if len(quantities) < 2 {
			continue
		}
		type quantityArgs struct {
			qty           string
			slots         map[int]byte
			allPositional bool
		}
		var withArgs []quantityArgs
		for qty, val := range quantities {
			if slots, allPos := formatArgSlots(val); len(slots) > 0 {
				withArgs = append(withArgs, quantityArgs{qty, slots, allPos})
			}
		}
		sort.Slice(withArgs, func(i, j int) bool { return withArgs[i].qty < withArgs[j].qty })
		if len(withArgs) < 2 {
			continue
		}
		filePath, line, located := pluralResourceLocation(idx, name)
		if !located {
			continue
		}
		ref := withArgs[0]
		for _, cur := range withArgs[1:] {
			// A quantity may drop arguments, so unnumbered specifiers of
			// different lengths cannot be lined up and are skipped.
			if len(cur.slots) != len(ref.slots) && !(cur.allPositional && ref.allPositional) {
				continue
			}
			slotNums := make([]int, 0, len(cur.slots))
			for n := range cur.slots {
				slotNums = append(slotNums, n)
			}
			sort.Ints(slotNums)
			for _, n := range slotNums {
				if want, ok := ref.slots[n]; ok && want != cur.slots[n] {
					ctx.Emit(baseFinding(filePath, line, r.BaseRule,
						fmt.Sprintf("Plural `%s`: format type mismatch at arg %d — `%s` uses `%%%c` but `%s` uses `%%%c`",
							name, n, cur.qty, cur.slots[n], ref.qty, want)))
					break
				}
			}
		}
	}
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
		// Judge only the default-locale definition; translations of a
		// placeholder-only string are expected to match it.
		if idx.StringSources != nil {
			src, ok := idx.StringSources[name]
			if !ok {
				continue
			}
			val = src.Value
		}
		if skipFormatChecks(idx, name) {
			continue
		}
		// Such strings are usually expanded as templates in code (`%1$s` wraps a label), and this rule has no source usage scan.
		if lower := strings.ToLower(name); strings.Contains(lower, "button") || strings.Contains(lower, "label") {
			continue
		}
		// A translator note says the pass-through wrapper is deliberate.
		if idx.StringsWithNote[name] {
			continue
		}
		if isTrivialSingleStringFormatResource(val) {
			filePath, line := stringResourceLocation(idx, name)
			ctx.Emit(baseFinding(filePath, line, r.BaseRule,
				fmt.Sprintf("String `%s` is only a string format placeholder, so it adds no text of its own. "+
					"Pass the argument directly unless the wrapper is needed for translation.", name)))
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
	for j < len(s) && isFormatFlag(s[j]) {
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

// isFormatFlag reports whether c is a java.util.Formatter flag: `-#+ 0,(<`.
func isFormatFlag(c byte) bool {
	switch c {
	case '-', '#', '+', ' ', '0', ',', '(', '<':
		return true
	}
	return false
}

// stringResourceLocation returns the recorded file and line of a default-locale
// string, falling back to the last-seen definition and then a placeholder path.
func stringResourceLocation(idx *android.ResourceIndex, name string) (string, int) {
	loc := idx.StringsLocation[name]
	if src, ok := idx.StringSources[name]; ok && src.FilePath != "" {
		loc = android.StringLocation{FilePath: src.FilePath, Line: src.Line}
	}
	if loc.FilePath == "" {
		return "res/values/strings.xml", 0
	}
	return loc.FilePath, loc.Line
}

// storedStringLocation returns the file and line that supplied the stored value of a string.
func storedStringLocation(idx *android.ResourceIndex, name string) (string, int) {
	if loc := idx.StringsLocation[name]; loc.FilePath != "" {
		return loc.FilePath, loc.Line
	}
	return "res/values/strings.xml", 0
}

// pluralResourceLocation returns the recorded file and line of a plurals entry.
// ok is false when the entry has no recorded position, so callers skip it rather than report a made-up one.
func pluralResourceLocation(idx *android.ResourceIndex, name string) (file string, line int, ok bool) {
	if loc, found := idx.PluralsLocation[name]; found && loc.FilePath != "" && loc.Line > 0 {
		return loc.FilePath, loc.Line, true
	}
	return "", 0, false
}

// skipFormatChecks reports whether a string is exempt from format validation.
// stringDefinition is one <string> definition: the default-locale value or a translation.
type stringDefinition struct {
	name, value, filePath string
	line                  int
	translation           bool
}

// stringDefinitions lists the default-locale value of every string followed by each translation
// whose text differs from the default, in a stable order. A string with no default-locale
// definition is listed from the stored value alone.
func stringDefinitions(idx *android.ResourceIndex) []stringDefinition {
	seen := make(map[string]bool, len(idx.Strings))
	names := make([]string, 0, len(idx.Strings))
	for name := range idx.Strings {
		seen[name] = true
		names = append(names, name)
	}
	for name := range idx.StringSources {
		if !seen[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var out []stringDefinition
	for _, name := range names {
		def, hasDefault := idx.StringSources[name]
		storedPath, storedLine := storedStringLocation(idx, name)
		stored, hasStored := idx.Strings[name]
		if hasDefault {
			out = append(out, stringDefinition{name: name, value: def.Value, filePath: def.FilePath, line: def.Line})
		} else if hasStored {
			out = append(out, stringDefinition{name: name, value: stored, filePath: storedPath, line: storedLine,
				translation: android.IsTranslationValuesPath(storedPath)})
		}
		listed := map[string]bool{}
		for _, tr := range idx.StringTranslations[name] {
			listed[tr.FilePath] = true
			if hasDefault && tr.Value == def.Value {
				continue
			}
			out = append(out, stringDefinition{name: name, value: tr.Value, filePath: tr.FilePath, line: tr.Line, translation: true})
		}
		// An index built without StringTranslations may still keep a translation as the stored value.
		if hasDefault && hasStored && stored != def.Value && android.IsTranslationValuesPath(storedPath) && !listed[storedPath] {
			out = append(out, stringDefinition{name: name, value: stored, filePath: storedPath, line: storedLine, translation: true})
		}
	}
	return out
}

func skipFormatChecks(idx *android.ResourceIndex, name string) bool {
	return idx.StringsNonTranslate[name] || idx.StringsNonFormatted[name]
}

// isTranslationOverride reports whether the stored value for name came from a
// translation file whose text differs from the default-locale value.
func isTranslationOverride(idx *android.ResourceIndex, name, val string) (android.StringSource, bool) {
	src, ok := idx.StringSources[name]
	if !ok || src.Value == val {
		return src, false
	}
	return src, android.IsTranslationValuesPath(idx.StringsLocation[name].FilePath)
}

// formatArgSlots returns the argument types by 1-based slot and whether every
// specifier is positional. `<` reuses the previous slot; `%n` and `%%` take none.
func formatArgSlots(s string) (slots map[int]byte, allPositional bool) {
	slots = map[int]byte{}
	allPositional = true
	next, prev := 1, 0
	for i := 0; i < len(s); i++ {
		if s[i] != '%' || i+1 >= len(s) {
			continue
		}
		if s[i+1] == '%' {
			i++
			continue
		}
		j := i + 1
		k := skipArgIndex(s, j)
		explicit := 0
		if k != j {
			explicit, _ = strconv.Atoi(s[j : k-1])
		}
		flagsStart := k
		k = skipFlagsAndWidth(s, k)
		reuse := strings.Contains(s[flagsStart:k], "<")
		k = skipPrecision(s, k)
		if k >= len(s) || !validConversions[s[k]] {
			continue
		}
		i = k
		if s[k] == 'n' {
			continue
		}
		slot := explicit
		switch {
		case reuse:
			slot = prev
		case explicit == 0:
			slot = next
			next++
			allPositional = false
		}
		if slot > 0 {
			slots[slot] = s[k]
			prev = slot
		}
	}
	return slots, allPositional
}

// templateTokenCount returns the number of distinct `^N` template tokens in s.
func templateTokenCount(s string) int {
	seen := map[byte]bool{}
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '^' && s[i+1] >= '1' && s[i+1] <= '9' {
			seen[s[i+1]] = true
		}
	}
	return len(seen)
}

// hasTemplateToken reports whether s contains a `^1`-style template placeholder.
func hasTemplateToken(s string) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '^' && s[i+1] >= '1' && s[i+1] <= '9' {
			return true
		}
	}
	return false
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
	for _, def := range stringDefinitions(idx) {
		name := def.name
		if idx.StringsNonTranslate[name] || idx.StringsNonFormatted[name] {
			continue
		}
		// Translations that copy the default are skipped by stringDefinitions; a translation with its own text is judged on its own entry.
		nonPos, pos := classifyFormatSpecifiers(def.value)
		if nonPos < 2 || pos > 0 {
			continue
		}
		ctx.Emit(baseFinding(def.filePath, def.line, r.BaseRule,
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
		for j < len(s) && isFormatFlag(s[j]) {
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
