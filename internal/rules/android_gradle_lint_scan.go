package rules

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// lintGradleStmt is one DSL statement as Android Lint's Gradle visitor reports it: a property assignment or a call with
// a single value, together with the closures it sits in.
type lintGradleStmt struct {
	Line, Col int
	Name      string
	// Value is the assigned or single-argument text, comments removed. Quotes stay in place.
	Value    string
	ValueCol int
	// Parent and GrandParent name the innermost enclosing closures. Control flow blocks do not count as closures.
	Parent, GrandParent string
}

// lintGradleScope is one open brace on the scanner stack. Control flow blocks have an empty name.
type lintGradleScope struct{ name string }

// lintGradleControlWords open braces that are statements, not closures.
var lintGradleControlWords = map[string]bool{
	"if": true, "else": true, "for": true, "while": true, "do": true, "try": true, "catch": true,
	"finally": true, "switch": true, "when": true, "synchronized": true,
}

// lintGradleSkipWords start declarations, which are not DSL statements.
var lintGradleSkipWords = map[string]bool{
	"def": true, "val": true, "var": true, "final": true, "fun": true, "class": true, "import": true,
	"package": true, "return": true, "throw": true, "new": true, "static": true, "void": true, "int": true,
	"String": true, "boolean": true, "long": true,
}

// scanLintGradleStatements splits a Groovy or Kotlin build script into statements, one segment at a time, keeping the
// stack of enclosing closures. Statements that span lines are skipped.
func scanLintGradleStatements(content string) []lintGradleStmt {
	var out []lintGradleStmt
	var stack []lintGradleScope
	inBlock := false
	for i, raw := range strings.Split(content, "\n") {
		var code string
		code, inBlock = blankGradleComments(strings.TrimSuffix(raw, "\r"), inBlock)
		out = scanLintGradleLine(out, &stack, code, i+1)
	}
	return out
}

// scanLintGradleLine splits one comment-free line at braces and semicolons that sit outside quotes and parentheses.
func scanLintGradleLine(out []lintGradleStmt, stack *[]lintGradleScope, code string, lineNo int) []lintGradleStmt {
	start, depth := 0, 0
	var quote byte
	flush := func(end int) {
		if depth != 0 || end <= start {
			return
		}
		parent, grand := lintGradleParents(*stack)
		if st, ok := parseLintGradleStatement(code[start:end], lineNo, start, parent, grand); ok {
			out = append(out, st)
		}
	}
	for i := 0; i < len(code); i++ {
		c := code[i]
		if quote != 0 {
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				flush(i)
				start = i + 1
			}
		case '{':
			if depth == 0 {
				*stack = append(*stack, lintGradleScope{name: lintGradleBlockName(code[start:i])})
				start = i + 1
			}
		case '}':
			if depth == 0 {
				flush(i)
				if n := len(*stack); n > 0 {
					*stack = (*stack)[:n-1]
				}
				start = i + 1
			}
		}
	}
	flush(len(code))
	return out
}

// lintGradleParents returns the two innermost closure names.
func lintGradleParents(stack []lintGradleScope) (parent, grand string) {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].name == "" {
			continue
		}
		if parent == "" {
			parent = stack[i].name
			continue
		}
		return parent, stack[i].name
	}
	return parent, ""
}

// lintGradleBlockName names the closure opened after header: the callee name, or empty for control flow.
func lintGradleBlockName(header string) string {
	h := strings.TrimSpace(header)
	if strings.HasSuffix(h, ")") {
		open := matchingOpenParen(h)
		if open < 0 {
			return ""
		}
		h = strings.TrimSpace(h[:open])
		if lintGradleControlWords[h] {
			return ""
		}
	}
	end := len(h)
	begin := end
	for begin > 0 {
		r, size := utf8.DecodeLastRuneInString(h[:begin])
		if !isGradleIdentPart(r) {
			break
		}
		begin -= size
	}
	name := h[begin:end]
	if name == "" || lintGradleControlWords[name] {
		return ""
	}
	return name
}

// matchingOpenParen returns the index of the parenthesis that closes at the end of s, or -1.
func matchingOpenParen(s string) int {
	depth := 0
	var quote byte
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if quote != 0 {
			if c == quote && (i == 0 || s[i-1] != '\\') {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case ')':
			depth++
		case '(':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// matchingCloseParen returns the index of the parenthesis that closes the one at open, or -1.
func matchingCloseParen(s string, open int) int {
	depth := 0
	var quote byte
	for i := open; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
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

// parseLintGradleStatement classifies seg as `name = value`, `name(value)` or `name value`.
func parseLintGradleStatement(seg string, lineNo, offset int, parent, grand string) (lintGradleStmt, bool) {
	lead := len(seg) - len(strings.TrimLeftFunc(seg, unicode.IsSpace))
	s := strings.TrimSpace(seg)
	if s == "" {
		return lintGradleStmt{}, false
	}
	nameEnd := 0
	for nameEnd < len(s) {
		r, size := utf8.DecodeRuneInString(s[nameEnd:])
		if !isGradleIdentPart(r) {
			break
		}
		nameEnd += size
	}
	if nameEnd == 0 {
		return lintGradleStmt{}, false
	}
	name := s[:nameEnd]
	if lintGradleSkipWords[name] {
		return lintGradleStmt{}, false
	}
	st := lintGradleStmt{Line: lineNo, Col: offset + lead + 1, Name: name, Parent: parent, GrandParent: grand}
	rest := s[nameEnd:]
	trimmed := strings.TrimLeftFunc(rest, unicode.IsSpace)
	var valStart int
	valEnd := len(s)
	switch {
	case strings.HasPrefix(trimmed, "=") && !strings.HasPrefix(trimmed, "=="):
		valStart = len(s) - len(trimmed) + 1
	case strings.HasPrefix(rest, "("):
		closeIdx := matchingCloseParen(s, nameEnd)
		// Infix calls such as `id("x") version "1"` keep the first argument list; member calls do not.
		if closeIdx < 0 || (closeIdx+1 < len(s) && s[closeIdx+1] != ' ' && s[closeIdx+1] != '\t') {
			return lintGradleStmt{}, false
		}
		valStart, valEnd = nameEnd+1, closeIdx
	case trimmed != "" && rest != trimmed && lintGradleCommandStart(trimmed):
		valStart = len(s) - len(trimmed)
	default:
		return lintGradleStmt{}, false
	}
	for valStart < valEnd && (s[valStart] == ' ' || s[valStart] == '\t') {
		valStart++
	}
	st.Value = strings.TrimSpace(s[valStart:valEnd])
	st.ValueCol = offset + lead + valStart + 1
	return st, true
}

// lintGradleCommandStart reports whether text can be the argument of a Groovy command call such as `compile 'a:b:1'`.
func lintGradleCommandStart(text string) bool {
	switch text[0] {
	case '"', '\'', '[', '(':
		return true
	}
	r, _ := utf8.DecodeRuneInString(text)
	return isGradleIdentStart(r) || unicode.IsDigit(r) || text[0] == '-'
}

// blankGradleComments replaces comments in line with spaces so columns survive, and carries block comment state.
func blankGradleComments(line string, inBlock bool) (string, bool) {
	b := []byte(line)
	var quote byte
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case inBlock:
			if c == '*' && i+1 < len(b) && b[i+1] == '/' {
				b[i], b[i+1] = ' ', ' '
				i++
				inBlock = false
			} else {
				b[i] = ' '
			}
		case quote != 0:
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '/' && i+1 < len(b) && b[i+1] == '/':
			for j := i; j < len(b); j++ {
				b[j] = ' '
			}
			return string(b), false
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			b[i], b[i+1] = ' ', ' '
			i++
			inBlock = true
		}
	}
	return string(b), inBlock
}
