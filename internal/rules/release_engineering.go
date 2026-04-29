package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/rules/semantics"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

const releaseEngineeringRuleSet = "release-engineering"

// BuildConfigDebugInLibraryRule flags BuildConfig.DEBUG references inside
// Android library modules, where the merged consumer BuildConfig sets DEBUG to
// false in release builds.
type BuildConfigDebugInLibraryRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Release-engineering rule. Detection scans module metadata and Gradle
// files for configuration drift and plugin hygiene; matches are
// project-structure-sensitive. Classified per roadmap/17.
func (r *BuildConfigDebugInLibraryRule) Confidence() float64 { return 0.75 }

// BuildConfigDebugInvertedRule flags `if (!BuildConfig.DEBUG) { ...logging... }`
// patterns where debug-only logging appears to be guarded in the opposite
// direction.
type BuildConfigDebugInvertedRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Release-engineering rule. Detection scans module metadata and Gradle
// files for configuration drift and plugin hygiene; matches are
// project-structure-sensitive. Classified per roadmap/17.
func (r *BuildConfigDebugInvertedRule) Confidence() float64 { return 0.75 }

// AllProjectsBlockRule flags deprecated allprojects { } usage in Gradle build
// scripts. Convention plugins or settings-level repositories are the
// recommended replacement.
type AllProjectsBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// HardcodedEnvironmentNameRule flags literal environment names passed into
// config/environment-like APIs where a build- or runtime-derived value is
// expected instead.
type HardcodedEnvironmentNameRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Release-engineering rule. Detection scans module metadata and Gradle
// files for configuration drift and plugin hygiene; matches are
// project-structure-sensitive. Classified per roadmap/17.
func (r *AllProjectsBlockRule) Confidence() float64 { return 0.75 }

// Confidence reports a tier-2 (medium) base confidence. Release-engineering rule. Detection scans module metadata and Gradle
// files for configuration drift and plugin hygiene; matches are
// project-structure-sensitive. Classified per roadmap/17.
func (r *HardcodedEnvironmentNameRule) Confidence() float64 { return 0.75 }

// ConventionPluginDeadCodeRule flags precompiled convention plugins declared
// under build-logic/ or buildSrc/ that are not applied by any module.
type ConventionPluginDeadCodeRule struct {
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Release-engineering rule. Detection scans module metadata and Gradle
// files for configuration drift and plugin hygiene; matches are
// project-structure-sensitive. Classified per roadmap/17.
func (r *ConventionPluginDeadCodeRule) Confidence() float64 { return 0.75 }

func (r *ConventionPluginDeadCodeRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{}
}

func (r *ConventionPluginDeadCodeRule) check(ctx *v2.Context) {
	pmi := ctx.ModuleIndex
	if pmi == nil || pmi.Graph == nil || pmi.Graph.RootDir == "" {
		return
	}

	plugins := discoverConventionPlugins(pmi.Graph.RootDir)
	if len(plugins) == 0 {
		return
	}

	applied := scanAppliedConventionPluginIDs(pmi.Graph)
	for _, plugin := range plugins {
		if applied[plugin.id] {
			continue
		}
		ctx.Emit(scanner.Finding{
			File:       plugin.path,
			Line:       1,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    fmt.Sprintf("Convention plugin '%s' is defined but never applied by any module build script.", plugin.id),
			Confidence: 0.9,
		})
	}
}

type conventionPlugin struct {
	id   string
	path string
}

var conventionPluginUsageRe = regexp.MustCompile(`id\s*\(\s*["']([^"']+)["']\s*\)`)
var commentedOutCallRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_\.]*\s*\([^)]*\)\s*[;{]?$`)

// CommentedOutCodeBlockRule flags runs of line comments that look like disabled
// Kotlin statements rather than prose.
type CommentedOutCodeBlockRule struct {
	LineBase
	BaseRule
	MinLines int
}

// GradleBuildContainsTodoRule flags TODO line comments in build.gradle(.kts)
// scripts so release-affecting build work does not linger unnoticed.
type GradleBuildContainsTodoRule struct {
	LineBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Release-engineering rule. Detection scans module metadata and Gradle
// files for configuration drift and plugin hygiene; matches are
// project-structure-sensitive. Classified per roadmap/17.
func (r *GradleBuildContainsTodoRule) Confidence() float64 { return 0.75 }

func (r *GradleBuildContainsTodoRule) check(ctx *v2.Context) {
	file := ctx.File
	if !isGradleBuildScript(file.Path) {
		return
	}

	for i, line := range file.Lines {
		commentIdx := strings.Index(line, "//")
		if commentIdx < 0 {
			continue
		}

		comment := strings.TrimSpace(line[commentIdx+2:])
		if !strings.HasPrefix(comment, "TODO") {
			continue
		}

		ctx.Emit(r.Finding(file, i+1, commentIdx+1,
			"TODO comment found in build.gradle(.kts); track build work in an issue or finish it before release."))
	}
}

// Confidence reports a tier-2 (medium) base confidence. Release-engineering rule. Detection scans module metadata and Gradle
// files for configuration drift and plugin hygiene; matches are
// project-structure-sensitive. Classified per roadmap/17.
func (r *CommentedOutCodeBlockRule) Confidence() float64 { return 0.75 }

func (r *CommentedOutCodeBlockRule) check(ctx *v2.Context) {
	file := ctx.File
	if !strings.HasSuffix(file.Path, ".kt") && !strings.HasSuffix(file.Path, ".kts") {
		return
	}

	startLine := -1
	count := 0
	flush := func(endLine int) {
		if count < r.MinLines || startLine < 0 {
			startLine = -1
			count = 0
			return
		}

		line := file.Lines[startLine]
		col := strings.Index(line, "//")
		if col < 0 {
			col = 0
		}

		msg := fmt.Sprintf("Commented-out code block detected across %d consecutive lines; delete it or restore it as live code.", endLine-startLine)
		ctx.Emit(r.Finding(file, startLine+1, col+1, msg))
		startLine = -1
		count = 0
	}

	for i, line := range file.Lines {
		if isPlausibleCommentedKotlin(line) {
			if startLine < 0 {
				startLine = i
			}
			count++
			continue
		}
		flush(i)
	}
	flush(len(file.Lines))
}

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

func discoverConventionPlugins(root string) []conventionPlugin {
	searchRoots := []string{
		filepath.Join(root, "build-logic", "src", "main", "kotlin"),
		filepath.Join(root, "buildSrc", "src", "main", "kotlin"),
	}

	var plugins []conventionPlugin
	seen := make(map[string]bool)
	for _, searchRoot := range searchRoots {
		info, err := os.Stat(searchRoot)
		if err != nil || !info.IsDir() {
			continue
		}

		_ = filepath.Walk(searchRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}

			id := conventionPluginID(searchRoot, path)
			if id == "" || seen[id] {
				return nil
			}

			seen[id] = true
			plugins = append(plugins, conventionPlugin{id: id, path: path})
			return nil
		})
	}

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].path < plugins[j].path
	})
	return plugins
}

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

func scanAppliedConventionPluginIDs(graph *module.ModuleGraph) map[string]bool {
	used := make(map[string]bool)
	for _, script := range gradleScriptsToInspect(graph) {
		data, err := os.ReadFile(script)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "apply false") {
				continue
			}
			for _, match := range conventionPluginUsageRe.FindAllStringSubmatch(line, -1) {
				used[match[1]] = true
			}
		}
	}
	return used
}

func gradleScriptsToInspect(graph *module.ModuleGraph) []string {
	if graph == nil {
		return nil
	}

	paths := map[string]bool{
		filepath.Join(graph.RootDir, "build.gradle.kts"):    true,
		filepath.Join(graph.RootDir, "build.gradle"):        true,
		filepath.Join(graph.RootDir, "settings.gradle.kts"): true,
		filepath.Join(graph.RootDir, "settings.gradle"):     true,
	}
	for _, mod := range graph.Modules {
		if mod == nil || mod.Dir == "" {
			continue
		}
		paths[filepath.Join(mod.Dir, "build.gradle.kts")] = true
		paths[filepath.Join(mod.Dir, "build.gradle")] = true
	}

	scripts := make([]string, 0, len(paths))
	for path := range paths {
		scripts = append(scripts, path)
	}
	sort.Strings(scripts)
	return scripts
}

func isBuildConfigDebugReferenceFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) == "navigation_expression" {
		return false
	}
	text := file.FlatNodeText(idx)
	return text == "BuildConfig.DEBUG" || strings.HasSuffix(text, ".BuildConfig.DEBUG")
}

func ifConditionAndThenBodyFlat(file *scanner.File, idx uint32) (condition uint32, body uint32) {
	if file == nil || idx == 0 || file.FlatType(idx) != "if_expression" {
		return 0, 0
	}

	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "if", "(", ")", "{", "}":
			continue
		default:
			if condition == 0 {
				condition = child
				continue
			}
			if body == 0 {
				body = child
				return condition, body
			}
		}
	}

	return condition, body
}

func isNegatedBuildConfigDebugConditionFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}

	text := strings.TrimSpace(file.FlatNodeText(idx))
	text = strings.TrimPrefix(text, "(")
	text = strings.TrimSuffix(text, ")")
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "!") {
		return false
	}

	inner := strings.TrimSpace(text[1:])
	inner = strings.TrimPrefix(inner, "(")
	inner = strings.TrimSuffix(inner, ")")
	inner = strings.TrimSpace(inner)
	return inner == "BuildConfig.DEBUG" || strings.HasSuffix(inner, ".BuildConfig.DEBUG")
}

func containsLoggingCallFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}

	found := false
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if found || file.FlatType(candidate) != "call_expression" {
			return
		}
		if isLikelyLoggingCallFlat(file, candidate) {
			found = true
		}
	})
	return found
}

func isLikelyLoggingCallFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return false
	}

	switch receiver := flatReceiverNameFromCall(file, idx); receiver {
	case "Log", "Timber", "logger", "log", "Logger":
		switch flatCallExpressionName(file, idx) {
		case "v", "d", "i", "w", "e", "wtf", "trace", "debug", "info", "warn", "warning", "error":
			return true
		}
	}

	switch flatCallExpressionName(file, idx) {
	case "println", "print", "debug", "info", "warn", "warning", "error", "trace", "log",
		"logDebug", "logInfo", "logWarn", "logWarning", "logError", "logTrace":
		return true
	default:
		return false
	}
}

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

func isEnvironmentConfigCallName(name string) bool {
	if name == "" {
		return false
	}
	return strings.Contains(name, "Environment") || strings.Contains(name, "Config") || strings.Contains(name, "Env")
}

func hardcodedEnvironmentLiteralFlat(file *scanner.File, arg uint32) string {
	if file == nil || arg == 0 {
		return ""
	}

	text := strings.TrimSpace(file.FlatNodeText(arg))
	if label := flatValueArgumentLabel(file, arg); label != "" {
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

type sourceModuleGradleInfo struct {
	found         bool
	isApplication bool
	isLibrary     bool
}

var (
	sourceModuleGradleCacheMu sync.RWMutex
	sourceModuleGradleCache   = map[string]sourceModuleGradleInfo{}
)

func isAndroidLibrarySourceFile(sourcePath string) bool {
	info := lookupSourceModuleGradleInfo(sourcePath)
	return info.found && info.isLibrary && !info.isApplication
}

func lookupSourceModuleGradleInfo(sourcePath string) sourceModuleGradleInfo {
	dir := filepath.Dir(sourcePath)
	for i := 0; i < 10; i++ {
		if info, ok := tryReadSourceModuleGradleInfo(dir); ok {
			return info
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return sourceModuleGradleInfo{}
}

func tryReadSourceModuleGradleInfo(dir string) (sourceModuleGradleInfo, bool) {
	sourceModuleGradleCacheMu.RLock()
	if cached, ok := sourceModuleGradleCache[dir]; ok {
		sourceModuleGradleCacheMu.RUnlock()
		return cached, cached.found
	}
	sourceModuleGradleCacheMu.RUnlock()

	var path string
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
			break
		}
	}
	if path == "" {
		return sourceModuleGradleInfo{}, false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return sourceModuleGradleInfo{}, false
	}
	content := string(data)
	info := sourceModuleGradleInfo{
		found:         true,
		isApplication: strings.Contains(content, "com.android.application") || strings.Contains(content, "applicationId") || containsPluginSuffix(content, "application"),
		isLibrary:     strings.Contains(content, "com.android.library") || containsPluginSuffix(content, "library"),
	}

	sourceModuleGradleCacheMu.Lock()
	sourceModuleGradleCache[dir] = info
	sourceModuleGradleCacheMu.Unlock()
	return info, true
}

// ---------- New release-engineering rules ----------

// CommentedOutImportRule flags `// import ...` lines — a commented-out import
// is either dead or a half-done refactor.
type CommentedOutImportRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CommentedOutImportRule) Confidence() float64 { return 0.90 }

// commentedImportRe matches a commented-out Kotlin import body. Requires a
// dotted-path that conforms to Kotlin import syntax — bare prose like
// "import order matters" won't match. Optional trailing `.*` and `as` alias.
var commentedImportRe = regexp.MustCompile(
	`^import\s+[\p{L}_][\p{L}\p{N}_]*(?:\.[\p{L}_][\p{L}\p{N}_]*)*(?:\.\*)?(?:\s+as\s+[\p{L}_][\p{L}\p{N}_]*)?\s*;?\s*$`)

func (r *CommentedOutImportRule) checkNode(ctx *v2.Context) {
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
	ctx.EmitAt(file.FlatRow(ctx.Idx)+1, 1,
		"Commented-out import; remove it or restore it as a live import.")
}

// DebugToastInProductionRule flags Toast.makeText calls whose message literal
// starts with "debug", "test", or "wip" (case-insensitive).
type DebugToastInProductionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *DebugToastInProductionRule) Confidence() float64 { return 0.85 }

var debugToastPrefixRe = regexp.MustCompile(`(?i)^["'](debug|test|wip)(?:[^A-Za-z0-9]|$)`)

// MergeConflictMarkerLeftoverRule flags unresolved merge conflict markers.
type MergeConflictMarkerLeftoverRule struct {
	LineBase
	BaseRule
}

func (r *MergeConflictMarkerLeftoverRule) Confidence() float64 { return 0.99 }

func (r *MergeConflictMarkerLeftoverRule) check(ctx *v2.Context) {
	file := ctx.File
	if !strings.HasSuffix(file.Path, ".kt") && !strings.HasSuffix(file.Path, ".kts") &&
		!strings.HasSuffix(file.Path, ".java") {
		return
	}

	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<<<<<<<") || trimmed == "=======" || strings.HasPrefix(trimmed, ">>>>>>>") {
			col := strings.IndexAny(line, "<=>")
			ctx.Emit(r.Finding(file, i+1, col+1,
				"Unresolved merge conflict marker; resolve the conflict before committing."))
		}
	}
}

// PrintlnInProductionRule flags println/print/System.out.println/System.err.println
// in non-test files, outside a top-level fun main().
type PrintlnInProductionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *PrintlnInProductionRule) Confidence() float64 { return 0.85 }

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
		text := strings.Join(strings.Fields(file.FlatNodeText(callee)), "")
		return strings.HasPrefix(text, "System.out.") || strings.HasPrefix(text, "System.err.")
	}
	return receiver == ""
}

// PrintStackTraceInProductionRule flags e.printStackTrace() calls in non-test
// files that import a logging framework.
type PrintStackTraceInProductionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *PrintStackTraceInProductionRule) Confidence() float64 { return 0.85 }

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
	for _, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "import ") {
			if trimmed != "" && !strings.HasPrefix(trimmed, "package ") && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "/*") && !strings.HasPrefix(trimmed, "*") {
				break
			}
			continue
		}
		pkg := strings.TrimSpace(strings.TrimPrefix(trimmed, "import "))
		for _, prefix := range loggingImports {
			if strings.Contains(pkg, prefix) {
				return true
			}
		}
	}
	return false
}

// HardcodedLocalhostUrlRule flags URL literals containing localhost or 10.0.2.2
// in non-test non-debug source files.
type HardcodedLocalhostUrlRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence: tier-1 — we dispatch on string_literal and read the literal's
// exact content via the AST's string_content children. A URL never crosses
// into a raw-string template or an interpolated expression by accident; if
// the literal has such an expression child, we refuse to match (we cannot
// prove the runtime URL is `localhost`). No line scanning, no quote
// gymnastics.
func (r *HardcodedLocalhostUrlRule) Confidence() float64 { return 0.95 }

var localhostUrlRe = regexp.MustCompile(`^https?://(localhost|127\.0\.0\.1|10\.0\.2\.2)(:\d+)?(/.*)?$`)

// check runs per string_literal. The former implementation scanned
// file.Lines for a regex that matched the literal together with its
// surrounding quote characters — this was fragile across raw string
// forms, multi-line strings, and comment contexts. Now we inspect the
// AST node directly.
func (r *HardcodedLocalhostUrlRule) check(ctx *v2.Context) {
	file := ctx.File
	if !strings.HasSuffix(file.Path, ".kt") && !strings.HasSuffix(file.Path, ".kts") {
		return
	}
	if isTestFile(file.Path) || isDebugSourceFile(file.Path) {
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
	if !localhostUrlRe.MatchString(content) {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Hardcoded localhost URL in production source; use a build config or environment variable."))
}

func isDebugSourceFile(path string) bool {
	return strings.Contains(path, "/debug/") || strings.Contains(path, "/src/debug/")
}

// TestOnlyImportInProductionRule flags test-framework imports in non-test files.
type TestOnlyImportInProductionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestOnlyImportInProductionRule) Confidence() float64 { return 0.95 }

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
func (r *TestOnlyImportInProductionRule) check(ctx *v2.Context) {
	file := ctx.File
	if !strings.HasSuffix(file.Path, ".kt") && !strings.HasSuffix(file.Path, ".kts") {
		return
	}
	if isTestFile(file.Path) {
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

// NonAsciiIdentifierRule flags class/function/property names containing
// non-ASCII characters.
type NonAsciiIdentifierRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *NonAsciiIdentifierRule) Confidence() float64 { return 0.95 }

// HardcodedLogTagRule flags Log.d("ClassName", ...) where the tag matches
// the enclosing class name instead of using a companion TAG constant.
type HardcodedLogTagRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *HardcodedLogTagRule) Confidence() float64 { return 0.80 }

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

// VisibleForTestingCallerInNonTestRule flags calls to @VisibleForTesting
// functions from non-test files.
type VisibleForTestingCallerInNonTestRule struct {
	BaseRule
}

func (r *VisibleForTestingCallerInNonTestRule) Confidence() float64 { return 0.80 }
func (r *VisibleForTestingCallerInNonTestRule) check(ctx *v2.Context) {
	index := ctx.CodeIndex
	if index == nil {
		return
	}

	targetsByName := make(map[string][]visibleForTestingTarget)
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			if file.FlatType(idx) != "function_declaration" {
				return
			}
			if !flatHasAnnotationNamed(file, idx, "VisibleForTesting") {
				return
			}
			name := file.FlatChildTextOrEmpty(idx, "simple_identifier")
			if name == "" {
				return
			}
			targetsByName[name] = append(targetsByName[name], visibleForTestingTarget{
				name:    name,
				owner:   flatEnclosingOwnerName(file, idx),
				file:    file.Path,
				node:    idx,
				minArgs: flatFunctionMinArgumentCount(file, idx),
				maxArgs: flatFunctionMaxArgumentCount(file, idx),
			})
		})
	}

	if len(targetsByName) == 0 {
		return
	}

	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if isTestFile(file.Path) {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			if file.FlatType(idx) != "call_expression" {
				return
			}
			target, confidence, ok := resolveVisibleForTestingCallTarget(file, idx, targetsByName)
			if !ok {
				return
			}
			ctx.Emit(scanner.Finding{
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				Col:        file.FlatCol(idx) + 1,
				RuleSet:    r.RuleSetName,
				Rule:       r.RuleName,
				Severity:   r.Sev,
				Message:    fmt.Sprintf("@VisibleForTesting function %q called from non-test code.", target.name),
				Confidence: confidence,
			})
		})
	}
}

type visibleForTestingTarget struct {
	name    string
	owner   string
	file    string
	node    uint32
	minArgs int
	maxArgs int
}

func resolveVisibleForTestingCallTarget(file *scanner.File, call uint32, targetsByName map[string][]visibleForTestingTarget) (visibleForTestingTarget, float64, bool) {
	callTarget, ok := semantics.ResolveCallTarget(&v2.Context{File: file}, call)
	if !ok || callTarget.CalleeName == "" {
		return visibleForTestingTarget{}, 0, false
	}
	candidates := targetsByName[callTarget.CalleeName]
	if len(candidates) == 0 {
		return visibleForTestingTarget{}, 0, false
	}
	argCount := len(callTarget.Arguments)

	receiver := semantics.ReferenceName(file, callTarget.Receiver.Node)
	if receiver != "" {
		if target, ok := uniqueVisibleForTestingTarget(candidates, func(t visibleForTestingTarget) bool {
			return visibleForTestingArityMatches(t, argCount) &&
				(t.owner == receiver || strings.HasSuffix(t.owner, "."+receiver))
		}); ok {
			confidence, ok := semantics.ConfidenceForEvidence(0.85, semantics.EvidenceQualifiedReceiver)
			return target, confidence, ok
		}
	}

	return visibleForTestingTarget{}, 0, false
}

func visibleForTestingArityMatches(target visibleForTestingTarget, argCount int) bool {
	return argCount >= target.minArgs && argCount <= target.maxArgs
}

func uniqueVisibleForTestingTarget(candidates []visibleForTestingTarget, match func(visibleForTestingTarget) bool) (visibleForTestingTarget, bool) {
	var found visibleForTestingTarget
	count := 0
	for _, candidate := range candidates {
		if !match(candidate) {
			continue
		}
		found = candidate
		count++
		if count > 1 {
			return visibleForTestingTarget{}, false
		}
	}
	return found, count == 1
}

func flatFunctionMinArgumentCount(file *scanner.File, funcDecl uint32) int {
	count := 0
	params, ok := file.FlatFindChild(funcDecl, "function_value_parameters")
	if !ok {
		return 0
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "parameter" && !flatParameterHasDefaultValue(file, child) {
			count++
		}
	}
	return count
}

func flatFunctionMaxArgumentCount(file *scanner.File, funcDecl uint32) int {
	count := 0
	params, ok := file.FlatFindChild(funcDecl, "function_value_parameters")
	if !ok {
		return 0
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "parameter" {
			count++
		}
	}
	return count
}

func flatParameterHasDefaultValue(file *scanner.File, param uint32) bool {
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			return true
		}
	}
	return false
}

func flatCallArgumentCount(file *scanner.File, call uint32) int {
	count := 0
	args := flatCallKeyArguments(file, call)
	if args != 0 {
		for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
			if file.FlatType(arg) == "value_argument" {
				count++
			}
		}
	}
	if flatCallTrailingLambda(file, call) != 0 {
		count++
	}
	return count
}

// OpenForTestingCallerInNonTestRule flags subclasses of @OpenForTesting
// types outside test source sets.
type OpenForTestingCallerInNonTestRule struct {
	BaseRule
}

func (r *OpenForTestingCallerInNonTestRule) Confidence() float64 { return 0.75 }
func (r *OpenForTestingCallerInNonTestRule) check(ctx *v2.Context) {
	index := ctx.CodeIndex
	if index == nil {
		return
	}

	scopes := make(map[string]openForTestingFileScope, len(index.Files))
	targets := openForTestingTargetSet{
		byFQN:    make(map[string]openForTestingTarget),
		bySimple: make(map[string][]openForTestingTarget),
	}
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		scope := openForTestingScope(file)
		scopes[file.Path] = scope
		file.FlatWalkAllNodes(0, func(idx uint32) {
			nodeType := file.FlatType(idx)
			if nodeType != "class_declaration" && nodeType != "object_declaration" {
				return
			}
			if !openForTestingDeclarationHasAnnotation(file, idx, scope.imports) {
				return
			}
			target := openForTestingTargetForDeclaration(file, idx, scope.pkg)
			if target.simple == "" || target.fqn == "" {
				return
			}
			targets.byFQN[target.fqn] = target
			targets.bySimple[target.simple] = append(targets.bySimple[target.simple], target)
		})
	}

	if len(targets.byFQN) == 0 {
		return
	}

	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if isTestFile(file.Path) {
			continue
		}
		scope := scopes[file.Path]
		file.FlatWalkNodes(0, "class_declaration", func(classDecl uint32) {
			for child := file.FlatFirstChild(classDecl); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) != "delegation_specifier" {
					continue
				}
				ref := openForTestingSupertypeRef(file, child)
				if ref.simple == "" {
					continue
				}
				target, confidence, ok := resolveOpenForTestingSupertype(file, ref, scope, targets)
				if !ok {
					continue
				}
				ctx.Emit(scanner.Finding{
					File:       file.Path,
					Line:       file.FlatRow(ref.node) + 1,
					Col:        file.FlatCol(ref.node) + 1,
					RuleSet:    r.RuleSetName,
					Rule:       r.RuleName,
					Severity:   r.Sev,
					Message:    fmt.Sprintf("@OpenForTesting type %q subclassed outside test code.", target.simple),
					Confidence: confidence,
				})
			}
		})
	}
}

type openForTestingFileScope struct {
	pkg     string
	imports map[string]string
}

type openForTestingTarget struct {
	simple string
	fqn    string
	file   string
	node   uint32
}

type openForTestingTargetSet struct {
	byFQN    map[string]openForTestingTarget
	bySimple map[string][]openForTestingTarget
}

type openForTestingTypeRef struct {
	simple string
	path   []string
	node   uint32
}

func openForTestingScope(file *scanner.File) openForTestingFileScope {
	return openForTestingFileScope{
		pkg:     openForTestingPackageName(file),
		imports: openForTestingImports(file),
	}
}

func openForTestingTargetForDeclaration(file *scanner.File, decl uint32, pkg string) openForTestingTarget {
	name := openForTestingDeclarationName(file, decl)
	if name == "" {
		return openForTestingTarget{}
	}
	parts := make([]string, 0, 4)
	if pkg != "" {
		parts = append(parts, pkg)
	}
	if owner := flatEnclosingOwnerName(file, decl); owner != "" {
		parts = append(parts, strings.Split(owner, ".")...)
	}
	parts = append(parts, name)
	return openForTestingTarget{
		simple: name,
		fqn:    strings.Join(parts, "."),
		file:   file.Path,
		node:   decl,
	}
}

func openForTestingDeclarationName(file *scanner.File, decl uint32) string {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "simple_identifier":
			return file.FlatNodeString(child, nil)
		}
	}
	return ""
}

func openForTestingDeclarationHasAnnotation(file *scanner.File, decl uint32, imports map[string]string) bool {
	if mods, ok := file.FlatFindChild(decl, "modifiers"); ok {
		for child := file.FlatFirstChild(mods); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "annotation" && openForTestingAnnotationMatches(file, child, imports) {
				return true
			}
		}
	}
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "annotation" && openForTestingAnnotationMatches(file, child, imports) {
			return true
		}
	}
	return false
}

func openForTestingAnnotationMatches(file *scanner.File, annotation uint32, imports map[string]string) bool {
	ref := openForTestingTypeRefFromNode(file, annotation)
	if ref.simple == "" {
		return false
	}
	if ref.simple == "OpenForTesting" {
		return true
	}
	fqn := imports[ref.simple]
	return fqn == "OpenForTesting" || strings.HasSuffix(fqn, ".OpenForTesting")
}

func openForTestingSupertypeRef(file *scanner.File, spec uint32) openForTestingTypeRef {
	return openForTestingTypeRefFromNode(file, spec)
}

func openForTestingTypeRefFromNode(file *scanner.File, root uint32) openForTestingTypeRef {
	userType := openForTestingFirstUserType(file, root)
	if userType == 0 {
		return openForTestingTypeRef{}
	}
	path, nameNode := openForTestingUserTypePath(file, userType)
	if len(path) == 0 || nameNode == 0 {
		return openForTestingTypeRef{}
	}
	return openForTestingTypeRef{
		simple: path[len(path)-1],
		path:   path,
		node:   nameNode,
	}
}

func openForTestingFirstUserType(file *scanner.File, root uint32) uint32 {
	if file == nil || file.FlatTree == nil || int(root) >= len(file.FlatTree.Nodes) {
		return 0
	}
	if file.FlatType(root) == "user_type" {
		return root
	}
	for child := file.FlatFirstChild(root); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "type_arguments" {
			continue
		}
		if found := openForTestingFirstUserType(file, child); found != 0 {
			return found
		}
	}
	return 0
}

func openForTestingUserTypePath(file *scanner.File, userType uint32) ([]string, uint32) {
	var path []string
	var nameNode uint32
	var walk func(uint32)
	walk = func(idx uint32) {
		if file.FlatType(idx) == "type_arguments" {
			return
		}
		switch file.FlatType(idx) {
		case "type_identifier", "simple_identifier":
			path = append(path, file.FlatNodeString(idx, nil))
			nameNode = idx
			return
		}
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			walk(child)
		}
	}
	walk(userType)
	return path, nameNode
}

func resolveOpenForTestingSupertype(file *scanner.File, ref openForTestingTypeRef, scope openForTestingFileScope, targets openForTestingTargetSet) (openForTestingTarget, float64, bool) {
	if len(ref.path) > 1 {
		qualified := strings.Join(ref.path, ".")
		if target, ok := targets.byFQN[qualified]; ok {
			return target, 0.90, true
		}
		if scope.pkg != "" {
			if target, ok := targets.byFQN[scope.pkg+"."+qualified]; ok {
				return target, 0.85, true
			}
		}
	}

	if fqn := scope.imports[ref.simple]; fqn != "" {
		if target, ok := targets.byFQN[fqn]; ok {
			return target, 0.90, true
		}
	}
	if scope.pkg != "" {
		if target, ok := targets.byFQN[scope.pkg+"."+ref.simple]; ok {
			return target, 0.85, true
		}
	}

	candidates := targets.bySimple[ref.simple]
	if len(candidates) == 0 {
		return openForTestingTarget{}, 0, false
	}
	var found openForTestingTarget
	count := 0
	refOwner := openForTestingInheritanceOwner(file, ref.node)
	for _, candidate := range candidates {
		if candidate.file != file.Path || flatEnclosingOwnerName(file, candidate.node) != refOwner {
			continue
		}
		found = candidate
		count++
		if count > 1 {
			return openForTestingTarget{}, 0, false
		}
	}
	if count == 1 {
		return found, 0.80, true
	}
	return openForTestingTarget{}, 0, false
}

func openForTestingInheritanceOwner(file *scanner.File, ref uint32) string {
	classDecl, ok := flatEnclosingAncestor(file, ref, "class_declaration")
	if !ok {
		return flatEnclosingOwnerName(file, ref)
	}
	return flatEnclosingOwnerName(file, classDecl)
}

func openForTestingPackageName(file *scanner.File) string {
	var parts []string
	for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "package_header" {
			continue
		}
		openForTestingCollectImportPath(file, child, false, &parts, nil)
		break
	}
	return strings.Join(parts, ".")
}

func openForTestingImports(file *scanner.File) map[string]string {
	imports := make(map[string]string)
	for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "import_header":
			openForTestingAddImport(file, child, imports)
		case "import_list":
			for imp := file.FlatFirstChild(child); imp != 0; imp = file.FlatNextSib(imp) {
				if file.FlatType(imp) == "import_header" {
					openForTestingAddImport(file, imp, imports)
				}
			}
		}
	}
	return imports
}

func openForTestingAddImport(file *scanner.File, importHeader uint32, imports map[string]string) {
	var path []string
	var alias string
	openForTestingCollectImportPath(file, importHeader, false, &path, &alias)
	if len(path) == 0 {
		return
	}
	fqn := strings.Join(path, ".")
	key := path[len(path)-1]
	if alias != "" {
		key = alias
	}
	imports[key] = fqn
}

func openForTestingCollectImportPath(file *scanner.File, idx uint32, afterAs bool, path *[]string, alias *string) {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		childType := file.FlatType(child)
		switch childType {
		case "as":
			afterAs = true
			continue
		case "simple_identifier", "type_identifier":
			name := file.FlatNodeString(child, nil)
			if afterAs && alias != nil {
				*alias = name
			} else {
				*path = append(*path, name)
			}
			continue
		}
		openForTestingCollectImportPath(file, child, afterAs, path, alias)
	}
}

// TestFixtureAccessedFromProductionRule flags usage of types declared under
// src/testFixtures/ from non-test files.
type TestFixtureAccessedFromProductionRule struct {
	BaseRule
}

func (r *TestFixtureAccessedFromProductionRule) Confidence() float64 { return 0.80 }

type testFixtureDeclaration struct {
	qualifiedName string
}

func (r *TestFixtureAccessedFromProductionRule) check(ctx *v2.Context) {
	index := ctx.CodeIndex
	if index == nil {
		return
	}

	parsedFiles := ctx.ParsedFiles
	if len(parsedFiles) == 0 {
		parsedFiles = index.Files
	}
	if len(parsedFiles) == 0 {
		return
	}

	filesByPath := make(map[string]*scanner.File, len(parsedFiles))
	packageByPath := make(map[string]string, len(parsedFiles))
	for _, file := range parsedFiles {
		if file == nil {
			continue
		}
		filesByPath[file.Path] = file
		packageByPath[file.Path] = sourcePackageName(file)
	}

	fixturesByQualified := make(map[string]testFixtureDeclaration)
	productionDeclarations := make(map[string]bool)
	for _, sym := range index.Symbols {
		if !isClassLikeFixtureSymbol(sym) {
			continue
		}
		file := filesByPath[sym.File]
		if file == nil {
			continue
		}
		packageName := packageByPath[sym.File]
		qualifiedName := qualifySourceName(packageName, sym.Name)
		if isTestFixturePath(sym.File) {
			if isNestedClassLikeSymbol(file, sym) {
				continue
			}
			fixturesByQualified[qualifiedName] = testFixtureDeclaration{
				qualifiedName: qualifiedName,
			}
			continue
		}
		productionDeclarations[qualifiedName] = true
	}
	for _, file := range parsedFiles {
		if file == nil || isTestFixturePath(file.Path) || isTestFile(filepath.ToSlash(file.Path)) {
			continue
		}
		for _, name := range topLevelClassLikeNames(file) {
			productionDeclarations[qualifySourceName(packageByPath[file.Path], name)] = true
		}
	}

	if len(fixturesByQualified) == 0 {
		return
	}

	emitted := make(map[string]bool)
	for _, file := range parsedFiles {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if isTestFile(filepath.ToSlash(file.Path)) || isTestFixturePath(file.Path) || isGeneratedSourcePath(file.Path) {
			continue
		}

		imports, importedNames := fixtureImportBindings(file, fixturesByQualified)
		for _, binding := range imports {
			if productionDeclarations[binding.decl.qualifiedName] {
				continue
			}
			r.emitTestFixtureAccess(ctx, file, binding.node, binding.decl, 0.95, emitted)
		}

		packageName := packageByPath[file.Path]
		fileCtx := &v2.Context{File: file, Resolver: ctx.Resolver, CodeIndex: index}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			decl, confidence, ok := resolveFixtureAccessNode(fileCtx, idx, packageName, fixturesByQualified, imports, importedNames, productionDeclarations)
			if !ok {
				return
			}
			r.emitTestFixtureAccess(ctx, file, idx, decl, confidence, emitted)
		})
	}
}

func resolveFixtureAccessNode(ctx *v2.Context, idx uint32, packageName string, fixtures map[string]testFixtureDeclaration, imports map[string]fixtureImportBinding, importedNames map[string]string, productionDeclarations map[string]bool) (testFixtureDeclaration, float64, bool) {
	if ctx == nil || ctx.File == nil {
		return testFixtureDeclaration{}, 0, false
	}
	file := ctx.File
	switch file.FlatType(idx) {
	case "call_expression":
		return resolveFixtureCall(ctx, idx, packageName, fixtures, imports, importedNames, productionDeclarations)
	case "navigation_expression":
		if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) == "call_expression" {
			return testFixtureDeclaration{}, 0, false
		}
		return resolveFixtureNavigation(ctx, idx, packageName, fixtures, imports, importedNames, productionDeclarations)
	case "user_type", "object_creation_expression":
		return resolveFixtureTypeLikeNode(ctx, idx, packageName, fixtures, imports, importedNames, productionDeclarations)
	case "type_identifier":
		if file.Language != scanner.LangJava || isDeclarationIdentifier(file, idx) || hasFlatAncestorTypeName(file, idx, "import_declaration", "package_declaration") {
			return testFixtureDeclaration{}, 0, false
		}
		return resolveFixtureTypeLikeNode(ctx, idx, packageName, fixtures, imports, importedNames, productionDeclarations)
	}
	return testFixtureDeclaration{}, 0, false
}

func resolveFixtureCall(ctx *v2.Context, call uint32, packageName string, fixtures map[string]testFixtureDeclaration, imports map[string]fixtureImportBinding, importedNames map[string]string, productionDeclarations map[string]bool) (testFixtureDeclaration, float64, bool) {
	if decl, ok := fixtureByQualifiedReferenceText(ctx.File, call, fixtures); ok {
		if productionDeclarations[decl.qualifiedName] {
			return testFixtureDeclaration{}, 0, false
		}
		return decl, 0.95, true
	}
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok {
		return testFixtureDeclaration{}, 0, false
	}
	if target.Resolved {
		if decl, ok := fixtures[target.QualifiedName]; ok {
			if productionDeclarations[decl.qualifiedName] {
				return testFixtureDeclaration{}, 0, false
			}
			return decl, 0.95, true
		}
	}
	if target.Receiver.Valid() {
		if decl, confidence, ok := resolveFixtureName(ctx, semantics.ReferenceName(ctx.File, target.Receiver.Node), target.Receiver.Node, packageName, fixtures, imports, importedNames, productionDeclarations); ok {
			return decl, confidence, true
		}
	}
	return resolveFixtureName(ctx, target.CalleeName, call, packageName, fixtures, imports, importedNames, productionDeclarations)
}

func resolveFixtureNavigation(ctx *v2.Context, nav uint32, packageName string, fixtures map[string]testFixtureDeclaration, imports map[string]fixtureImportBinding, importedNames map[string]string, productionDeclarations map[string]bool) (testFixtureDeclaration, float64, bool) {
	if decl, ok := fixtureByQualifiedReferenceText(ctx.File, nav, fixtures); ok {
		if productionDeclarations[decl.qualifiedName] {
			return testFixtureDeclaration{}, 0, false
		}
		return decl, 0.95, true
	}
	receiver := firstReferenceIdentifier(ctx.File, nav)
	return resolveFixtureName(ctx, receiver, nav, packageName, fixtures, imports, importedNames, productionDeclarations)
}

func resolveFixtureTypeLikeNode(ctx *v2.Context, idx uint32, packageName string, fixtures map[string]testFixtureDeclaration, imports map[string]fixtureImportBinding, importedNames map[string]string, productionDeclarations map[string]bool) (testFixtureDeclaration, float64, bool) {
	if hasFlatAncestorTypeName(ctx.File, idx, "line_comment", "multiline_comment", "string_literal", "raw_string_literal", "character_literal") {
		return testFixtureDeclaration{}, 0, false
	}
	if decl, ok := fixtureByQualifiedReferenceText(ctx.File, idx, fixtures); ok {
		if productionDeclarations[decl.qualifiedName] {
			return testFixtureDeclaration{}, 0, false
		}
		return decl, 0.95, true
	}
	if ctx.Resolver != nil {
		if typ := ctx.Resolver.ResolveFlatNode(idx, ctx.File); typ != nil {
			if decl, ok := fixtures[typ.FQN]; ok {
				if productionDeclarations[decl.qualifiedName] {
					return testFixtureDeclaration{}, 0, false
				}
				return decl, 0.95, true
			}
		}
	}
	return resolveFixtureName(ctx, lastReferenceIdentifier(ctx.File, idx), idx, packageName, fixtures, imports, importedNames, productionDeclarations)
}

func resolveFixtureName(ctx *v2.Context, name string, ref uint32, packageName string, fixtures map[string]testFixtureDeclaration, imports map[string]fixtureImportBinding, importedNames map[string]string, productionDeclarations map[string]bool) (testFixtureDeclaration, float64, bool) {
	if name == "" {
		return testFixtureDeclaration{}, 0, false
	}
	if binding, ok := imports[name]; ok {
		if productionDeclarations[binding.decl.qualifiedName] {
			return testFixtureDeclaration{}, 0, false
		}
		return binding.decl, 0.95, true
	}
	if ctx.Resolver != nil {
		if imported := ctx.Resolver.ResolveImport(name, ctx.File); imported != "" {
			if decl, ok := fixtures[imported]; ok {
				if productionDeclarations[decl.qualifiedName] {
					return testFixtureDeclaration{}, 0, false
				}
				return decl, 0.95, true
			}
			return testFixtureDeclaration{}, 0, false
		}
	}
	if imported, ok := importedNames[name]; ok {
		if _, isFixtureImport := fixtures[imported]; !isFixtureImport {
			return testFixtureDeclaration{}, 0, false
		}
	}

	qualifiedName := qualifySourceName(packageName, name)
	decl, ok := fixtures[qualifiedName]
	if !ok {
		return testFixtureDeclaration{}, 0, false
	}
	if productionDeclarations[qualifiedName] {
		return testFixtureDeclaration{}, 0, false
	}
	if sameFileClassLikeDeclarationShadows(ctx, name, ref) {
		return testFixtureDeclaration{}, 0, false
	}
	if !samePackageReferenceContext(ctx.File, ref) {
		return testFixtureDeclaration{}, 0, false
	}
	return decl, 0.80, true
}

func sameFileClassLikeDeclarationShadows(ctx *v2.Context, name string, ref uint32) bool {
	if ctx == nil || ctx.File == nil || name == "" {
		return false
	}
	file := ctx.File
	var shadowed bool
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if shadowed {
			return
		}
		switch file.FlatType(idx) {
		case "class_declaration", "object_declaration", "interface_declaration":
			if isNestedClassLikeNode(file, idx) && !semantics.SameEnclosingOwner(file, idx, ref) {
				return
			}
			if firstChildText(file, idx, "type_identifier", "simple_identifier", "identifier") == name &&
				semantics.SameFileDeclarationMatch(ctx, idx, ref) {
				shadowed = true
			}
		}
	})
	return shadowed
}

func samePackageReferenceContext(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "call_expression", "user_type", "type_identifier", "object_creation_expression":
		return true
	case "navigation_expression":
		return firstReferenceIdentifier(file, idx) != ""
	default:
		return false
	}
}

func firstReferenceIdentifier(file *scanner.File, idx uint32) string {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "simple_identifier", "type_identifier", "identifier":
			return file.FlatNodeText(child)
		case "scoped_identifier", "scoped_type_identifier", "navigation_expression":
			if name := firstReferenceIdentifier(file, child); name != "" {
				return name
			}
		}
	}
	return ""
}

func lastReferenceIdentifier(file *scanner.File, idx uint32) string {
	last := ""
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		switch file.FlatType(candidate) {
		case "simple_identifier", "type_identifier", "identifier":
			if !isDeclarationIdentifier(file, candidate) {
				last = file.FlatNodeText(candidate)
			}
		}
	})
	if last != "" {
		return last
	}
	if file.FlatType(idx) == "simple_identifier" || file.FlatType(idx) == "type_identifier" || file.FlatType(idx) == "identifier" {
		return file.FlatNodeText(idx)
	}
	return ""
}

type fixtureImportBinding struct {
	node uint32
	decl testFixtureDeclaration
}

func (r *TestFixtureAccessedFromProductionRule) emitTestFixtureAccess(ctx *v2.Context, file *scanner.File, idx uint32, decl testFixtureDeclaration, confidence float64, emitted map[string]bool) {
	line := file.FlatRow(idx) + 1
	col := file.FlatCol(idx) + 1
	key := fmt.Sprintf("%s:%d:%d:%s", file.Path, line, col, decl.qualifiedName)
	if emitted[key] {
		return
	}
	emitted[key] = true
	ctx.Emit(scanner.Finding{
		File:       file.Path,
		Line:       line,
		Col:        col,
		RuleSet:    r.RuleSetName,
		Rule:       r.RuleName,
		Severity:   r.Sev,
		Message:    fmt.Sprintf("Test fixture type %q from testFixtures/ used in production code.", decl.qualifiedName),
		Confidence: confidence,
	})
}

func fixtureImportBindings(file *scanner.File, fixtures map[string]testFixtureDeclaration) (map[string]fixtureImportBinding, map[string]string) {
	bindings := make(map[string]fixtureImportBinding)
	importedNames := make(map[string]string)
	file.FlatWalkAllNodes(0, func(idx uint32) {
		nodeType := file.FlatType(idx)
		if nodeType != "import_header" && nodeType != "import_declaration" {
			return
		}
		qualifiedName, localName, wildcard := parseSourceImport(file.FlatNodeText(idx))
		if qualifiedName == "" || wildcard {
			return
		}
		importedNames[localName] = qualifiedName
		if decl, ok := fixtures[qualifiedName]; ok {
			bindings[localName] = fixtureImportBinding{
				node: idx,
				decl: decl,
			}
		}
	})
	return bindings, importedNames
}

func fixtureByQualifiedReferenceText(file *scanner.File, idx uint32, fixtures map[string]testFixtureDeclaration) (testFixtureDeclaration, bool) {
	for current, ok := idx, true; ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "user_type", "navigation_expression", "call_expression", "object_creation_expression", "scoped_identifier", "scoped_type_identifier", "field_access", "method_invocation":
			text := compactSourceReference(file.FlatNodeText(current))
			for qualifiedName, decl := range fixtures {
				if text == qualifiedName ||
					strings.HasPrefix(text, qualifiedName+"<") ||
					strings.HasPrefix(text, qualifiedName+"(") ||
					strings.HasPrefix(text, qualifiedName+".") {
					return decl, true
				}
			}
		}
	}
	return testFixtureDeclaration{}, false
}

func compactSourceReference(text string) string {
	text = strings.TrimSpace(text)
	replacer := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "")
	return replacer.Replace(text)
}

func isDeclarationIdentifier(file *scanner.File, idx uint32) bool {
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "class_declaration", "object_declaration", "function_declaration", "variable_declaration", "function_value_parameter",
		"interface_declaration", "method_declaration", "constructor_declaration", "variable_declarator", "formal_parameter":
		return true
	}
	return false
}

func topLevelClassLikeNames(file *scanner.File) []string {
	var names []string
	file.FlatWalkAllNodes(0, func(idx uint32) {
		switch file.FlatType(idx) {
		case "class_declaration", "object_declaration", "interface_declaration":
			if isNestedClassLikeNode(file, idx) {
				return
			}
			if name := firstChildText(file, idx, "type_identifier", "simple_identifier", "identifier"); name != "" {
				names = append(names, name)
			}
		}
	})
	return names
}

func firstChildText(file *scanner.File, idx uint32, nodeTypes ...string) string {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		childType := file.FlatType(child)
		for _, nodeType := range nodeTypes {
			if childType == nodeType {
				return file.FlatNodeText(child)
			}
		}
	}
	return ""
}

func sourcePackageName(file *scanner.File) string {
	if file == nil || file.FlatTree == nil {
		return ""
	}
	var packageName string
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if packageName != "" {
			return
		}
		nodeType := file.FlatType(idx)
		if nodeType != "package_header" && nodeType != "package_declaration" {
			return
		}
		text := strings.TrimSpace(file.FlatNodeText(idx))
		text = strings.TrimPrefix(text, "package")
		text = strings.TrimSpace(strings.TrimSuffix(text, ";"))
		packageName = text
	})
	return packageName
}

func parseSourceImport(text string) (qualifiedName string, localName string, wildcard bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "import") {
		return "", "", false
	}
	body := strings.TrimSpace(strings.TrimPrefix(text, "import"))
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

func isClassLikeFixtureSymbol(sym scanner.Symbol) bool {
	return sym.Name != "" && (sym.Kind == "class" || sym.Kind == "interface" || sym.Kind == "object")
}

func isNestedClassLikeSymbol(file *scanner.File, sym scanner.Symbol) bool {
	if file == nil || sym.StartByte < 0 || sym.EndByte <= sym.StartByte {
		return false
	}
	idx, ok := file.FlatNamedDescendantForByteRange(uint32(sym.StartByte), uint32(sym.EndByte))
	if !ok {
		return false
	}
	return isNestedClassLikeNode(file, idx)
}

func isNestedClassLikeNode(file *scanner.File, idx uint32) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "class_declaration", "object_declaration", "interface_declaration":
			return true
		}
	}
	return false
}

func hasFlatAncestorTypeName(file *scanner.File, idx uint32, nodeTypes ...string) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		currentType := file.FlatType(current)
		for _, nodeType := range nodeTypes {
			if currentType == nodeType {
				return true
			}
		}
	}
	return false
}

func qualifySourceName(packageName, name string) string {
	if packageName == "" {
		return name
	}
	return packageName + "." + name
}

func simpleSourceName(qualifiedName string) string {
	if idx := strings.LastIndex(qualifiedName, "."); idx >= 0 {
		return qualifiedName[idx+1:]
	}
	return qualifiedName
}

func isTestFixturePath(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/testFixtures/")
}

func isGeneratedSourcePath(path string) bool {
	path = filepath.ToSlash(path)
	return strings.Contains(path, "/build/generated/") ||
		strings.Contains(path, "/generated/") ||
		strings.Contains(path, "/ksp/") ||
		strings.Contains(path, "/kapt/")
}

// TimberTreeNotPlantedRule flags projects that use Timber.d/i/w/e but have
// no Timber.plant() call reachable from Application.onCreate.
type TimberTreeNotPlantedRule struct {
	BaseRule
}

func (r *TimberTreeNotPlantedRule) Confidence() float64 { return 0.75 }
func (r *TimberTreeNotPlantedRule) check(ctx *v2.Context) {
	files := timberProjectFiles(ctx)
	if len(files) == 0 {
		return
	}

	var firstUsage *timberCallSite
	hasStartupPlant := false

	for _, file := range files {
		if file == nil || file.FlatTree == nil || isTestFile(file.Path) {
			continue
		}
		imports := timberImportsForFile(file)
		file.FlatWalkNodes(0, "call_expression", func(call uint32) {
			site, ok := resolveTimberCall(ctx, file, call, imports)
			if !ok {
				return
			}
			if site.kind == timberCallPlant {
				if timberPlantInApplicationOnCreate(ctx, file, call) {
					hasStartupPlant = true
				}
				return
			}
			if firstUsage == nil || site.before(*firstUsage) {
				firstUsage = &site
			}
		})
	}

	if firstUsage == nil || hasStartupPlant {
		return
	}

	ctx.Emit(scanner.Finding{
		File:       firstUsage.file.Path,
		Line:       firstUsage.line,
		Col:        firstUsage.col,
		RuleSet:    r.RuleSetName,
		Rule:       r.RuleName,
		Severity:   r.Sev,
		Message:    "Timber is used but Timber.plant() is not called from Application.onCreate; logs will be silently dropped.",
		Confidence: firstUsage.confidence,
	})
}

type timberCallKind int

const (
	timberCallUsage timberCallKind = iota
	timberCallPlant
)

type timberCallSite struct {
	kind       timberCallKind
	file       *scanner.File
	call       uint32
	line       int
	col        int
	confidence float64
}

func (s timberCallSite) before(other timberCallSite) bool {
	if s.file.Path != other.file.Path {
		return s.file.Path < other.file.Path
	}
	if s.line != other.line {
		return s.line < other.line
	}
	return s.col < other.col
}

var timberLoggingCallees = map[string]bool{
	"v":   true,
	"d":   true,
	"i":   true,
	"w":   true,
	"e":   true,
	"wtf": true,
}

type timberImportInfo struct {
	receivers map[string]bool
	members   map[string]bool
	wildcard  bool
}

func timberProjectFiles(ctx *v2.Context) []*scanner.File {
	if ctx == nil {
		return nil
	}
	if ctx.CodeIndex != nil && len(ctx.CodeIndex.Files) > 0 {
		return ctx.CodeIndex.Files
	}
	return ctx.ParsedFiles
}

func timberImportsForFile(file *scanner.File) timberImportInfo {
	info := timberImportInfo{
		receivers: map[string]bool{"timber.log.Timber": true},
		members:   make(map[string]bool),
	}
	if file == nil || file.FlatTree == nil {
		return info
	}
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		text := strings.TrimSpace(file.FlatNodeText(node))
		text = strings.TrimSpace(strings.TrimPrefix(text, "import"))
		if text == "" {
			return
		}
		path := text
		alias := ""
		if idx := strings.Index(path, " as "); idx >= 0 {
			alias = strings.TrimSpace(path[idx+4:])
			path = strings.TrimSpace(path[:idx])
		}
		switch {
		case path == "timber.log.Timber":
			name := "Timber"
			if alias != "" {
				name = alias
			}
			info.receivers[name] = true
		case strings.HasPrefix(path, "timber.log.Timber."):
			member := strings.TrimPrefix(path, "timber.log.Timber.")
			if member == "*" {
				info.wildcard = true
				return
			}
			if alias != "" {
				member = alias
			}
			info.members[member] = true
		}
	})
	return info
}

func resolveTimberCall(parent *v2.Context, file *scanner.File, call uint32, imports timberImportInfo) (timberCallSite, bool) {
	localCtx := &v2.Context{File: file, Resolver: parent.Resolver, CodeIndex: parent.CodeIndex}
	target, ok := semantics.ResolveCallTarget(localCtx, call)
	if !ok {
		return timberCallSite{}, false
	}

	if target.Resolved {
		if kind, ok := resolvedTimberCallKind(target.QualifiedName); ok {
			return newTimberCallSite(file, call, kind, 0.9), true
		}
		return timberCallSite{}, false
	}

	kind, calleeOK := timberCallKindForName(target.CalleeName)
	if !calleeOK {
		return timberCallSite{}, false
	}

	if target.Receiver.Valid() {
		receiver := timberQualifiedPath(file, target.Receiver.Node)
		if timberReceiverMatches(receiver, imports) {
			return newTimberCallSite(file, call, kind, 0.7), true
		}
		return timberCallSite{}, false
	}

	if imports.wildcard || imports.members[target.CalleeName] {
		return newTimberCallSite(file, call, kind, 0.7), true
	}
	return timberCallSite{}, false
}

func newTimberCallSite(file *scanner.File, call uint32, kind timberCallKind, confidence float64) timberCallSite {
	return timberCallSite{
		kind:       kind,
		file:       file,
		call:       call,
		line:       file.FlatRow(call) + 1,
		col:        file.FlatCol(call) + 1,
		confidence: confidence,
	}
}

func timberCallKindForName(name string) (timberCallKind, bool) {
	if name == "plant" {
		return timberCallPlant, true
	}
	if timberLoggingCallees[name] {
		return timberCallUsage, true
	}
	return timberCallUsage, false
}

func resolvedTimberCallKind(qualifiedName string) (timberCallKind, bool) {
	name := strings.TrimSpace(qualifiedName)
	if paren := strings.Index(name, "("); paren >= 0 {
		name = name[:paren]
	}
	name = strings.ReplaceAll(name, "#", ".")
	if !strings.HasPrefix(name, "timber.log.Timber.") {
		return timberCallUsage, false
	}
	return timberCallKindForName(timberSimpleName(name))
}

func timberReceiverMatches(receiver string, imports timberImportInfo) bool {
	if receiver == "" {
		return false
	}
	if receiver == "timber.log.Timber" || imports.receivers[receiver] {
		return true
	}
	return false
}

func timberPlantInApplicationOnCreate(ctx *v2.Context, file *scanner.File, call uint32) bool {
	fn, ok := timberEnclosingAncestor(file, call, "function_declaration")
	if !ok || semantics.DeclarationName(file, fn) != "onCreate" || !file.FlatHasModifier(fn, "override") {
		return false
	}
	cls, ok := timberEnclosingAncestor(file, fn, "class_declaration", "object_declaration")
	if !ok {
		return false
	}
	return timberClassExtendsApplication(file, cls, ctx.Resolver, make(map[uint32]bool))
}

func timberClassExtendsApplication(file *scanner.File, classNode uint32, resolver typeinfer.TypeResolver, seen map[uint32]bool) bool {
	if classNode == 0 || seen[classNode] {
		return false
	}
	seen[classNode] = true

	localDecls := androidSameFileClassDeclarations(file)
	for _, super := range androidDirectSupertypesFlat(file, classNode) {
		if super.simple == "" {
			continue
		}
		if timberSupertypeIsLocalApplication(file, super, classNode, resolver, localDecls, seen) {
			return true
		}
		if timberSupertypeIsAndroidApplication(file, super, resolver) {
			return true
		}
	}
	return false
}

func timberSupertypeIsLocalApplication(file *scanner.File, super androidSupertypeRef, classNode uint32, resolver typeinfer.TypeResolver, localDecls map[string][]androidClassDecl, seen map[uint32]bool) bool {
	for _, decl := range localDecls[super.simple] {
		if decl.idx == classNode {
			continue
		}
		if timberClassExtendsApplication(file, decl.idx, resolver, seen) {
			return true
		}
		if super.simple == "Application" {
			return false
		}
	}
	return false
}

func timberSupertypeIsAndroidApplication(file *scanner.File, super androidSupertypeRef, resolver typeinfer.TypeResolver) bool {
	if super.name == "android.app.Application" {
		return true
	}
	if resolver != nil {
		name := super.name
		if !super.qualified {
			if fqn := resolver.ResolveImport(super.simple, file); fqn != "" {
				name = fqn
			}
		}
		if name == "android.app.Application" {
			return true
		}
		if info := resolver.ClassHierarchy(name); info != nil {
			for _, parent := range info.Supertypes {
				if parent == "android.app.Application" {
					return true
				}
			}
		}
	}
	return super.simple == "Application" && !super.qualified
}

func timberQualifiedPath(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	switch file.FlatType(idx) {
	case "simple_identifier", "type_identifier":
		return file.FlatNodeText(idx)
	case "navigation_expression":
		var parts []string
		file.FlatWalkAllNodes(idx, func(node uint32) {
			switch file.FlatType(node) {
			case "simple_identifier", "type_identifier":
				parts = append(parts, file.FlatNodeText(node))
			}
		})
		return strings.Join(parts, ".")
	case "call_expression":
		target, ok := semantics.ResolveCallTarget(&v2.Context{File: file}, idx)
		if ok && target.Receiver.Valid() {
			return timberQualifiedPath(file, target.Receiver.Node)
		}
	}
	return ""
}

func timberEnclosingAncestor(file *scanner.File, idx uint32, types ...string) (uint32, bool) {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		t := file.FlatType(p)
		for _, want := range types {
			if t == want {
				return p, true
			}
		}
	}
	return 0, false
}

func timberSimpleName(name string) string {
	name = strings.TrimSpace(name)
	if dot := strings.LastIndexAny(name, ".#"); dot >= 0 {
		return name[dot+1:]
	}
	return name
}
