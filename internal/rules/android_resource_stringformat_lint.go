package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
)

// This file mirrors the XML half of Lint's StringFormatDetector: how a <string>
// is classified as a format string, how arguments are numbered, and the
// StringFormatInvalid, StringFormatCount and StringFormatMatches checks that compare a
// default string against each qualified variant of the same name.

// lintFormatRegexp is StringFormatDetector.FORMAT.
var lintFormatRegexp = regexp.MustCompile(`%(\d+\$)?([-+#, 0(<]*)?(\d+)?(\.\d+)?([tT])?([a-zA-Z%])`)

type lintFormatKind int

const (
	lintNotFormatted lintFormatKind = iota
	lintFormatted
	lintIgnored
)

// lintFormatArg is one format argument found by lintFormatArgs.
type lintFormatArg struct {
	index int
	conv  string
	text  string
}

// lintConversionClass is StringFormatDetector.getConversionClass.
func lintConversionClass(c byte) int {
	switch c {
	case 's', 'S':
		return 1
	case 'c', 'C':
		return 2
	case 'd', 'o', 'x', 'X':
		return 3
	case 'a', 'A', 'e', 'E', 'f', 'g', 'G':
		return 4
	case 'b', 'B':
		return 5
	case 'h', 'H':
		return 6
	case '%':
		return 7
	case 'n':
		return 8
	case 't', 'T':
		return 9
	}
	return 0
}

// lintIncompatible is StringFormatDetector.isIncompatible.
func lintIncompatible(a, b byte) bool {
	ca, cb := lintConversionClass(a), lintConversionClass(b)
	return ca != cb && ca != 0 && cb != 0
}

// lintStripQuotes is StringFormatDetector.stripQuotes: it drops unescaped double
// quotes and keeps unescaped single quotes only inside double quotes.
func lintStripQuotes(s string) string {
	var b strings.Builder
	escaped, quoted := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			b.WriteByte(c)
			escaped = false
			continue
		}
		escaped = c == '\\'
		switch c {
		case '"':
			quoted = !quoted
		case '\'':
			if quoted {
				b.WriteByte(c)
			}
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// lintClassifyFormatString is StringFormatDetector.checkTextNode without the
// PluralsCandidate side check.
func lintClassifyFormatString(text, formattedAttr string) lintFormatKind {
	found := false
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '\\' {
			i++
		}
		if c != '%' {
			continue
		}
		if formattedAttr != "" && !strings.EqualFold(formattedAttr, "true") {
			return lintNotFormatted
		}
		loc := lintFormatRegexp.FindStringSubmatchIndex(text[i:])
		if loc == nil {
			return lintNotFormatted
		}
		conv := text[i+loc[12] : i+loc[13]]
		if lintConversionClass(conv[0]) == 0 || loc[10] >= 0 {
			return lintIgnored
		}
		found = true
		i++
	}
	if found {
		return lintFormatted
	}
	trimmed := strings.TrimLeft(text, " \t\r\n\f\v")
	if trimmed != "" && (trimmed[0] == '@' || trimmed[0] == '?') {
		return lintIgnored
	}
	return lintNotFormatted
}

// lintFormatArgs is StringFormatDetector.getFormatArgumentSequenceWithIndex. A
// numbered argument moves the running counter to the number after it.
func lintFormatArgs(s string) []lintFormatArg {
	var out []lintFormatArg
	counter := 1
	pos := 0
	for pos <= len(s) {
		loc := lintFormatRegexp.FindStringSubmatchIndex(s[pos:])
		if loc == nil {
			break
		}
		start, end := pos+loc[0], pos+loc[1]
		conv := s[pos+loc[12] : pos+loc[13]]
		if conv == "%" || conv == "n" {
			pos = end
			continue
		}
		if start > 0 && s[start-1] == '\\' {
			pos = start + 1
			continue
		}
		var index int
		if loc[2] >= 0 {
			index, _ = strconv.Atoi(s[pos+loc[2] : pos+loc[3]-1])
			counter = index + 1
		} else {
			index = counter
			counter++
		}
		out = append(out, lintFormatArg{index: index, conv: conv, text: s[start:end]})
		pos = end
	}
	return out
}

// lintFormatArgCount is StringFormatDetector.getFormatArgumentCount.
func lintFormatArgCount(s string, seen map[int]bool) int {
	maxIndex := 0
	for _, a := range lintFormatArgs(s) {
		if seen != nil {
			seen[a.index] = true
		}
		if a.index > maxIndex {
			maxIndex = a.index
		}
	}
	return maxIndex
}

type lintFormatEntry struct {
	def  android.StringDefinition
	text string
}

type lintFormatFinding struct {
	issue string
	path  string
	line  int
	msg   string
}

// lintStringDefinitions returns every <string> definition. Indexes built without
// StringDefinitions fall back to the default and translation maps.
func lintStringDefinitions(idx *android.ResourceIndex) []android.StringDefinition {
	if len(idx.StringDefinitions) > 0 {
		return idx.StringDefinitions
	}
	var out []android.StringDefinition
	for _, d := range stringDefinitions(idx) {
		formatted := ""
		if idx.StringsNonFormatted[d.name] {
			formatted = "false"
		}
		out = append(out, android.StringDefinition{
			Name: d.name, FilePath: d.filePath, Line: d.line, Text: d.value,
			Formatted: formatted, DefaultFolder: !d.translation,
		})
	}
	return out
}

func lintFileNameWithParent(path string) string {
	return filepath.Base(filepath.Dir(filepath.FromSlash(path))) + "/" + filepath.Base(filepath.FromSlash(path))
}

// analyzeStringFormats runs the Invalid, Count and Matches checks of Lint's
// StringFormatDetector over the string resources of idx.
func analyzeStringFormats(idx *android.ResourceIndex) []lintFormatFinding {
	byName := map[string][]android.StringDefinition{}
	var names []string
	for _, d := range lintStringDefinitions(idx) {
		if _, ok := byName[d.Name]; !ok {
			names = append(names, d.Name)
		}
		byName[d.Name] = append(byName[d.Name], d)
	}
	sort.Strings(names)
	var findings []lintFormatFinding
	seen := map[lintFormatFinding]bool{}
	add := func(f lintFormatFinding) {
		if !seen[f] {
			seen[f] = true
			findings = append(findings, f)
		}
	}
	for _, name := range names {
		defs := byName[name]
		sort.SliceStable(defs, func(i, j int) bool {
			if defs[i].FilePath != defs[j].FilePath {
				return defs[i].FilePath < defs[j].FilePath
			}
			return defs[i].Line < defs[j].Line
		})
		var base *lintFormatEntry
		for _, d := range defs {
			if !d.DefaultFolder {
				continue
			}
			kind := lintClassifyFormatString(lintStripQuotes(d.Text), d.Formatted)
			if kind == lintIgnored {
				base = nil
				continue
			}
			e := &lintFormatEntry{def: d}
			if kind == lintFormatted {
				e.text = d.Text
			}
			base = e
			analyzeStringFormatList(name, []lintFormatEntry{*e}, add)
		}
		for _, d := range defs {
			if d.DefaultFolder {
				continue
			}
			kind := lintClassifyFormatString(lintStripQuotes(d.Text), d.Formatted)
			if kind == lintIgnored {
				continue
			}
			e := lintFormatEntry{def: d}
			if kind == lintFormatted {
				e.text = d.Text
			}
			list := []lintFormatEntry{e}
			if base != nil {
				if f, handled := tokenStylePair(name, *base, e); handled {
					if f != nil {
						add(*f)
					}
					continue
				}
				list = []lintFormatEntry{*base, e}
			}
			analyzeStringFormatList(name, list, add)
		}
	}
	return findings
}

// tokenStylePair handles a default and a variant that use different placeholder styles,
// where one side only has `^N` tokens. Lint has no such case; some localization pipelines write
// `^N` in one file and convert to `%N$s` per locale. It returns handled when the pair is of that
// kind, and a finding only when the argument counts still differ.
func tokenStylePair(name string, base, variant lintFormatEntry) (finding *lintFormatFinding, handled bool) {
	baseArgs, variantArgs := lintFormatArgCount(base.def.Text, nil), lintFormatArgCount(variant.def.Text, nil)
	var baseCount, variantCount int
	switch {
	case baseArgs == 0 && variantArgs > 0 && hasTemplateToken(base.def.Text):
		baseCount, variantCount = templateTokenCount(base.def.Text), variantArgs
	case variantArgs == 0 && baseArgs > 0 && hasTemplateToken(variant.def.Text):
		baseCount, variantCount = baseArgs, templateTokenCount(variant.def.Text)
	default:
		return nil, false
	}
	if baseCount == variantCount {
		return nil, true
	}
	return &lintFormatFinding{
		issue: "StringFormatCount", path: variant.def.FilePath, line: variant.def.Line,
		msg: fmt.Sprintf("Inconsistent number of arguments in formatting string `%s`; found both %d here and %d in %s. One side uses `^N` tokens and the other `%%` specifiers.",
			name, variantCount, baseCount, lintFileNameWithParent(base.def.FilePath)),
	}, true
}

// analyzeStringFormatList is checkArity plus checkTypes over one default and variant pair.
func analyzeStringFormatList(name string, list []lintFormatEntry, add func(lintFormatFinding)) {
	prevCount := -1
countLoop:
	for _, e := range list {
		seenIdx := map[int]bool{}
		count := lintFormatArgCount(e.text, seenIdx)
		if prevCount != -1 && prevCount != count {
			add(lintFormatFinding{
				issue: "StringFormatCount", path: e.def.FilePath, line: e.def.Line,
				msg: fmt.Sprintf("Inconsistent number of arguments in formatting string `%s`; found both %d here and %d in %s",
					name, count, prevCount, lintFileNameWithParent(list[0].def.FilePath)),
			})
			break countLoop
		}
		var missing []string
		for i := 1; i < count; i++ {
			if !seenIdx[i] {
				missing = append(missing, strconv.Itoa(i))
			}
		}
		if len(missing) > 0 {
			add(lintFormatFinding{
				issue: "StringFormatCount", path: e.def.FilePath, line: e.def.Line,
				msg: fmt.Sprintf("Formatting string '`%s`' is not referencing numbered arguments [%s]", name, strings.Join(missing, ", ")),
			})
		}
		prevCount = count
	}

	types := map[int]string{}
	typeFiles := map[int]string{}
	for _, e := range list {
		for _, a := range lintFormatArgs(e.text) {
			if len(a.text) > 2 && a.text[len(a.text)-2] == ' ' {
				switch a.text[len(a.text)-1] {
				case 'd', 'o', 'x', 'X':
				default:
					add(lintFormatFinding{
						issue: "StringFormatInvalid", path: e.def.FilePath, line: e.def.Line,
						msg: fmt.Sprintf("Incorrect formatting string `%s`; missing conversion character in '`%s`'?", name, a.text),
					})
					continue
				}
			}
			prev, ok := types[a.index]
			if !ok {
				types[a.index] = a.conv
				typeFiles[a.index] = e.def.FilePath
				continue
			}
			if prev != a.conv && lintIncompatible(prev[0], a.conv[0]) {
				add(lintFormatFinding{
					issue: "StringFormatMatches", path: e.def.FilePath, line: e.def.Line,
					msg: fmt.Sprintf("Inconsistent formatting types for argument #%d in format string `%s` ('%s'): Found both '`%s`' here and '`%s`' in %s",
						a.index, name, a.text, a.conv, prev, lintFileNameWithParent(typeFiles[a.index])),
				})
			}
		}
	}
}
