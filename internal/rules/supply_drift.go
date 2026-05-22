package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/module"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// KotlinVersionMismatchAcrossModulesRule flags modules with divergent Kotlin JVM targets.
type KotlinVersionMismatchAcrossModulesRule struct {
	BaseRule
}

func (r *KotlinVersionMismatchAcrossModulesRule) Confidence() float64 {
	return api.ConfidenceMediumHigh
}

func (r *KotlinVersionMismatchAcrossModulesRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{}
}

func (r *KotlinVersionMismatchAcrossModulesRule) check(ctx *api.Context) {
	if ctx.ModuleIndex == nil || ctx.ModuleIndex.Graph == nil {
		return
	}
	modules := collectModuleKotlinJvmTargets(ctx.ModuleIndex.Graph)
	if len(modules) < 2 {
		return
	}
	majority := majorityKotlinJvmTarget(modules)
	if majority == 0 {
		return
	}
	summary := formatModuleKotlinJvmTargets(modules)
	for _, mod := range modules {
		if mod.value == majority {
			continue
		}
		ctx.Emit(scanner.Finding{
			File:       mod.buildFile,
			Line:       mod.line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    fmt.Sprintf("Module %s uses Kotlin JVM target %d, but the project majority is %d. Align Kotlin JVM targets across modules. Project values: %s.", mod.modulePath, mod.value, majority, summary),
			Confidence: r.Confidence(),
		})
	}
}

// JvmTargetMismatchRule flags Kotlin and Java JVM target settings that disagree.
type JvmTargetMismatchRule struct {
	GradleBase
	BaseRule
}

func (r *JvmTargetMismatchRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *JvmTargetMismatchRule) check(ctx *api.Context) {
	path, content := ctx.GradlePath, ctx.GradleContent
	if !isGradleRepositoryScript(path) {
		return
	}
	targets := collectGradleJvmTargets(content)
	if !targets.hasMismatch() {
		return
	}
	ctx.Emit(scanner.Finding{
		File:       path,
		Line:       targets.line(),
		Col:        1,
		RuleSet:    r.RuleSetName,
		Rule:       r.RuleName,
		Severity:   r.Sev,
		Message:    fmt.Sprintf("Kotlin JVM target %s and Java compatibility target %s disagree. Align jvmTarget, sourceCompatibility, targetCompatibility, and toolchain settings.", targets.kotlinSummary(), targets.javaSummary()),
		Confidence: r.Confidence(),
	})
}

type gradleJvmTargetValue struct {
	value int
	line  int
}

type moduleKotlinJvmTarget struct {
	modulePath string
	buildFile  string
	line       int
	value      int
}

func collectModuleKotlinJvmTargets(graph *module.Graph) []moduleKotlinJvmTarget {
	var modules []moduleKotlinJvmTarget
	for _, mod := range graph.Modules {
		buildFile := gradleBuildFileForModule(mod.Dir)
		if buildFile == "" {
			continue
		}
		data, err := os.ReadFile(buildFile)
		if err != nil {
			continue
		}
		targets := collectGradleJvmTargets(string(data))
		value, line, ok := primaryKotlinJvmTarget(targets)
		if !ok {
			continue
		}
		modules = append(modules, moduleKotlinJvmTarget{modulePath: mod.Path, buildFile: buildFile, line: line, value: value})
	}
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].modulePath < modules[j].modulePath
	})
	return modules
}

func primaryKotlinJvmTarget(targets gradleJvmTargets) (value, line int, ok bool) {
	if len(targets.toolchain) > 0 {
		return targets.toolchain[0].value, targets.toolchain[0].line, true
	}
	if len(targets.kotlin) > 0 {
		return targets.kotlin[0].value, targets.kotlin[0].line, true
	}
	return 0, 0, false
}

func majorityKotlinJvmTarget(modules []moduleKotlinJvmTarget) int {
	counts := map[int]int{}
	for _, mod := range modules {
		counts[mod.value]++
	}
	bestValue, bestCount := 0, 0
	for value, count := range counts {
		if count > bestCount || (count == bestCount && (bestValue == 0 || value < bestValue)) {
			bestValue, bestCount = value, count
		}
	}
	if len(counts) < 2 {
		return 0
	}
	return bestValue
}

func formatModuleKotlinJvmTargets(modules []moduleKotlinJvmTarget) string {
	parts := make([]string, 0, len(modules))
	for _, mod := range modules {
		parts = append(parts, fmt.Sprintf("%s=%d", mod.modulePath, mod.value))
	}
	return strings.Join(parts, ", ")
}

type gradleJvmTargets struct {
	kotlin    []gradleJvmTargetValue
	java      []gradleJvmTargetValue
	toolchain []gradleJvmTargetValue
}

func (t gradleJvmTargets) hasMismatch() bool {
	kotlin := append([]gradleJvmTargetValue{}, t.kotlin...)
	javaTargets := append([]gradleJvmTargetValue{}, t.java...)
	if len(t.toolchain) > 0 {
		if len(kotlin) == 0 {
			kotlin = append(kotlin, t.toolchain...)
		}
		if len(javaTargets) == 0 {
			javaTargets = append(javaTargets, t.toolchain...)
		}
	}
	for _, k := range kotlin {
		for _, j := range javaTargets {
			if k.value != j.value {
				return true
			}
		}
	}
	return false
}

func (t gradleJvmTargets) line() int {
	for _, values := range [][]gradleJvmTargetValue{t.kotlin, t.java, t.toolchain} {
		if len(values) > 0 && values[0].line > 0 {
			return values[0].line
		}
	}
	return 1
}

func (t gradleJvmTargets) kotlinSummary() string {
	return gradleJvmTargetSummary(t.kotlin, t.toolchain)
}

func (t gradleJvmTargets) javaSummary() string {
	return gradleJvmTargetSummary(t.java, t.toolchain)
}

func gradleJvmTargetSummary(values, toolchain []gradleJvmTargetValue) string {
	if len(values) == 0 && len(toolchain) > 0 {
		values = toolchain
	}
	seen := map[int]bool{}
	var parts []string
	for _, value := range values {
		if !seen[value.value] {
			seen[value.value] = true
			parts = append(parts, strconv.Itoa(value.value))
		}
	}
	if len(parts) == 0 {
		return "<unset>"
	}
	return strings.Join(parts, ",")
}

var (
	gradleJvmTargetAssignRe      = regexp.MustCompile(`\bjvmTarget\b\s*(?:=|\()\s*["']?([^"')\s]+)`)
	gradleJavaCompatibilityRe    = regexp.MustCompile(`\b(?:sourceCompatibility|targetCompatibility)\b\s*(?:=|\()\s*([^)\s;]+)`)
	gradleJvmToolchainCallRe     = regexp.MustCompile(`\bjvmToolchain\s*\(\s*(\d+)\s*\)`)
	gradleJvmToolchainLanguageRe = regexp.MustCompile(`\blanguageVersion\b\s*=\s*JavaLanguageVersion\.of\(\s*(\d+)\s*\)`)
	gradlePluginIDCallRe         = regexp.MustCompile(`\bid\s*\(\s*["']([^"']+)["']\s*\)`)
	gradlePluginIDGroovyRe       = regexp.MustCompile(`\bid\s+["']([^"']+)["']`)
	gradleKotlinPluginCallRe     = regexp.MustCompile(`\bkotlin\s*\(\s*["']([^"']+)["']\s*\)`)
	gradleApplyPluginKtsRe       = regexp.MustCompile(`\bapply\s*\(\s*plugin\s*=\s*["']([^"']+)["']\s*\)`)
	gradleApplyPluginGroovyRe    = regexp.MustCompile(`\bapply\s+plugin\s*:\s*["']([^"']+)["']`)
)

func collectGradleJvmTargets(content string) gradleJvmTargets {
	var targets gradleJvmTargets
	for i, raw := range strings.Split(content, "\n") {
		if isGradleCommentLine(raw) {
			continue
		}
		line := gradleStripStringsAndComments(raw)
		rawNoComment := stripGradleLineComment(raw)
		lineNo := i + 1
		if match := gradleJvmTargetAssignRe.FindStringSubmatch(rawNoComment); len(match) == 2 {
			if value, ok := normalizeJvmTargetVersion(match[1]); ok {
				targets.kotlin = append(targets.kotlin, gradleJvmTargetValue{value: value, line: lineNo})
			}
		}
		if match := gradleJavaCompatibilityRe.FindStringSubmatch(rawNoComment); len(match) == 2 {
			if value, ok := normalizeJvmTargetVersion(match[1]); ok {
				targets.java = append(targets.java, gradleJvmTargetValue{value: value, line: lineNo})
			}
		}
		if match := gradleJvmToolchainCallRe.FindStringSubmatch(line); len(match) == 2 {
			if value, ok := normalizeJvmTargetVersion(match[1]); ok {
				targets.toolchain = append(targets.toolchain, gradleJvmTargetValue{value: value, line: lineNo})
			}
		}
		if match := gradleJvmToolchainLanguageRe.FindStringSubmatch(line); len(match) == 2 {
			if value, ok := normalizeJvmTargetVersion(match[1]); ok {
				targets.toolchain = append(targets.toolchain, gradleJvmTargetValue{value: value, line: lineNo})
			}
		}
	}
	return targets
}

func normalizeJvmTargetVersion(raw string) (int, bool) {
	value := strings.Trim(strings.TrimSpace(raw), `"'`)
	value = strings.TrimPrefix(value, "JavaVersion.")
	value = strings.TrimPrefix(value, "VERSION_")
	value = strings.TrimPrefix(value, "VERSION_1_")
	if value == "1.8" || value == "8" {
		return 8, true
	}
	value = strings.TrimPrefix(value, "1_")
	value = strings.TrimPrefix(value, "VERSION_")
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

type ConventionPluginAppliedToWrongTargetRule struct {
	BaseRule

	PluginTargetMap []string
}

func (r *ConventionPluginAppliedToWrongTargetRule) Confidence() float64 {
	return api.ConfidenceMediumLowPlus
}

func (r *ConventionPluginAppliedToWrongTargetRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{}
}

func (r *ConventionPluginAppliedToWrongTargetRule) check(ctx *api.Context) {
	pmi := ctx.ModuleIndex
	if pmi == nil || pmi.Graph == nil || pmi.Graph.RootDir == "" {
		return
	}
	explicitTargets := conventionPluginTargetMap(r.PluginTargetMap)
	for _, script := range conventionPluginTargetScripts(pmi.Graph) {
		for _, pluginID := range conventionAppliedPluginIDs(script.path) {
			pluginTarget, ok := conventionPluginIntendedTarget(pluginID, explicitTargets)
			if !ok || pluginTarget == "any" || pluginTarget == script.target {
				continue
			}
			if script.target == "unknown" {
				continue
			}
			ctx.Emit(scanner.Finding{
				File:       script.path,
				Line:       1,
				Col:        1,
				RuleSet:    r.RuleSetName,
				Rule:       r.RuleName,
				Severity:   r.Sev,
				Message:    fmt.Sprintf("Convention plugin %q targets %s modules but is applied to a %s build script.", pluginID, pluginTarget, script.target),
				Confidence: r.Confidence(),
			})
		}
	}
}

type conventionTargetScript struct {
	path   string
	target string
}

func conventionPluginTargetMap(entries []string) map[string]string {
	out := make(map[string]string)
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			key, value, ok = strings.Cut(entry, ":")
		}
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.ToLower(strings.TrimSpace(value))
		if key == "" || !conventionKnownTarget(value) {
			continue
		}
		out[key] = value
	}
	return out
}

func conventionKnownTarget(target string) bool {
	switch target {
	case "android", "jvm", "any":
		return true
	default:
		return false
	}
}

func conventionPluginIntendedTarget(pluginID string, explicit map[string]string) (string, bool) {
	if len(explicit) > 0 {
		target, ok := explicit[pluginID]
		return target, ok
	}
	lower := strings.ToLower(pluginID)
	switch {
	case strings.Contains(lower, "android"):
		return "android", true
	case strings.Contains(lower, "jvm"), strings.Contains(lower, "java"):
		return "jvm", true
	default:
		return "any", true
	}
}

func conventionPluginTargetScripts(graph *module.Graph) []conventionTargetScript {
	var scripts []conventionTargetScript
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		path := filepath.Join(graph.RootDir, name)
		if supplyDriftFileExists(path) {
			scripts = append(scripts, conventionTargetScript{path: path, target: "root"})
		}
	}
	for _, mod := range graph.Modules {
		if mod == nil || mod.Dir == "" {
			continue
		}
		if path := gradleBuildFileForModule(mod.Dir); path != "" {
			scripts = append(scripts, conventionTargetScript{path: path, target: conventionModuleTarget(mod.Dir, path)})
		}
	}
	sort.Slice(scripts, func(i, j int) bool { return scripts[i].path < scripts[j].path })
	return scripts
}

func conventionModuleTarget(moduleDir, buildPath string) string {
	ids := conventionAppliedPluginIDs(buildPath)
	for _, id := range ids {
		lower := strings.ToLower(id)
		switch {
		case strings.HasPrefix(lower, "com.android."), strings.Contains(lower, "android.application"), strings.Contains(lower, "android.library"):
			return "android"
		}
	}
	if supplyDriftFileExists(filepath.Join(moduleDir, "src", "main", "AndroidManifest.xml")) {
		return "android"
	}
	for _, id := range ids {
		lower := strings.ToLower(id)
		switch lower {
		case "org.jetbrains.kotlin.jvm", "java", "java-library", "application":
			return "jvm"
		}
	}
	return "unknown"
}

func conventionAppliedPluginIDs(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var ids []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "apply false") {
			continue
		}
		for _, match := range conventionPluginUsageRe.FindAllStringSubmatch(line, -1) {
			id := strings.TrimSpace(match[1])
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func supplyDriftFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type ApplyPluginTwiceRule struct {
	GradleBase
	BaseRule
}

func (r *ApplyPluginTwiceRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *ApplyPluginTwiceRule) check(ctx *api.Context) {
	path, content := ctx.GradlePath, ctx.GradleContent
	if !isGradleBuildScript(path) {
		return
	}
	for _, duplicate := range findApplyPluginTwice(content) {
		finding := scanner.Finding{
			File:       path,
			Line:       duplicate.line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    fmt.Sprintf("Plugin %q is applied in both plugins { } and apply(plugin = ...). Keep a single application form.", duplicate.id),
			Confidence: r.Confidence(),
		}
		finding.Fix = deleteContentLineFix(content, duplicate.line)
		ctx.Emit(finding)
	}
}

// deleteContentLineFix returns a byte-mode Fix that removes the 1-indexed
// line from a raw text buffer. Used by Gradle rules that operate on
// ctx.GradleContent rather than a parsed scanner.File. Returns nil when
// the line number is out of range.
func deleteContentLineFix(content string, line int) *scanner.Fix {
	if line < 1 {
		return nil
	}
	currentLine := 1
	start := 0
	for start < len(content) && currentLine < line {
		if content[start] == '\n' {
			currentLine++
		}
		start++
	}
	if currentLine != line {
		return nil
	}
	end := start
	for end < len(content) && content[end] != '\n' {
		end++
	}
	if end < len(content) && content[end] == '\n' {
		end++
	} else if start > 0 && content[start-1] == '\n' {
		start--
	}
	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   start,
		EndByte:     end,
		Replacement: "",
	}
}

type applyPluginTwiceFinding struct {
	id   string
	line int
}

type gradlePluginOccurrence struct {
	id   string
	line int
}

func findApplyPluginTwice(content string) []applyPluginTwiceFinding {
	declared := map[string]gradlePluginOccurrence{}
	var applied []gradlePluginOccurrence
	depth := 0
	inPluginsBlock := false
	pluginsBlockDepth := 0

	for i, rawLine := range strings.Split(content, "\n") {
		lineNo := i + 1
		code := gradleStripStringsAndComments(rawLine)
		rawNoComment := stripGradleLineComment(rawLine)
		if !inPluginsBlock && gradlePluginsBlockStarts(code) {
			inPluginsBlock = true
			pluginsBlockDepth = depth
		}
		if inPluginsBlock {
			for _, occurrence := range gradlePluginBlockOccurrences(rawNoComment, lineNo) {
				if _, ok := declared[occurrence.id]; !ok {
					declared[occurrence.id] = occurrence
				}
			}
		}
		applied = append(applied, gradleApplyPluginOccurrences(rawNoComment, lineNo)...)

		openBraces, closeBraces := countGradleBraces(code)
		depth += openBraces - closeBraces
		if depth < 0 {
			depth = 0
		}
		if inPluginsBlock && depth <= pluginsBlockDepth {
			inPluginsBlock = false
		}
	}

	seen := map[string]bool{}
	var findings []applyPluginTwiceFinding
	for _, occurrence := range applied {
		if seen[occurrence.id] {
			continue
		}
		if _, ok := declared[occurrence.id]; ok {
			findings = append(findings, applyPluginTwiceFinding(occurrence))
			seen[occurrence.id] = true
		}
	}
	return findings
}

func gradlePluginsBlockStarts(line string) bool {
	compact := strings.Join(strings.Fields(line), "")
	return strings.Contains(compact, "plugins{")
}

func gradlePluginBlockOccurrences(line string, lineNo int) []gradlePluginOccurrence {
	if strings.Contains(line, "apply false") {
		return nil
	}
	var occurrences []gradlePluginOccurrence
	for _, match := range gradlePluginIDCallRe.FindAllStringSubmatch(line, -1) {
		occurrences = append(occurrences, gradlePluginOccurrence{id: match[1], line: lineNo})
	}
	for _, match := range gradlePluginIDGroovyRe.FindAllStringSubmatch(line, -1) {
		occurrences = append(occurrences, gradlePluginOccurrence{id: match[1], line: lineNo})
	}
	for _, match := range gradleKotlinPluginCallRe.FindAllStringSubmatch(line, -1) {
		if id := kotlinGradlePluginID(match[1]); id != "" {
			occurrences = append(occurrences, gradlePluginOccurrence{id: id, line: lineNo})
		}
	}
	return occurrences
}

func gradleApplyPluginOccurrences(line string, lineNo int) []gradlePluginOccurrence {
	if !strings.HasPrefix(strings.TrimSpace(line), "apply") {
		return nil
	}
	var occurrences []gradlePluginOccurrence
	for _, match := range gradleApplyPluginKtsRe.FindAllStringSubmatch(line, -1) {
		occurrences = append(occurrences, gradlePluginOccurrence{id: match[1], line: lineNo})
	}
	for _, match := range gradleApplyPluginGroovyRe.FindAllStringSubmatch(line, -1) {
		occurrences = append(occurrences, gradlePluginOccurrence{id: match[1], line: lineNo})
	}
	return occurrences
}

func kotlinGradlePluginID(name string) string {
	switch strings.TrimSpace(name) {
	case "android":
		return "org.jetbrains.kotlin.android"
	case "jvm":
		return "org.jetbrains.kotlin.jvm"
	case "multiplatform":
		return "org.jetbrains.kotlin.multiplatform"
	case "kapt":
		return "org.jetbrains.kotlin.kapt"
	case "plugin.serialization", "serialization":
		return "org.jetbrains.kotlin.plugin.serialization"
	default:
		return ""
	}
}

type ConfigurationsAllSideEffectRule struct {
	GradleBase
	BaseRule

	AllowInConventionPlugins bool
}

func (r *ConfigurationsAllSideEffectRule) Confidence() float64 { return api.ConfidenceMediumLowPlus }

func (r *ConfigurationsAllSideEffectRule) check(ctx *api.Context) {
	path, content := ctx.GradlePath, ctx.GradleContent
	if !isGradleRepositoryScript(path) {
		return
	}
	if r.AllowInConventionPlugins && isConventionPluginBuildPath(path) {
		return
	}
	for _, block := range findConfigurationsAllSideEffects(content) {
		ctx.Emit(scanner.Finding{
			File:       path,
			Line:       block.line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    fmt.Sprintf("configurations.all mutates dependency resolution via %s. Prefer configuring a named configuration or a convention plugin.", block.mutator),
			Confidence: r.Confidence(),
		})
	}
}

type configurationsAllSideEffect struct {
	line    int
	mutator string
}

func isConventionPluginBuildPath(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.HasPrefix(normalized, "build-logic/") ||
		strings.HasPrefix(normalized, "buildSrc/") ||
		strings.Contains(normalized, "/build-logic/") ||
		strings.Contains(normalized, "/buildSrc/")
}

func findConfigurationsAllSideEffects(content string) []configurationsAllSideEffect {
	var findings []configurationsAllSideEffect
	depth := 0
	inBlock := false
	blockDepth := 0
	blockLine := 0
	blockMutator := ""

	for i, rawLine := range strings.Split(content, "\n") {
		code := gradleStripStringsAndComments(rawLine)
		if !inBlock && configurationsAllBlockStarts(code) {
			inBlock = true
			blockDepth = depth
			blockLine = i + 1
			blockMutator = ""
		}
		if inBlock && blockMutator == "" {
			blockMutator = configurationsAllSideEffectMutator(code)
		}

		openBraces, closeBraces := countGradleBraces(code)
		depth += openBraces - closeBraces
		if depth < 0 {
			depth = 0
		}
		if inBlock && depth <= blockDepth {
			if blockMutator != "" {
				findings = append(findings, configurationsAllSideEffect{
					line:    blockLine,
					mutator: blockMutator,
				})
			}
			inBlock = false
		}
	}
	return findings
}

func configurationsAllBlockStarts(line string) bool {
	compact := strings.Join(strings.Fields(line), "")
	return strings.Contains(compact, "configurations.all{") ||
		strings.Contains(compact, "configurations.all({")
}

func configurationsAllSideEffectMutator(line string) string {
	compact := strings.Join(strings.Fields(line), "")
	switch {
	case strings.Contains(compact, "resolutionStrategy{") ||
		strings.Contains(compact, "resolutionStrategy.") ||
		strings.Contains(compact, "resolutionStrategy("):
		return "resolutionStrategy"
	case strings.Contains(compact, "dependencies.add("):
		return "dependencies.add"
	case strings.Contains(compact, ".exclude(") || strings.HasPrefix(compact, "exclude("):
		return "exclude"
	case strings.Contains(compact, ".force(") || strings.HasPrefix(compact, "force("):
		return "force"
	default:
		return ""
	}
}
