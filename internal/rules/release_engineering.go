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
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
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
	LineBase
	BaseRule
}

func (r *CommentedOutImportRule) Confidence() float64 { return 0.90 }

func (r *CommentedOutImportRule) check(ctx *v2.Context) {
	file := ctx.File
	if !strings.HasSuffix(file.Path, ".kt") && !strings.HasSuffix(file.Path, ".kts") {
		return
	}

	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "//") {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
		if !strings.HasPrefix(body, "import ") {
			continue
		}
		col := strings.Index(line, "//")
		ctx.Emit(r.Finding(file, i+1, col+1,
			"Commented-out import; remove it or restore it as a live import."))
	}
}

// DebugToastInProductionRule flags Toast.makeText calls whose message literal
// starts with "debug", "test", or "wip" (case-insensitive).
type DebugToastInProductionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *DebugToastInProductionRule) Confidence() float64 { return 0.85 }

var debugToastPrefixRe = regexp.MustCompile(`(?i)^["'](debug|test|wip)`)

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
	LineBase
	BaseRule
}

func (r *HardcodedLocalhostUrlRule) Confidence() float64 { return 0.85 }

var localhostUrlRe = regexp.MustCompile(`"https?://(localhost|127\.0\.0\.1|10\.0\.2\.2)(:\d+)?(/[^"]*)?"|'https?://(localhost|127\.0\.0\.1|10\.0\.2\.2)(:\d+)?(/[^']*)?'`)

func (r *HardcodedLocalhostUrlRule) check(ctx *v2.Context) {
	file := ctx.File
	if !strings.HasSuffix(file.Path, ".kt") && !strings.HasSuffix(file.Path, ".kts") {
		return
	}
	if isTestFile(file.Path) {
		return
	}
	if isDebugSourceFile(file.Path) {
		return
	}

	for i, line := range file.Lines {
		loc := localhostUrlRe.FindStringIndex(line)
		if loc == nil {
			continue
		}
		ctx.Emit(r.Finding(file, i+1, loc[0]+1,
			"Hardcoded localhost URL in production source; use a build config or environment variable."))
	}
}

func isDebugSourceFile(path string) bool {
	return strings.Contains(path, "/debug/") || strings.Contains(path, "/src/debug/")
}

// TestOnlyImportInProductionRule flags test-framework imports in non-test files.
type TestOnlyImportInProductionRule struct {
	LineBase
	BaseRule
}

func (r *TestOnlyImportInProductionRule) Confidence() float64 { return 0.90 }

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

func (r *TestOnlyImportInProductionRule) check(ctx *v2.Context) {
	file := ctx.File
	if !strings.HasSuffix(file.Path, ".kt") && !strings.HasSuffix(file.Path, ".kts") {
		return
	}
	if isTestFile(file.Path) {
		return
	}

	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "import ") {
			continue
		}
		pkg := strings.TrimSpace(strings.TrimPrefix(trimmed, "import "))
		for _, prefix := range testOnlyImportPrefixes {
			if strings.HasPrefix(pkg, prefix) {
				col := strings.Index(line, "import")
				ctx.Emit(r.Finding(file, i+1, col+1,
					fmt.Sprintf("Test-only import %q in non-test file; move this code to a test source set.", pkg)))
				break
			}
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

	vftDecls := make(map[string]struct{})
	for _, file := range index.Files {
		for i, line := range file.Lines {
			if !strings.Contains(line, "@VisibleForTesting") {
				continue
			}
			for j := i + 1; j < len(file.Lines); j++ {
				nextLine := strings.TrimSpace(file.Lines[j])
				if strings.HasPrefix(nextLine, "@") {
					continue
				}
				if name := extractDeclName(nextLine); name != "" {
					vftDecls[name] = struct{}{}
				}
				break
			}
		}
	}

	if len(vftDecls) == 0 {
		return
	}

	// Bucket by byte length so per-line lookup is O(line_len × #lengths) rather
	// than O(line_len × #names) — a large Android repo has hundreds of names
	// but only ~20 unique lengths.
	namesByLen := make(map[int]map[string]struct{})
	for name := range vftDecls {
		L := len(name)
		bucket := namesByLen[L]
		if bucket == nil {
			bucket = make(map[string]struct{})
			namesByLen[L] = bucket
		}
		bucket[name] = struct{}{}
	}
	lengths := make([]int, 0, len(namesByLen))
	for L := range namesByLen {
		lengths = append(lengths, L)
	}

	sc := newVftScanner(namesByLen, lengths)
	for _, file := range index.Files {
		if isTestFile(file.Path) {
			continue
		}
		for i, line := range file.Lines {
			if len(line) < 2 || !strings.ContainsAny(line, "( ") {
				continue
			}
			if strings.Contains(line, "@VisibleForTesting") {
				continue
			}

			sc.reset()
			sc.scan(line, '(')
			sc.scan(line, ' ')
			if len(sc.matched) == 0 {
				continue
			}
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "fun ") || strings.HasPrefix(trimmed, "internal ") || strings.HasPrefix(trimmed, "private ") {
				continue
			}
			for name := range sc.matched {
				col := strings.Index(line, name)
				ctx.Emit(scanner.Finding{
					File:       file.Path,
					Line:       i + 1,
					Col:        col + 1,
					RuleSet:    r.RuleSetName,
					Rule:       r.RuleName,
					Severity:   r.Sev,
					Message:    fmt.Sprintf("@VisibleForTesting function %q called from non-test code.", name),
					Confidence: 0.7,
				})
			}
		}
	}
}

// vftScanner holds the read-only name dictionary (bucketed by length) and a
// reusable match set for a single pass over a file's lines. reset() clears
// matches between lines without reallocating the map.
type vftScanner struct {
	namesByLen map[int]map[string]struct{}
	lengths    []int
	matched    map[string]struct{}
}

func newVftScanner(namesByLen map[int]map[string]struct{}, lengths []int) *vftScanner {
	return &vftScanner{
		namesByLen: namesByLen,
		lengths:    lengths,
		matched:    make(map[string]struct{}),
	}
}

func (s *vftScanner) reset() {
	for k := range s.matched {
		delete(s.matched, k)
	}
}

// scan records all names that appear immediately before any `boundary` byte in
// the line. Uses SIMD-accelerated IndexByte to skip to each boundary because
// `(` and ` ` are sparse relative to the full byte range.
func (s *vftScanner) scan(line string, boundary byte) {
	start := 0
	for start < len(line) {
		rel := strings.IndexByte(line[start:], boundary)
		if rel < 0 {
			return
		}
		pos := start + rel
		// Names end in an identifier byte; if line[pos-1] isn't, skip the
		// O(#lengths) bucket loop entirely.
		if pos == 0 || !isIdentByte(line[pos-1]) {
			start = pos + 1
			continue
		}
		for _, L := range s.lengths {
			if pos < L {
				continue
			}
			cand := line[pos-L : pos]
			if _, ok := s.namesByLen[L][cand]; ok {
				s.matched[cand] = struct{}{}
			}
		}
		start = pos + 1
	}
}

func isIdentByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func extractDeclName(line string) string {
	for _, prefix := range []string{"fun ", "val ", "var ", "internal fun ", "internal val ", "internal var "} {
		if idx := strings.Index(line, prefix); idx >= 0 {
			rest := line[idx+len(prefix):]
			end := strings.IndexAny(rest, "( :<")
			if end > 0 {
				return strings.TrimSpace(rest[:end])
			}
		}
	}
	return ""
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

	openForTestTypes := make(map[string]bool)
	for _, file := range index.Files {
		for i, line := range file.Lines {
			if strings.Contains(strings.TrimSpace(line), "@OpenForTesting") {
				for j := i + 1; j < len(file.Lines); j++ {
					nextLine := strings.TrimSpace(file.Lines[j])
					if strings.HasPrefix(nextLine, "@") {
						continue
					}
					if idx := strings.Index(nextLine, "class "); idx >= 0 {
						rest := nextLine[idx+6:]
						end := strings.IndexAny(rest, "( :{<")
						if end > 0 {
							openForTestTypes[strings.TrimSpace(rest[:end])] = true
						}
					}
					break
				}
			}
		}
	}

	if len(openForTestTypes) == 0 {
		return
	}

	for _, file := range index.Files {
		if isTestFile(file.Path) {
			continue
		}
		for i, line := range file.Lines {
			for typeName := range openForTestTypes {
				if strings.Contains(line, ": "+typeName) || strings.Contains(line, ":"+typeName) ||
					strings.Contains(line, typeName+"()") {
					trimmed := strings.TrimSpace(line)
					if strings.Contains(trimmed, "class ") && (strings.Contains(trimmed, ": "+typeName) || strings.Contains(trimmed, ":"+typeName)) {
						col := strings.Index(line, typeName)
						ctx.Emit(scanner.Finding{
							File:       file.Path,
							Line:       i + 1,
							Col:        col + 1,
							RuleSet:    r.RuleSetName,
							Rule:       r.RuleName,
							Severity:   r.Sev,
							Message:    fmt.Sprintf("@OpenForTesting type %q subclassed outside test code.", typeName),
							Confidence: 0.65,
						})
					}
				}
			}
		}
	}
}

// TestFixtureAccessedFromProductionRule flags usage of types declared under
// src/testFixtures/ from non-test files.
type TestFixtureAccessedFromProductionRule struct {
	BaseRule
}

func (r *TestFixtureAccessedFromProductionRule) Confidence() float64 { return 0.80 }
func (r *TestFixtureAccessedFromProductionRule) check(ctx *v2.Context) {
	index := ctx.CodeIndex
	if index == nil {
		return
	}

	fixtureTypes := make(map[string]string)
	for _, file := range index.Files {
		if !strings.Contains(file.Path, "/testFixtures/") {
			continue
		}
		for _, line := range file.Lines {
			trimmed := strings.TrimSpace(line)
			for _, prefix := range []string{"class ", "object ", "interface ", "data class ", "sealed class ", "enum class "} {
				if idx := strings.Index(trimmed, prefix); idx >= 0 {
					rest := trimmed[idx+len(prefix):]
					end := strings.IndexAny(rest, "( :{<")
					if end > 0 {
						name := strings.TrimSpace(rest[:end])
						fixtureTypes[name] = file.Path
					} else if len(rest) > 0 {
						fixtureTypes[strings.TrimSpace(rest)] = file.Path
					}
				}
			}
		}
	}

	if len(fixtureTypes) == 0 {
		return
	}

	for _, file := range index.Files {
		if isTestFile(file.Path) || strings.Contains(file.Path, "/testFixtures/") {
			continue
		}
		for i, line := range file.Lines {
			for typeName := range fixtureTypes {
				if strings.Contains(line, typeName) {
					col := strings.Index(line, typeName)
					ctx.Emit(scanner.Finding{
						File:       file.Path,
						Line:       i + 1,
						Col:        col + 1,
						RuleSet:    r.RuleSetName,
						Rule:       r.RuleName,
						Severity:   r.Sev,
						Message:    fmt.Sprintf("Test fixture type %q from testFixtures/ used in production code.", typeName),
						Confidence: 0.7,
					})
				}
			}
		}
	}
}

// TimberTreeNotPlantedRule flags projects that use Timber.d/i/w/e but have
// no Timber.plant() call reachable from Application.onCreate.
type TimberTreeNotPlantedRule struct {
	BaseRule
}

func (r *TimberTreeNotPlantedRule) Confidence() float64 { return 0.75 }
func (r *TimberTreeNotPlantedRule) check(ctx *v2.Context) {
	index := ctx.CodeIndex
	if index == nil {
		return
	}

	var hasTimberUsage bool
	var hasTimberPlant bool
	var firstTimberUsage *scanner.File
	var firstTimberLine int

	for _, file := range index.Files {
		for i, line := range file.Lines {
			trimmed := strings.TrimSpace(line)
			if strings.Contains(trimmed, "Timber.plant(") || strings.Contains(trimmed, "Timber.plant (") {
				hasTimberPlant = true
			}
			if !hasTimberUsage && timberUsageRe.MatchString(trimmed) {
				hasTimberUsage = true
				firstTimberUsage = file
				firstTimberLine = i
			}
		}
		if hasTimberPlant {
			return
		}
	}

	if !hasTimberUsage || hasTimberPlant {
		return
	}

	ctx.Emit(scanner.Finding{
		File:       firstTimberUsage.Path,
		Line:       firstTimberLine + 1,
		Col:        1,
		RuleSet:    r.RuleSetName,
		Rule:       r.RuleName,
		Severity:   r.Sev,
		Message:    "Timber is used but Timber.plant() was never called; logs will be silently dropped.",
		Confidence: 0.7,
	})
}

var timberUsageRe = regexp.MustCompile(`Timber\.(v|d|i|w|e|wtf)\s*\(`)
