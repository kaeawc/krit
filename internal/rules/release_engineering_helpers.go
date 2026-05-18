package rules

// Pure helpers extracted from release_engineering.go.
//
// Functions in this file have no dependency on tree-sitter ASTs
// (*scanner.File) or any other Krit infrastructure — they take and return
// strings/paths/maps. That makes them straightforward to unit-test in
// isolation; see release_engineering_helpers_test.go.

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/sourceheader"
)

// commentedOutCallRe matches a single line that looks like a Kotlin call
// expression — used to recognise commented-out code blocks.
var commentedOutCallRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_\.]*\s*\([^)]*\)\s*[;{]?$`)

// isPlausibleCommentedKotlin reports whether a single line looks like
// commented-out Kotlin source rather than prose.
func isPlausibleCommentedKotlin(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "//") {
		return false
	}

	body := strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
	if body == "" {
		return false
	}
	if strings.HasSuffix(body, "{") || strings.HasSuffix(body, "}") || strings.HasSuffix(body, ";") {
		return true
	}

	for _, marker := range []string{"val ", "var ", "fun ", "if ", "when ", "return "} {
		if strings.Contains(body, marker) {
			return true
		}
	}

	if strings.Contains(body, " = ") && !strings.Contains(body, "==") {
		return true
	}

	return commentedOutCallRe.MatchString(body)
}

// conventionPluginID derives the Gradle plugin ID for a precompiled convention
// plugin file under build-logic/src/main/kotlin or buildSrc/src/main/kotlin.
// Returns "" when path is not a Gradle script under searchRoot.
func conventionPluginID(searchRoot, path string) string {
	rel, err := filepath.Rel(searchRoot, path)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)

	switch {
	case strings.HasSuffix(rel, ".gradle.kts"):
		rel = strings.TrimSuffix(rel, ".gradle.kts")
	case strings.HasSuffix(rel, ".gradle"):
		rel = strings.TrimSuffix(rel, ".gradle")
	default:
		return ""
	}

	rel = strings.Trim(rel, "./")
	if rel == "" {
		return ""
	}
	return strings.ReplaceAll(rel, "/", ".")
}

// isGradleBuildScript reports whether path's basename is a Gradle build script.
func isGradleBuildScript(path string) bool {
	switch filepath.Base(path) {
	case "build.gradle", "build.gradle.kts":
		return true
	default:
		return false
	}
}

var hardcodedEnvironmentNames = map[string]bool{
	"dev":       true,
	"localhost": true,
	"prod":      true,
	"qa":        true,
	"staging":   true,
}

// isEnvironmentConfigCallName reports whether name looks like a function/type
// that configures an environment.
func isEnvironmentConfigCallName(name string) bool {
	if name == "" {
		return false
	}
	return strings.Contains(name, "Environment") || strings.Contains(name, "Config") || strings.Contains(name, "Env")
}

// hardcodedEnvironmentLiteral returns the unquoted hardcoded environment name
// if the given source text is a quoted literal whose lower-cased value is one
// of the well-known environment names. labelPresent indicates whether the
// argument has a `name = ...` label that should be stripped first.
func hardcodedEnvironmentLiteral(text string, labelPresent bool) string {
	text = strings.TrimSpace(text)
	if labelPresent {
		if idx := strings.Index(text, "="); idx >= 0 {
			text = strings.TrimSpace(text[idx+1:])
		}
	}

	unquoted, err := strconv.Unquote(text)
	if err != nil {
		return ""
	}
	if !hardcodedEnvironmentNames[strings.ToLower(unquoted)] {
		return ""
	}
	return unquoted
}

// gradleStripCommentsFull returns content with Gradle (Groovy /
// Kotlin DSL) line comments and block comments replaced by spaces
// (preserving newlines). String literal bodies are preserved verbatim,
// which lets classifier substring checks see plugin-id strings like
// `'com.android.library'` while ignoring `// classpath
// 'com.android.application'` and `/* ... applicationId ... */` blocks.
// Triple-quoted Kotlin / Groovy strings are recognised so a `*/`
// embedded inside a raw string does not falsely close a comment, and
// backslash escapes inside regular strings are honored so an escaped
// quote does not prematurely close the literal.
func gradleStripCommentsFull(content string) string {
	return gradleStripContent(content, true)
}

// gradleStripCommentsAndStringBodiesFull returns content with Gradle
// (Groovy / Kotlin DSL) comments and string literal bodies replaced by
// spaces (preserving newlines). Both regular and triple-quoted strings
// are handled. The output preserves quote characters, comment-start
// punctuation positions, and newlines, so a substring search for a code
// token such as `applicationId` only sees real DSL references.
func gradleStripCommentsAndStringBodiesFull(content string) string {
	return gradleStripContent(content, false)
}

// gradleStripContent is the shared lexer behind gradleStripCommentsFull
// and gradleStripCommentsAndStringBodiesFull. preserveStringBodies
// controls whether string literal bodies are written through verbatim
// (true) or replaced with spaces (false).
func gradleStripContent(content string, preserveStringBodies bool) string {
	var b strings.Builder
	b.Grow(len(content))
	n := len(content)
	i := 0
	for i < n {
		c := content[i]
		if c == '/' && i+1 < n && content[i+1] == '*' {
			i = gradleConsumeBlockComment(&b, content, i)
			continue
		}
		if c == '/' && i+1 < n && content[i+1] == '/' {
			i = gradleConsumeLineComment(&b, content, i)
			continue
		}
		if (c == '"' || c == '\'') && i+2 < n && content[i+1] == c && content[i+2] == c {
			i = gradleConsumeTripleQuoted(&b, content, i, preserveStringBodies)
			continue
		}
		if c == '"' || c == '\'' {
			i = gradleConsumeRegularString(&b, content, i, preserveStringBodies)
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// gradleConsumeBlockComment writes a blanked-out block comment starting
// at content[i] (which must be `/*`) and returns the index just past the
// closing `*/`, or n if unterminated.
func gradleConsumeBlockComment(b *strings.Builder, content string, i int) int {
	n := len(content)
	b.WriteByte(' ')
	b.WriteByte(' ')
	i += 2
	for i < n {
		if content[i] == '*' && i+1 < n && content[i+1] == '/' {
			b.WriteByte(' ')
			b.WriteByte(' ')
			return i + 2
		}
		if content[i] == '\n' {
			b.WriteByte('\n')
		} else {
			b.WriteByte(' ')
		}
		i++
	}
	return i
}

// gradleConsumeLineComment writes a blanked-out line comment starting at
// content[i] (which must be `//`) and returns the index of the next
// newline (or n).
func gradleConsumeLineComment(b *strings.Builder, content string, i int) int {
	n := len(content)
	for i < n && content[i] != '\n' {
		b.WriteByte(' ')
		i++
	}
	return i
}

// gradleConsumeTripleQuoted handles a triple-quoted string literal
// starting at content[i] (which must be three identical quote bytes).
// When preserveBody is true, the body is written through verbatim;
// otherwise non-newline bytes are blanked. Returns the index past the
// closing delimiter or n if unterminated.
func gradleConsumeTripleQuoted(b *strings.Builder, content string, i int, preserveBody bool) int {
	n := len(content)
	quote := content[i]
	b.WriteByte(quote)
	b.WriteByte(quote)
	b.WriteByte(quote)
	i += 3
	for i < n {
		if i+2 < n && content[i] == quote && content[i+1] == quote && content[i+2] == quote {
			b.WriteByte(quote)
			b.WriteByte(quote)
			b.WriteByte(quote)
			return i + 3
		}
		if preserveBody {
			b.WriteByte(content[i])
		} else if content[i] == '\n' {
			b.WriteByte('\n')
		} else {
			b.WriteByte(' ')
		}
		i++
	}
	return i
}

// gradleConsumeRegularString handles a single- or double-quoted string
// literal starting at content[i]. preserveBody mirrors
// gradleConsumeTripleQuoted's semantics. Returns the index past the
// closing quote, or before a bare newline for an unterminated literal.
func gradleConsumeRegularString(b *strings.Builder, content string, i int, preserveBody bool) int {
	n := len(content)
	quote := content[i]
	b.WriteByte(quote)
	i++
	escaped := false
	for i < n {
		if content[i] == '\n' {
			return i
		}
		if escaped {
			escaped = false
			if preserveBody {
				b.WriteByte(content[i])
			} else {
				b.WriteByte(' ')
			}
			i++
			continue
		}
		if content[i] == '\\' {
			escaped = true
			if preserveBody {
				b.WriteByte(content[i])
			} else {
				b.WriteByte(' ')
			}
			i++
			continue
		}
		if content[i] == quote {
			b.WriteByte(quote)
			return i + 1
		}
		if preserveBody {
			b.WriteByte(content[i])
		} else {
			b.WriteByte(' ')
		}
		i++
	}
	return i
}

// gradleContainsCodeToken reports whether content contains needle as a
// whole token outside string literals or comments. Caller should pass
// the output of gradleStripCommentsAndStringBodiesFull as content so
// the check only sees real code. Word-boundary matching uses
// isGradleIdentByte so `myApplicationIdSuffix` won't be reported for
// `applicationId`, and `com.android.application2` won't be reported
// for `com.android.application`.
func gradleContainsCodeToken(content, needle string) bool {
	for offset := 0; ; {
		idx := strings.Index(content[offset:], needle)
		if idx < 0 {
			return false
		}
		idx += offset
		offset = idx + len(needle)
		if idx > 0 && isGradleIdentByte(content[idx-1]) {
			continue
		}
		end := idx + len(needle)
		if end < len(content) && isGradleIdentByte(content[end]) {
			continue
		}
		return true
	}
}

func isGradleIdentByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '$'
}

// stripBlockComments removes /* ... */ regions from line, threading the
// open-block state across lines. The returned bool reports whether the
// line ends still inside an unclosed block comment.
func stripBlockComments(line string, inBlock bool) (string, bool) {
	var b strings.Builder
	i := 0
	for i < len(line) {
		if inBlock {
			end := strings.Index(line[i:], "*/")
			if end < 0 {
				return b.String(), true
			}
			i += end + 2
			inBlock = false
			continue
		}
		if i+1 < len(line) && line[i] == '/' && line[i+1] == '*' {
			inBlock = true
			i += 2
			continue
		}
		b.WriteByte(line[i])
		i++
	}
	return b.String(), inBlock
}

// isDebugSourceFile reports whether path lies in a debug source set.
func isDebugSourceFile(path string) bool {
	return strings.Contains(path, "/debug/") || strings.Contains(path, "/src/debug/")
}

// isAndroidTestSupportArtifactSource reports whether path lies in a directory
// that produces an Android test-support artifact (fakes, fixtures, idling
// resources, instrumentation utilities, …).
func isAndroidTestSupportArtifactSource(path string) bool {
	normalized := filepath.ToSlash(strings.ToLower(path))
	if isTestSupportFile(normalized) {
		return true
	}
	for _, marker := range []string{
		"/test-fixtures/",
		"/testfixtures/",
		"/fakes/",
		"/fake/",
		"/mocks/",
		"/mock/",
		"/testing-ui/",
		"/ui-testing/",
		"/maestro-runner/",
		"/idling-resources/",
		"/idlingresources/",
		"/shared-instrumentation/",
		"/instrumentation-utils/",
		"/instrumentation-tests/",
		"/instrumentationtests/",
		"/android-test-utils/",
		"/androidtest-utils/",
		"/test-utils/",
		"/testutils/",
		"/testinternal/",
		"/testdebug/",
		"/testrelease/",
		"/testwrapper/",
		"/espressomodule.kt",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

// compactSourceReference strips whitespace from a source-reference text so
// references can be compared structurally regardless of formatting.
func compactSourceReference(text string) string {
	text = strings.TrimSpace(text)
	replacer := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "")
	return replacer.Replace(text)
}

// qualifySourceName joins a Kotlin/Java package and a simple name into a fully
// qualified name. An empty package returns the bare name.
func qualifySourceName(packageName, name string) string {
	if packageName == "" {
		return name
	}
	return packageName + "." + name
}

// simpleSourceName returns the trailing simple name component of a fully
// qualified name (e.g. "com.example.Foo" -> "Foo").
func simpleSourceName(qualifiedName string) string {
	if idx := strings.LastIndex(qualifiedName, "."); idx >= 0 {
		return qualifiedName[idx+1:]
	}
	return qualifiedName
}

// isTestFixturePath reports whether path is under a Gradle testFixtures source
// set.
func isTestFixturePath(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/testFixtures/")
}

// isGeneratedSourcePath reports whether path is under a generated/build output
// directory (KSP, KAPT, build/generated, etc.).
func isGeneratedSourcePath(path string) bool {
	path = filepath.ToSlash(path)
	return strings.Contains(path, "/build/generated/") ||
		strings.Contains(path, "/generated/") ||
		strings.Contains(path, "/ksp/") ||
		strings.Contains(path, "/kapt/")
}

// parseSourceImport parses a Kotlin/Java import line.
//
//	"import com.example.Foo"           -> ("com.example.Foo", "Foo", false)
//	"import com.example.Foo as Bar"    -> ("com.example.Foo", "Bar", false)
//	"import com.example.*"             -> ("com.example",     "",    true)
//	"import static x.Y.z"              -> ("x.Y.z",           "z",   false)
//
// Returns ("", "", false) when text is not an import line.
func parseSourceImport(text string) (qualifiedName string, localName string, wildcard bool) {
	// FirstSourceLine skips tree-sitter trailing trivia (block comment
	// trailers, blank-line tails) so the import-keyword gate isn't fooled
	// by a leading line/block comment.
	line := sourceheader.FirstSourceLine(text)
	if !strings.HasPrefix(line, "import") {
		return "", "", false
	}
	body := strings.TrimSpace(strings.TrimPrefix(line, "import"))
	body = strings.TrimSpace(strings.TrimSuffix(body, ";"))
	body = strings.TrimPrefix(body, "static ")
	body = strings.TrimSpace(body)
	if body == "" {
		return "", "", false
	}

	alias := ""
	if before, after, ok := strings.Cut(body, " as "); ok {
		body = strings.TrimSpace(before)
		alias = strings.TrimSpace(after)
	}
	if strings.HasSuffix(body, ".*") {
		return strings.TrimSuffix(body, ".*"), "", true
	}
	localName = alias
	if localName == "" {
		localName = simpleSourceName(body)
	}
	return body, localName, false
}

// timberSimpleName returns the trailing component of a Timber qualified name,
// splitting on either '.' or '#' (used by the resolver's "x.y.Z#method" form).
func timberSimpleName(name string) string {
	name = strings.TrimSpace(name)
	if dot := strings.LastIndexAny(name, ".#"); dot >= 0 {
		return name[dot+1:]
	}
	return name
}
