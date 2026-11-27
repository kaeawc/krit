package librarymodel

import "strings"

// gradleCode marks the bytes of a Gradle script, Kotlin or Groovy DSL, that are code: outside comments and outside
// the contents of string and character literals. A literal's quotes are code, and so is the code of a `${...}`
// template, the literals inside it lexed in turn, so a template's quotes and braces don't end the literal around it.
// The settings scanners match their patterns only where a match starts in code, so text in a string or a comment
// declares no catalog (#1872 review). scanner's Kotlin lexer can't serve here: it reads Groovy's 'single-quoted'
// strings as code.
//
// Both DSLs: "..." and """...""" take templates; '...' (a Groovy string, a Kotlin character) and Groovy's
// triple-single-quoted strings don't. A Kotlin raw string has no escapes; groovy selects Groovy's, where every
// literal has them. Groovy's slashy strings aren't recognized, as a slash can't be told from a division without
// parsing.
func gradleCode(src string, groovy bool) []bool {
	lx := gradleLexer{src: src, code: make([]bool, len(src)), groovy: groovy}
	lx.scan(0, false)
	return lx.code
}

// gradleCodeAt reports whether the byte at i is code (gradleCode).
func gradleCodeAt(code []bool, i int) bool {
	return i >= 0 && i < len(code) && code[i]
}

// isGroovyScript reports a Groovy DSL script by its file name: settings.gradle, not settings.gradle.kts.
func isGroovyScript(path string) bool {
	return !strings.HasSuffix(path, ".kts")
}

type gradleLexer struct {
	src    string
	code   []bool
	groovy bool
}

// scan marks code from i. In a template it returns after the '}' closing it.
func (lx *gradleLexer) scan(i int, inTemplate bool) int {
	src := lx.src
	depth := 0
	for i < len(src) {
		switch c := src[i]; {
		case c == '/' && strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		case c == '/' && strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return len(src)
			}
			i += end + 4
			continue
		case c == '"' || c == '\'':
			i = lx.literal(i)
			continue
		case c == '`':
			// A Kotlin backtick name.
			end := strings.IndexAny(src[i+1:], "`\n")
			if end < 0 {
				return len(src)
			}
			i += end + 2
			continue
		case c == '{':
			depth++
		case c == '}':
			if inTemplate && depth == 0 {
				lx.code[i] = true
				return i + 1
			}
			depth--
		}
		lx.code[i] = true
		i++
	}
	return i
}

// literal skips the string or character literal whose opening quote is at i and returns the index after it.
func (lx *gradleLexer) literal(i int) int {
	quote := lx.src[i]
	if strings.HasPrefix(lx.src[i:], strings.Repeat(string(quote), 3)) {
		return lx.tripleLiteral(i, quote)
	}
	lx.code[i] = true
	for i++; i < len(lx.src); {
		switch lx.src[i] {
		case '\\':
			i += 2
			continue
		case quote:
			lx.code[i] = true
			return i + 1
		case '\n':
			return i
		}
		i = lx.literalByte(i, quote)
	}
	return i
}

// tripleLiteral is literal for a triple-quoted literal whose opening quotes start at i.
func (lx *gradleLexer) tripleLiteral(i int, quote byte) int {
	src := lx.src
	lx.code[i], lx.code[i+1], lx.code[i+2] = true, true, true
	// A Kotlin raw string has no escapes.
	escapes := quote == '\'' || lx.groovy
	for i += 3; i < len(src); {
		switch c := src[i]; {
		case escapes && c == '\\':
			i += 2
			continue
		case c == quote:
			// A run of three or more quotes closes the literal at its end, as in Kotlin's """a"""".
			run := i
			for run < len(src) && src[run] == quote {
				run++
			}
			if run-i >= 3 {
				lx.code[run-3], lx.code[run-2], lx.code[run-1] = true, true, true
				return run
			}
			i = run
			continue
		}
		i = lx.literalByte(i, quote)
	}
	return i
}

// literalByte steps over the literal content at i: a ${...} template of a double-quoted literal, whose code it
// marks, or one byte.
func (lx *gradleLexer) literalByte(i int, quote byte) int {
	if quote == '"' && lx.src[i] == '$' && i+1 < len(lx.src) && lx.src[i+1] == '{' {
		lx.code[i], lx.code[i+1] = true, true
		return lx.scan(i+2, true)
	}
	return i + 1
}
