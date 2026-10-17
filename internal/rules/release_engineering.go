package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/module"
	api "github.com/kaeawc/krit/internal/rules/api"
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
func (r *BuildConfigDebugInLibraryRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *BuildConfigDebugInvertedRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *AllProjectsBlockRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence. Release-engineering rule. Detection scans module metadata and Gradle
// files for configuration drift and plugin hygiene; matches are
// project-structure-sensitive. Classified per roadmap/17.
func (r *HardcodedEnvironmentNameRule) Confidence() float64 { return api.ConfidenceMedium }

// ConventionPluginDeadCodeRule flags precompiled convention plugins declared
// under build-logic/ or buildSrc/ that are not applied by any module.
type ConventionPluginDeadCodeRule struct {
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Release-engineering rule. Detection scans module metadata and Gradle
// files for configuration drift and plugin hygiene; matches are
// project-structure-sensitive. Classified per roadmap/17.
func (r *ConventionPluginDeadCodeRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ConventionPluginDeadCodeRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{}
}

func (r *ConventionPluginDeadCodeRule) check(ctx *api.Context) {
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
			Confidence: api.ConfidenceHigher,
		})
	}
}

type conventionPlugin struct {
	id   string
	path string
}

var conventionPluginUsageRe = regexp.MustCompile(`id\s*\(\s*["']([^"']+)["']\s*\)`)

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
	GradleBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Release-engineering rule. Detection scans module metadata and Gradle
// files for configuration drift and plugin hygiene; matches are
// project-structure-sensitive. Classified per roadmap/17.
func (r *GradleBuildContainsTodoRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *GradleBuildContainsTodoRule) check(ctx *api.Context) {
	for i, raw := range strings.Split(ctx.GradleContent, "\n") {
		commentIdx := gradleLineCommentStart(raw)
		if commentIdx < 0 {
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(raw[commentIdx+2:]), "TODO") {
			continue
		}
		ctx.Emit(scanner.Finding{
			File:    ctx.GradlePath,
			Line:    i + 1,
			Col:     commentIdx + 1,
			Message: "TODO comment found in build.gradle(.kts); track build work in an issue or finish it before release.",
		})
	}
}

// Confidence reports a tier-2 (medium) base confidence. Release-engineering rule. Detection scans module metadata and Gradle
// files for configuration drift and plugin hygiene; matches are
// project-structure-sensitive. Classified per roadmap/17.
func (r *CommentedOutCodeBlockRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *CommentedOutCodeBlockRule) check(ctx *api.Context) {
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
		f := r.Finding(file, startLine+1, col+1, msg)
		f.Fix = deleteLineRangeFix(file, startLine, endLine-1)
		ctx.Emit(f)
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
				return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
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

func scanAppliedConventionPluginIDs(graph *module.Graph) map[string]bool {
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

func gradleScriptsToInspect(graph *module.Graph) []string {
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

func hardcodedEnvironmentLiteralFlat(file *scanner.File, arg uint32) string {
	if file == nil || arg == 0 {
		return ""
	}
	labelPresent := flatValueArgumentLabel(file, arg) != ""
	return hardcodedEnvironmentLiteral(file.FlatNodeText(arg), labelPresent)
}

type sourceModuleGradleInfo struct {
	found         bool
	isApplication bool
	isLibrary     bool
}

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
	info := filefacts.StringFact(fileFactsCache(), dir, slotSourceModuleGradle, func() sourceModuleGradleInfo {
		return readSourceModuleGradleInfo(dir)
	})
	return info, info.found
}

func readSourceModuleGradleInfo(dir string) sourceModuleGradleInfo {
	var path string
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
			break
		}
	}
	if path == "" {
		return sourceModuleGradleInfo{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return sourceModuleGradleInfo{}
	}
	content := string(data)
	// Plugin ids live inside string literals (`id("com.android.application")`,
	// `classpath 'com.android.library'`), so for those checks we only
	// strip comments and keep string bodies. `applicationId` is a real
	// DSL property identifier, so for that check we additionally blank
	// out string bodies — otherwise a description like
	// `description = "applicationId is configured per flavor"` would
	// flip a library module to "application" classification.
	commentsStripped := gradleStripCommentsFull(content)
	codeOnly := gradleStripCommentsAndStringBodiesFull(content)
	return sourceModuleGradleInfo{
		found: true,
		isApplication: strings.Contains(commentsStripped, "com.android.application") ||
			gradleContainsCodeToken(codeOnly, "applicationId") ||
			containsPluginSuffix(commentsStripped, "application"),
		isLibrary: strings.Contains(commentsStripped, "com.android.library") ||
			containsPluginSuffix(commentsStripped, "library"),
	}
}
