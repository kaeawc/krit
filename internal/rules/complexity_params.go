package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// longParamEntry is one parameter of a function or constructor as detekt's LongParameterList sees it.
type longParamEntry struct {
	node       uint32
	text       string // "name: Type"
	hasDefault bool
	annotated  bool
}

// longParamAnnotationNames returns the short names of the annotations in a modifiers or parameter_modifiers
// node.
func longParamAnnotationNames(file *scanner.File, mods uint32) []string {
	var names []string
	var walk func(n uint32)
	walk = func(n uint32) {
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "annotation" {
				text := strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(c)), "@")
				if i := strings.IndexAny(text, "( \t\n<"); i >= 0 {
					text = text[:i]
				}
				text = strings.TrimPrefix(text, "param:")
				text = strings.TrimPrefix(text, "field:")
				text = strings.TrimPrefix(text, "get:")
				text = strings.TrimPrefix(text, "set:")
				text = strings.TrimPrefix(text, "property:")
				names = append(names, text)
				continue
			}
			walk(c)
		}
	}
	walk(mods)
	return names
}

// importedFqName resolves a simple annotation name through the file's imports, or returns the name.
func importedFqName(file *scanner.File, name string) string {
	if strings.Contains(name, ".") {
		return name
	}
	for _, line := range file.Lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "import ") && strings.HasSuffix(strings.Fields(line)[1], "."+name) {
			return strings.Fields(line)[1]
		}
	}
	return name
}

// compileAnnotationPatterns turns detekt's ignoreAnnotatedParameter globs into regexes: `.` is literal and `*`
// matches anything.
func compileAnnotationPatterns(patterns []string) []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		p = strings.ReplaceAll(p, ".", `\.`)
		p = strings.ReplaceAll(p, "*", ".*")
		if re, err := regexp.Compile("^(?:" + p + ")$"); err == nil {
			out = append(out, re)
		}
	}
	return out
}

func annotatedParamMatches(file *scanner.File, names []string, res []*regexp.Regexp) bool {
	for _, n := range names {
		fq := importedFqName(file, n)
		for _, re := range res {
			if re.MatchString(n) || re.MatchString(fq) {
				return true
			}
		}
	}
	return false
}

// paramTypeText returns the declared type text that follows the colon of a parameter node.
func paramTypeText(file *scanner.File, param uint32) string {
	sawColon := false
	for c := file.FlatFirstChild(param); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == ":" {
			sawColon = true
			continue
		}
		if sawColon && file.FlatIsNamed(c) {
			return file.FlatNodeText(c)
		}
	}
	return ""
}

func paramName(file *scanner.File, param uint32) string {
	for c := file.FlatFirstChild(param); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "simple_identifier" {
			return file.FlatNodeText(c)
		}
	}
	return ""
}

// functionParamEntries lists the parameters of a function_value_parameters node.
func functionParamEntries(file *scanner.File, params uint32, res []*regexp.Regexp) []longParamEntry {
	var out []longParamEntry
	var mods uint32
	for c := file.FlatFirstChild(params); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "parameter_modifiers":
			mods = c
		case "parameter":
			e := longParamEntry{node: c, text: paramName(file, c) + ": " + paramTypeText(file, c)}
			if mods != 0 {
				e.annotated = annotatedParamMatches(file, longParamAnnotationNames(file, mods), res)
			}
			if n := file.FlatNextSib(c); n != 0 && file.FlatType(n) == "=" {
				e.hasDefault = true
			}
			out = append(out, e)
			mods = 0
		}
	}
	return out
}

// classParamEntries lists the parameters of a primary_constructor node.
func classParamEntries(file *scanner.File, ctor uint32, res []*regexp.Regexp) []longParamEntry {
	var out []longParamEntry
	for c := file.FlatFirstChild(ctor); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) != "class_parameter" {
			continue
		}
		e := longParamEntry{node: c, text: paramName(file, c) + ": " + paramTypeText(file, c), hasDefault: file.FlatHasChildOfType(c, "=")}
		if mods, ok := file.FlatFindChild(c, "modifiers"); ok {
			e.annotated = annotatedParamMatches(file, longParamAnnotationNames(file, mods), res)
		}
		out = append(out, e)
	}
	return out
}

// longParameterListMessage formats detekt's message. The parameter list is cut after 30 entries.
func longParameterListMessage(description string, entries []longParamEntry, threshold int) string {
	texts := make([]string, 0, len(entries))
	for i, e := range entries {
		if i == 30 {
			texts = append(texts, "...")
			break
		}
		texts = append(texts, e.text)
	}
	return fmt.Sprintf("The %s(%s) has too many parameters. The current threshold is set to %d.", description, strings.Join(texts, ", "), threshold)
}

// countLongParams counts the parameters detekt counts: all but the annotated ones the config ignores, and
// the ones with a default value when ignoreDefaultParameters is set.
func countLongParams(entries []longParamEntry, ignoreDefaults bool) int {
	n := 0
	for _, e := range entries {
		if e.annotated {
			continue
		}
		if ignoreDefaults && e.hasDefault {
			continue
		}
		n++
	}
	return n
}
