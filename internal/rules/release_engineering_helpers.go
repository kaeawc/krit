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
