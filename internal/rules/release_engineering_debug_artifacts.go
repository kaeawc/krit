package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------- New release-engineering rules ----------

// CommentedOutImportRule flags `// import ...` lines — a commented-out import
// is either dead or a half-done refactor.
type CommentedOutImportRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CommentedOutImportRule) Confidence() float64 { return api.ConfidenceHigher }

// commentedImportRe matches a commented-out Kotlin import body. Requires a
// dotted-path that conforms to Kotlin import syntax — bare prose like
// "import order matters" won't match. Optional trailing `.*` and `as` alias.
var commentedImportRe = regexp.MustCompile(
	`^import\s+[\p{L}_][\p{L}\p{N}_]*(?:\.[\p{L}_][\p{L}\p{N}_]*)*(?:\.\*)?(?:\s+as\s+[\p{L}_][\p{L}\p{N}_]*)?\s*;?\s*$`)

func (r *CommentedOutImportRule) checkNode(ctx *api.Context) {
	file := ctx.File
	if !strings.HasSuffix(file.Path, ".kt") && !strings.HasSuffix(file.Path, ".kts") {
		return
	}
	text := file.FlatNodeText(ctx.Idx)
	if !strings.HasPrefix(text, "//") {
		return
	}
	body := strings.TrimSpace(strings.TrimPrefix(text, "//"))
	if !strings.HasPrefix(body, "import ") {
		return
	}
	if !commentedImportRe.MatchString(body) {
		return
	}
	row := file.FlatRow(ctx.Idx)
	f := r.Finding(file, row+1, 1,
		"Commented-out import; remove it or restore it as a live import.")
	f.Fix = deleteLineFix(file, row)
	ctx.Emit(f)
}

// deleteLineFix returns a byte-mode Fix that removes a single 0-indexed
// row, including its trailing newline (or the preceding newline when the
// line is the last in the file). Returns nil when the row is out of range.
func deleteLineFix(file *scanner.File, row int) *scanner.Fix {
	return deleteLineRangeFix(file, row, row)
}

// deleteLineRangeFix returns a byte-mode Fix that removes lines
// [startRow, endRow] (inclusive, 0-indexed) along with their trailing
// newlines. When the range reaches the end of the file with no trailing
// newline, the preceding newline is consumed instead so neighboring lines
// stay separated. Returns nil when the range is out of bounds.
func deleteLineRangeFix(file *scanner.File, startRow, endRow int) *scanner.Fix {
	if startRow < 0 || endRow < startRow || endRow >= len(file.Lines) {
		return nil
	}
	start := file.LineOffset(startRow)
	end := file.LineOffset(endRow) + len(file.Lines[endRow])
	if end < len(file.Content) && file.Content[end] == '\n' {
		end++
	} else if start > 0 && file.Content[start-1] == '\n' {
		start--
	}
	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   start,
		EndByte:     end,
		Replacement: "",
	}
}

// DebugToastInProductionRule flags Toast.makeText calls whose message literal
// starts with "debug", "test", or "wip" (case-insensitive).
type DebugToastInProductionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *DebugToastInProductionRule) Confidence() float64 { return api.ConfidenceHigh }

var debugToastPrefixRe = regexp.MustCompile(`(?i)^["'](debug|test|wip)(?:[^A-Za-z0-9]|$)`)

// MergeConflictMarkerLeftoverRule flags unresolved merge conflict markers.
type MergeConflictMarkerLeftoverRule struct {
	LineBase
	BaseRule
}

func (r *MergeConflictMarkerLeftoverRule) Confidence() float64 { return 0.99 }

func (r *MergeConflictMarkerLeftoverRule) check(ctx *api.Context) {
	file := ctx.File
	if !strings.HasSuffix(file.Path, ".kt") && !strings.HasSuffix(file.Path, ".kts") &&
		!strings.HasSuffix(file.Path, ".java") {
		return
	}

	insideRawString := computeRawStringLines(file.Lines)
	insideBlockComment := computeBlockCommentLines(file.Lines)

	for i, line := range file.Lines {
		// Real merge conflict markers always begin at column 0 with no
		// leading whitespace. Requiring an exact prefix match avoids
		// flagging markers that appear inside indented strings or
		// comments.
		isStart := strings.HasPrefix(line, "<<<<<<<")
		isEnd := strings.HasPrefix(line, ">>>>>>>")
		isSep := strings.TrimRight(line, " \t") == "======="
		if !isStart && !isEnd && !isSep {
			continue
		}
		// Skip lines that are continuations of a multi-line raw string or
		// block comment — those are textual content, not real markers.
		if insideRawString[i] || insideBlockComment[i] {
			continue
		}
		ctx.Emit(r.Finding(file, i+1, 1,
			"Unresolved merge conflict marker; resolve the conflict before committing."))
	}
}

// PrintlnInProductionRule flags println/print/System.out.println/System.err.println
// in non-test files, outside a top-level fun main().
type PrintlnInProductionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *PrintlnInProductionRule) Confidence() float64 { return api.ConfidenceHigh }

var printlnNames = map[string]bool{
	"println": true,
	"print":   true,
}

func isProductionPrintCallFlat(file *scanner.File, idx uint32, name, receiver string) bool {
	if !printlnNames[name] {
		return false
	}
	callee, _ := flatCallExpressionParts(file, idx)
	if callee != 0 && file.FlatType(callee) == "navigation_expression" {
		segs := flatNavigationChainIdentifiers(file, callee)
		// Require exactly the System.out.<name> / System.err.<name> shape
		// where <name> is the println/print already validated above. Walking
		// the chain structurally avoids false positives from text-joining
		// (comments, whitespace collapse) and false negatives from
		// non-bare-identifier receivers (e.g. `getSystem().out.println`).
		return len(segs) == 3 && segs[0] == "System" &&
			(segs[1] == "out" || segs[1] == "err") &&
			segs[2] == name
	}
	if receiver != "" {
		return false
	}
	// Bare `println(...)` resolves to kotlin.io.println only when nothing
	// in the file shadows the built-in name: no top-level/member/local
	// function_declaration named `println`, and no non-kotlin.io import
	// (or alias) that brings a same-named symbol into scope.
	return !kotlinFileShadowsBuiltinPrint(file, name)
}

// printBuiltinShadowFacts records, per file, whether the kotlin.io
// built-in `println` and/or `print` are shadowed by a same-file
// declaration or non-kotlin.io import. PrintlnInProduction calls into
// this once per check; the dispatcher invokes the check per matching
// call_expression, so without memoization a file with N bare
// println/print calls would re-walk the entire AST and import header N
// times.
type printBuiltinShadowFacts struct {
	println bool
	print   bool
}

func (f printBuiltinShadowFacts) shadows(name string) bool {
	switch name {
	case "println":
		return f.println
	case "print":
		return f.print
	}
	return false
}

// kotlinFileShadowsBuiltinPrint returns true when the file declares a
// function named `name` at any scope, or imports a non-kotlin.io symbol
// that would resolve before kotlin.io.<name> for a bare call. Result is
// cached per-file via filefacts; only one AST walk and one import scan
// run per file regardless of how many bare-call sites the rule visits.
func kotlinFileShadowsBuiltinPrint(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	return printBuiltinShadowFactsFor(file).shadows(name)
}

func printBuiltinShadowFactsFor(file *scanner.File) printBuiltinShadowFacts {
	return filefacts.FileFact(fileFactsCache(), file, slotPrintBuiltinShadows, func() printBuiltinShadowFacts {
		return computePrintBuiltinShadowFacts(file)
	})
}

func computePrintBuiltinShadowFacts(file *scanner.File) printBuiltinShadowFacts {
	var facts printBuiltinShadowFacts
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if facts.println && facts.print {
			return
		}
		if file.FlatType(idx) != "function_declaration" {
			return
		}
		switch flatFunctionName(file, idx) {
		case "println":
			facts.println = true
		case "print":
			facts.print = true
		}
	})
	if facts.println && facts.print {
		return facts
	}
	// One import-header pass detects shadowing for both names.
	kotlinScanImportsForPrintShadow(file, &facts)
	return facts
}

// kotlinScanImportsForPrintShadow walks the file's import header and
// sets facts.println / facts.print whenever an import statement (or
// `as` alias) brings a same-named symbol into scope, other than an
// explicit `import kotlin.io.<name>` re-import of the built-in.
// Comments are stripped so a commented-out import does not register.
func kotlinScanImportsForPrintShadow(file *scanner.File, facts *printBuiltinShadowFacts) {
	inBlockComment := false
	for _, line := range file.Lines {
		if facts.println && facts.print {
			return
		}
		stripped, stillInBlock := stripBlockComments(line, inBlockComment)
		inBlockComment = stillInBlock
		if i := strings.Index(stripped, "//"); i >= 0 {
			stripped = stripped[:i]
		}
		trimmed := strings.TrimSpace(stripped)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "import ") {
			if strings.HasPrefix(trimmed, "package ") {
				continue
			}
			// Imports must precede the first declaration; stop scanning.
			return
		}
		spec := strings.TrimSpace(strings.TrimPrefix(trimmed, "import "))
		// Alias takes precedence: `import a.b.X as <name>` shadows.
		if _, after, ok := strings.Cut(spec, " as "); ok {
			alias := strings.TrimSpace(strings.TrimSuffix(after, ";"))
			markPrintShadow(facts, alias)
			continue
		}
		spec = strings.TrimSpace(strings.TrimSuffix(spec, ";"))
		switch spec {
		case "kotlin.io.println", "kotlin.io.print":
			// Re-import of the built-in does not shadow.
			continue
		}
		if i := strings.LastIndex(spec, "."); i >= 0 {
			markPrintShadow(facts, spec[i+1:])
		}
	}
}

func markPrintShadow(facts *printBuiltinShadowFacts, name string) {
	switch name {
	case "println":
		facts.println = true
	case "print":
		facts.print = true
	}
}

func printlnNonProductionPath(path string) bool {
	if scanner.IsTestFile(path) || isGradleBuildScript(path) || strings.HasSuffix(filepath.Base(path), ".kts") {
		return true
	}
	lower := strings.ToLower(path)
	return strings.Contains(lower, "/samples/") ||
		strings.Contains(lower, "/sample/") ||
		strings.Contains(lower, "/demos/") ||
		strings.Contains(lower, "/demo/")
}

func printlnInsideGradleTaskAction(file *scanner.File, idx uint32) bool {
	fn, ok := flatEnclosingFunction(file, idx)
	return ok && flatHasAnnotationNamed(file, fn, "TaskAction")
}

// PrintStackTraceInProductionRule flags e.printStackTrace() calls in non-test
// files that import a logging framework.
type PrintStackTraceInProductionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *PrintStackTraceInProductionRule) Confidence() float64 { return api.ConfidenceHigh }

var loggingImports = []string{
	"timber.log.Timber",
	"android.util.Log",
	"org.slf4j.",
	"java.util.logging.",
	"io.github.oshai.kotlinlogging.",
	"mu.KotlinLogging",
	"ch.qos.logback.",
	"org.apache.logging.",
}

func hasLoggingImport(file *scanner.File) bool {
	inBlockComment := false
	for _, line := range file.Lines {
		stripped, stillInBlock := stripBlockComments(line, inBlockComment)
		inBlockComment = stillInBlock
		if i := strings.Index(stripped, "//"); i >= 0 {
			stripped = stripped[:i]
		}
		trimmed := strings.TrimSpace(stripped)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "import ") {
			if strings.HasPrefix(trimmed, "package ") {
				continue
			}
			break
		}
		pkg := strings.TrimSpace(strings.TrimPrefix(trimmed, "import "))
		// Strip a trailing alias (`import a.b.C as D`) and Java's
		// optional `;` terminator so the prefix anchor compares against
		// the qualified name itself.
		if before, _, ok := strings.Cut(pkg, " as "); ok {
			pkg = strings.TrimSpace(before)
		}
		pkg = strings.TrimSpace(strings.TrimSuffix(pkg, ";"))
		for _, prefix := range loggingImports {
			// Anchor the match at the start of the import path so
			// `import foo.mu.X` or `import com.example.demu.X` does not
			// falsely qualify as a logging import. Class-name entries
			// (e.g. `timber.log.Timber`) and package-prefix entries
			// ending in `.` (e.g. `java.util.logging.`) are both
			// covered by HasPrefix.
			if strings.HasPrefix(pkg, prefix) {
				return true
			}
		}
	}
	return false
}

// HardcodedLocalhostURLRule flags URL literals containing localhost or 10.0.2.2
// in non-test non-debug source files.
type HardcodedLocalhostURLRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence: tier-1 — we dispatch on string_literal and read the literal's
// exact content via the AST's string_content children. A URL never crosses
// into a raw-string template or an interpolated expression by accident; if
// the literal has such an expression child, we refuse to match (we cannot
// prove the runtime URL is `localhost`). No line scanning, no quote
// gymnastics.
func (r *HardcodedLocalhostURLRule) Confidence() float64 { return api.ConfidenceVeryHigh }

var localhostURLRe = regexp.MustCompile(`^https?://(localhost|127\.0\.0\.1|10\.0\.2\.2)(:\d+)?(/.*)?$`)

// check runs per string_literal. The former implementation scanned
// file.Lines for a regex that matched the literal together with its
// surrounding quote characters — this was fragile across raw string
// forms, multi-line strings, and comment contexts. Now we inspect the
// AST node directly.
func (r *HardcodedLocalhostURLRule) check(ctx *api.Context) {
	file := ctx.File
	if file.Language != scanner.LangKotlin && file.Language != scanner.LangJava {
		return
	}
	if scanner.IsTestFile(file.Path) || isDebugSourceFile(file.Path) {
		return
	}
	idx := ctx.Idx
	if flatContainsStringInterpolation(file, idx) {
		return // interpolated content — cannot confirm runtime value.
	}
	content := stringLiteralContent(file, idx)
	if content == "" {
		return
	}
	if !localhostURLRe.MatchString(content) {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Hardcoded localhost URL in production source; use a build config or environment variable."))
}

// TestOnlyImportInProductionRule flags test-framework imports in non-test files.
type TestOnlyImportInProductionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestOnlyImportInProductionRule) Confidence() float64 { return api.ConfidenceVeryHigh }

var testOnlyImportPrefixes = []string{
	"org.mockito.",
	"io.mockk.",
	"org.junit.",
	"org.testng.",
	"org.robolectric.",
	"androidx.test.",
	"com.google.common.truth.",
	"org.assertj.",
	"org.hamcrest.",
	"kotlin.test.",
	"org.mockserver.",
	"com.nhaarman.mockitokotlin2.",
	"org.mockito_kotlin.",
}

// check runs per import_header. It reads the imported FQN directly from
// the `identifier` child and flags it when the FQN is under any of the
// test-framework package prefixes and the file is not itself a test
// source. This replaces a per-line `strings.HasPrefix(trimmed, "import ")`
// scan that also dealt with stripping trailing `as Alias` syntax — the
// AST already separates the identifier from the import_alias child.
func (r *TestOnlyImportInProductionRule) check(ctx *api.Context) {
	file := ctx.File
	if !strings.HasSuffix(file.Path, ".kt") && !strings.HasSuffix(file.Path, ".kts") {
		return
	}
	if scanner.IsTestFile(file.Path) {
		return
	}
	if isAndroidTestSupportArtifactSource(file.Path) {
		return
	}
	idx := ctx.Idx
	ident, ok := file.FlatFindChild(idx, "identifier")
	if !ok {
		return
	}
	fqn := file.FlatNodeText(ident)
	for _, prefix := range testOnlyImportPrefixes {
		if strings.HasPrefix(fqn, prefix) {
			ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				fmt.Sprintf("Test-only import %q in non-test file; move this code to a test source set.", fqn)))
			return
		}
	}
}

// NonASCIIIdentifierRule flags class/function/property names containing
// non-ASCII characters.
type NonASCIIIdentifierRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *NonASCIIIdentifierRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// HardcodedLogTagRule flags Log.d("ClassName", ...) where the tag matches
// the enclosing class name instead of using a companion TAG constant.
type HardcodedLogTagRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *HardcodedLogTagRule) Confidence() float64 { return api.ConfidenceMediumHigh }

var logLevelMethods = map[string]bool{
	"v": true, "d": true, "i": true, "w": true, "e": true, "wtf": true,
}

func flatEnclosingClassName(file *scanner.File, idx uint32) string {
	current := idx
	for {
		parent, ok := file.FlatParent(current)
		if !ok || parent == 0 {
			return ""
		}
		if file.FlatType(parent) == "class_declaration" {
			for i := 0; i < file.FlatChildCount(parent); i++ {
				child := file.FlatChild(parent, i)
				if file.FlatType(child) == "type_identifier" {
					return file.FlatNodeText(child)
				}
			}
			return ""
		}
		current = parent
	}
}

func flatEnclosingOwnerName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	var owners []string
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "class_declaration", "object_declaration":
			if name := flatOwnerDeclarationName(file, current); name != "" {
				owners = append(owners, name)
			}
		}
	}
	for i, j := 0, len(owners)-1; i < j; i, j = i+1, j-1 {
		owners[i], owners[j] = owners[j], owners[i]
	}
	return strings.Join(owners, ".")
}

func flatOwnerDeclarationName(file *scanner.File, idx uint32) string {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "simple_identifier":
			return file.FlatNodeString(child, nil)
		}
	}
	return ""
}
