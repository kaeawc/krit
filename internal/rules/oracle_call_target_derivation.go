package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// deriveAnnotatedDeclarationCalleeNames extracts a conservative lexical
// callee allowlist from raw Kotlin files. It intentionally works on raw
// source bytes because this runs before the parser and before krit-types.
// Missed names can drop oracle-only findings, so when the scanner sees an
// annotated declaration shape but cannot recover its name it returns
// uncertain=true and the caller falls back to the broad JVM call pass.
func deriveAnnotatedDeclarationCalleeNames(files []*scanner.File, identifiers []string) ([]string, bool) {
	if len(files) == 0 || len(identifiers) == 0 {
		return nil, false
	}
	names := make([]string, 0)
	uncertain := false
	for _, file := range files {
		if file == nil || len(file.Content) == 0 {
			continue
		}
		fileNames, fileUncertain := deriveAnnotatedDeclarationCalleeNamesFromContent(string(file.Content), identifiers)
		names = append(names, fileNames...)
		if fileUncertain {
			uncertain = true
		}
	}
	return names, uncertain
}

func deriveAnnotatedDeclarationCalleeNamesFromContent(content string, identifiers []string) ([]string, bool) {
	annotationNames := annotationIdentifierSet(content, identifiers)
	if len(annotationNames) == 0 {
		return nil, false
	}

	var out []string
	pendingAnnotation := false
	pendingLines := 0
	uncertain := false

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			if pendingAnnotation {
				pendingLines++
			}
			continue
		}
		if lineHasAnnotationIdentifier(line, annotationNames) {
			pendingAnnotation = true
			pendingLines = 0
		}
		if !pendingAnnotation {
			continue
		}
		if name, hasDecl, unknown := annotatedDeclarationName(line); hasDecl {
			if unknown || name == "" {
				uncertain = true
			} else {
				out = append(out, name)
			}
			pendingAnnotation = false
			pendingLines = 0
			continue
		}
		pendingLines++
		if pendingLines > 25 {
			// An annotation with a very large argument block may still be
			// valid, but keeping the filter enabled past this point risks a
			// false negative. Let the caller fall back to broad resolution.
			uncertain = true
			pendingAnnotation = false
			pendingLines = 0
		}
	}
	return out, uncertain
}

func annotationIdentifierSet(content string, identifiers []string) map[string]bool {
	set := make(map[string]bool, len(identifiers)*2)
	for _, id := range identifiers {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		set[id] = true
		if simple := simpleQualifiedName(id); simple != "" {
			set[simple] = true
		}
	}
	if len(set) == 0 {
		return nil
	}

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "import ") || !strings.Contains(line, " as ") {
			continue
		}
		before, after, ok := strings.Cut(line, " as ")
		if !ok {
			continue
		}
		imported := strings.TrimSpace(strings.TrimPrefix(before, "import "))
		importedSimple := simpleQualifiedName(imported)
		if !set[imported] && !set[importedSimple] {
			continue
		}
		alias := readLeadingIdentifier(strings.TrimSpace(after))
		if alias != "" {
			set[alias] = true
		}
	}
	return set
}

func lineHasAnnotationIdentifier(line string, identifiers map[string]bool) bool {
	for i := strings.IndexByte(line, '@'); i >= 0; {
		j := i + 1
		for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
			j++
		}
		start := j
		for j < len(line) && isIdentByte(line[j]) {
			j++
		}
		if j < len(line) && line[j] == ':' && start < j {
			// Use-site target, e.g. @get:Deprecated.
			j++
		} else {
			j = start
		}
		for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
			j++
		}
		start = j
		for j < len(line) && (isIdentByte(line[j]) || line[j] == '.') {
			j++
		}
		token := strings.TrimSpace(line[start:j])
		if token != "" && (identifiers[token] || identifiers[simpleQualifiedName(token)]) {
			return true
		}
		next := strings.IndexByte(line[i+1:], '@')
		if next < 0 {
			break
		}
		i += next + 1
	}
	return false
}

func annotatedDeclarationName(line string) (name string, hasDecl bool, unknown bool) {
	line = stripLeadingAnnotations(line)
	bestKind := ""
	bestIdx := -1
	for _, kind := range []string{"fun", "class", "interface", "object", "val", "var"} {
		idx := findKeyword(line, kind)
		if idx < 0 {
			continue
		}
		if bestIdx < 0 || idx < bestIdx {
			bestIdx = idx
			bestKind = kind
		}
	}
	if bestIdx < 0 {
		return "", false, false
	}
	rest := strings.TrimSpace(line[bestIdx+len(bestKind):])
	switch bestKind {
	case "fun":
		name = functionDeclarationName(rest)
	default:
		name = readLeadingIdentifier(rest)
	}
	if name == "" {
		return "", true, true
	}
	return name, true, false
}

func stripLeadingAnnotations(line string) string {
	line = strings.TrimSpace(line)
	for strings.HasPrefix(line, "@") {
		i := 1
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		start := i
		for i < len(line) && isIdentByte(line[i]) {
			i++
		}
		if i < len(line) && line[i] == ':' && start < i {
			i++
		}
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		for i < len(line) && (isIdentByte(line[i]) || line[i] == '.') {
			i++
		}
		if i < len(line) && line[i] == '(' {
			if end := matchingParenEnd(line[i:]); end >= 0 {
				i += end + 1
			}
		}
		line = strings.TrimSpace(line[i:])
	}
	return line
}

func matchingParenEnd(s string) int {
	depth := 0
	var quote byte
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			continue
		}
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func functionDeclarationName(rest string) string {
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "<") {
		if end := matchingAngleEnd(rest); end >= 0 && end < len(rest)-1 {
			rest = strings.TrimSpace(rest[end+1:])
		}
	}
	if paren := strings.IndexByte(rest, '('); paren >= 0 {
		rest = rest[:paren]
	}
	return lastIdentifier(rest)
}

func matchingAngleEnd(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func findKeyword(s, kw string) int {
	for offset := 0; offset < len(s); {
		idx := strings.Index(s[offset:], kw)
		if idx < 0 {
			return -1
		}
		idx += offset
		beforeOK := idx == 0 || !isIdentByte(s[idx-1])
		after := idx + len(kw)
		afterOK := after == len(s) || !isIdentByte(s[after])
		if beforeOK && afterOK {
			return idx
		}
		offset = idx + len(kw)
	}
	return -1
}

func readLeadingIdentifier(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || !isIdentStartByte(s[0]) {
		return ""
	}
	end := 1
	for end < len(s) && isIdentByte(s[end]) {
		end++
	}
	return s[:end]
}

func lastIdentifier(s string) string {
	end := len(s)
	for end > 0 && !isIdentByte(s[end-1]) {
		end--
	}
	start := end
	for start > 0 && isIdentByte(s[start-1]) {
		start--
	}
	if start == end || !isIdentStartByte(s[start]) {
		return ""
	}
	return s[start:end]
}

func simpleQualifiedName(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.LastIndexByte(s, '.'); idx >= 0 && idx < len(s)-1 {
		return s[idx+1:]
	}
	return s
}

func isIdentStartByte(b byte) bool {
	return b == '_' || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isIdentByte(b byte) bool {
	return isIdentStartByte(b) || (b >= '0' && b <= '9')
}
