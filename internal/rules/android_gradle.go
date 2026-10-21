package rules

// Android Gradle DSL lint rules.
// These rules analyze build.gradle / build.gradle.kts files using the Gradle
// scanner from internal/android/gradle.go.
//
// They run once per project on the parsed BuildConfig and raw file content,
// similar to how the manifest rules operate on AndroidManifest.xml.

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// GradleBase is an empty marker type embedded by Gradle rule
// implementations. The rule registry source records AndroidDependencies()
// metadata on api.Rule.AndroidDeps.
type GradleBase struct{}

func (GradleBase) AndroidDependencies() AndroidDataDependency {
	return AndroidDepGradle
}

// findGradleLine returns the 1-based line number of the first match of re in content, or 0.
// Still used for patterns that must remain regex (AGP version extraction).
func findGradleLine(content string, re *regexp.Regexp) int {
	loc := re.FindStringIndex(content)
	if loc == nil {
		return 0
	}
	return strings.Count(content[:loc[0]], "\n") + 1
}

// findGradleLineStr returns the 1-based line number of the first
// non-comment line where substr appears as code (i.e. outside string
// literals and trailing `//` line comments), or 0 if absent. Whole-line
// comments (after stripping leading whitespace, starting with `//`,
// `*`, or `/*`) are also skipped so we don't report findings on
// commented-out code or fragments inside doc-block continuations.
func findGradleLineStr(content, substr string) int {
	for i, line := range strings.Split(content, "\n") {
		if isGradleCommentLine(line) {
			continue
		}
		if strings.Contains(gradleStripStringsAndComments(line), substr) {
			return i + 1
		}
	}
	return 0
}

// gradleStripStringsAndComments returns line with the contents of
// string literals replaced by empty quotes ("" or ”) and any trailing
// `//` line comment removed. Backslash escapes inside string literals
// are honored so an escaped quote does not prematurely close the
// literal. The output preserves byte positions of code outside string
// literals so substring searches against code tokens (e.g. `targetSdk`,
// `mavenLocal()`, dependency coordinates) will not falsely match
// content embedded in a string like "http://example.com" or a comment
// like `// see targetSdk note`.
func gradleStripStringsAndComments(line string) string {
	var b strings.Builder
	b.Grow(len(line))
	inQuote := rune(0)
	escaped := false
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRuneInString(line[i:])
		if inQuote != 0 {
			if escaped {
				escaped = false
				i += size
				continue
			}
			if r == '\\' {
				escaped = true
				i += size
				continue
			}
			if r == inQuote {
				b.WriteRune(r)
				inQuote = 0
			}
			i += size
			continue
		}
		if r == '"' || r == '\'' {
			inQuote = r
			b.WriteRune(r)
			i += size
			continue
		}
		if r == '/' && i+1 < len(line) && line[i+1] == '/' {
			break
		}
		b.WriteRune(r)
		i += size
	}
	return b.String()
}

// isGradleCommentLine reports true when a Gradle line (Groovy or
// Kotlin DSL) is a single-line comment or a block-comment continuation
// and should therefore be excluded from semantic checks.
func isGradleCommentLine(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "//") || strings.HasPrefix(t, "*") || strings.HasPrefix(t, "/*")
}

// gradleLineCommentStart returns the byte index of the first `//` that
// begins a real Gradle line comment — i.e. one outside a single- or
// double-quoted string literal. Returns -1 when the line has no comment
// or only sees `//` inside a string (e.g. `"https://x"`). Backslash
// escapes inside literals are honored.
func gradleLineCommentStart(line string) int {
	var inQuote byte
	escaped := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inQuote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == inQuote {
				inQuote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQuote = c
			continue
		}
		if c == '/' && i+1 < len(line) && line[i+1] == '/' {
			return i
		}
	}
	return -1
}

func stripGradleLineComment(line string) string {
	inQuote := byte(0)
	triple := false
	escaped := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if escaped {
			escaped = false
			continue
		}
		if inQuote != 0 {
			if !triple && c == '\\' {
				escaped = true
				continue
			}
			if c == inQuote {
				if triple {
					if i+2 < len(line) && line[i+1] == inQuote && line[i+2] == inQuote {
						inQuote = 0
						triple = false
						i += 2
					}
					continue
				}
				inQuote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			if i+2 < len(line) && line[i+1] == c && line[i+2] == c {
				inQuote = c
				triple = true
				i += 2
				continue
			}
			inQuote = c
			continue
		}
		if c == '/' && i+1 < len(line) && line[i+1] == '/' {
			return line[:i]
		}
	}
	return line
}

func isGradleIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isGradleIdentPart(r rune) bool {
	return isGradleIdentStart(r) || unicode.IsDigit(r)
}

// gradleLineRef pairs a line's 1-based number with its raw text.
type gradleLineRef struct {
	num  int
	text string
}

// gradleDependencyBlockLines returns the non-comment lines that fall
// inside a `dependencies { ... }` block, including a nested
// `buildscript { dependencies { ... } }`. Brace tracking uses comment-
// and string-aware stripping so braces inside literals do not confuse
// depth. The block check provides receiver proof against local
// `fun implementation(s: String)` shadows that look like a dependency
// call but are not.
func gradleDependencyBlockLines(content string) []gradleLineRef {
	var out []gradleLineRef
	var stack []bool // each frame: true if it is a `dependencies { ... }` block
	insideDep := func() bool {
		for _, isDep := range stack {
			if isDep {
				return true
			}
		}
		return false
	}
	for i, raw := range strings.Split(content, "\n") {
		if !isGradleCommentLine(raw) && insideDep() {
			out = append(out, gradleLineRef{num: i + 1, text: raw})
		}
		stripped := gradleStripStringsAndComments(raw)
		for j := 0; j < len(stripped); j++ {
			switch stripped[j] {
			case '{':
				prefix := stripped[:j]
				if lastBrace := strings.LastIndexByte(prefix, '{'); lastBrace >= 0 {
					prefix = prefix[lastBrace+1:]
				}
				stack = append(stack, gradleHasDependenciesKeyword(prefix))
			case '}':
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
		}
	}
	return out
}

// gradleHasDependenciesKeyword reports whether s, the stripped text just
// before a `{`, contains a bare `dependencies` token (not a substring
// match like `subDependencies`). The token may be followed by optional
// whitespace before the brace.
func gradleHasDependenciesKeyword(s string) bool {
	idx := strings.Index(s, "dependencies")
	for idx >= 0 {
		end := idx + len("dependencies")
		startOK := idx == 0 || !isGradleIdentPartAt(s, idx, decodeBefore)
		endOK := end == len(s) || !isGradleIdentPartAt(s, end, decodeAfter)
		if startOK && endOK {
			return true
		}
		next := strings.Index(s[end:], "dependencies")
		if next < 0 {
			return false
		}
		idx = end + next
	}
	return false
}

type decodeDir int

const (
	decodeBefore decodeDir = iota
	decodeAfter
)

// isGradleIdentPartAt decodes the rune that bounds the candidate
// "dependencies" match and applies the identifier-part predicate to
// it. The previous implementation used rune(s[idx-1]) / rune(s[idx]),
// which is a byte-to-rune cast: when the surrounding byte was a UTF-8
// continuation byte (0x80-0xBF) the cast produced a value that
// IsLetter/IsDigit reject, so isGradleIdentPart returned false and
// the boundary check passed even when the real surrounding rune was a
// non-ASCII identifier character.
func isGradleIdentPartAt(s string, pos int, dir decodeDir) bool {
	switch dir {
	case decodeBefore:
		r, _ := utf8.DecodeLastRuneInString(s[:pos])
		return isGradleIdentPart(r)
	case decodeAfter:
		r, _ := utf8.DecodeRuneInString(s[pos:])
		return isGradleIdentPart(r)
	}
	return false
}

var (
	ktsDepConfigCallRe   = regexp.MustCompile(`\b(?:implementation|api|compileOnly|runtimeOnly|testImplementation|testRuntimeOnly|testCompileOnly|androidTestImplementation|androidTestCompileOnly|androidTestRuntimeOnly|debugImplementation|releaseImplementation|kapt|ksp|annotationProcessor)\s*\(`)
	ktsNamedArgVersionRe = regexp.MustCompile(`\bversion\s*=\s*["']([^"']+)["']`)
	ktsNamedArgGroupRe   = regexp.MustCompile(`\bgroup\s*=\s*["']([^"']+)["']`)
	ktsNamedArgNameRe    = regexp.MustCompile(`\bname\s*=\s*["']([^"']+)["']`)
)
