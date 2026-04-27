package rules

// Android Gradle DSL lint rules.
// These rules analyze build.gradle / build.gradle.kts files using the Gradle
// scanner from internal/android/gradle.go.
//
// They run once per project on the parsed BuildConfig and raw file content,
// similar to how the manifest rules operate on AndroidManifest.xml.

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/android"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// GradleBase is an empty marker type embedded by Gradle rule
// implementations. The v2 registry source records AndroidDependencies()
// metadata on v2.Rule.AndroidDeps.
type GradleBase struct{}

func (GradleBase) AndroidDependencies() AndroidDataDependency {
	return AndroidDepGradle
}

// ---------------------------------------------------------------------------
// Helper to create a Gradle finding
// ---------------------------------------------------------------------------

func gradleFinding(path string, line int, rule BaseRule, msg string) scanner.Finding {
	return scanner.Finding{
		File:     path,
		Line:     line,
		Col:      1,
		RuleSet:  rule.RuleSetName,
		Rule:     rule.RuleName,
		Severity: rule.Sev,
		Message:  msg,
	}
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
// non-comment line containing substr, or 0 if absent. Comment lines
// (after stripping leading whitespace, starting with `//` or `*`)
// are skipped so we don't report findings on commented-out code.
func findGradleLineStr(content, substr string) int {
	for i, line := range strings.Split(content, "\n") {
		if isGradleCommentLine(line) {
			continue
		}
		if strings.Contains(line, substr) {
			return i + 1
		}
	}
	return 0
}

// isGradleCommentLine reports true when a Gradle line (Groovy or
// Kotlin DSL) is a single-line comment or a block-comment continuation
// and should therefore be excluded from semantic checks.
func isGradleCommentLine(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "//") || strings.HasPrefix(t, "*") || strings.HasPrefix(t, "/*")
}

// ---------------------------------------------------------------------------
// Rule: GradlePluginCompatibility
// ---------------------------------------------------------------------------

// GradlePluginCompatibilityRule checks AGP version vs Gradle version compatibility.
// Delegates to the scanner's lintGradleCompatibility via LintBuildGradle.
type GradlePluginCompatibilityRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradlePluginCompatibilityRule) Confidence() float64 { return 0.75 }

func (r *GradlePluginCompatibilityRule) check(ctx *v2.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	lints := android.LintBuildGradle(content, cfg, 0) // threshold 0 = skip MinSdkTooLow
	for _, l := range lints {
		if l.Rule == "GradleCompatibility" {
			line := l.Line
			if line == 0 {
				line = 1
			}
			ctx.Emit(gradleFinding(path, line, r.BaseRule, l.Message))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: StringInteger
// ---------------------------------------------------------------------------

// StringIntegerRule flags string values where integers are expected in Gradle
// build files — for example, minSdk = "24" instead of minSdk = 24.
type StringIntegerRule struct {
	GradleBase
	AndroidRule
}

// sdkPropertyNames lists the Gradle DSL property names whose values
// must be integers, not string literals, in both Groovy (.gradle) and
// Kotlin (.kts) DSL.
var sdkPropertyNames = []string{
	"minSdk", "minSdkVersion",
	"targetSdk", "targetSdkVersion",
	"compileSdk", "compileSdkVersion",
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *StringIntegerRule) Confidence() float64 { return 0.75 }

func (r *StringIntegerRule) check(ctx *v2.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	for i, line := range strings.Split(content, "\n") {
		if gradleLineHasQuotedIntegerSDKProperty(line) {
			ctx.Emit(gradleFinding(path, i+1, r.BaseRule,
				"SDK version should be an integer, not a string. Remove quotes."))
		}
	}
}

func gradleLineHasQuotedIntegerSDKProperty(line string) bool {
	line = stripGradleLineComment(line)
	if strings.TrimSpace(line) == "" {
		return false
	}
	for _, prop := range sdkPropertyNames {
		if gradleLineHasQuotedIntegerAssignment(line, prop) {
			return true
		}
	}
	return false
}

func stripGradleLineComment(line string) string {
	inQuote := rune(0)
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if inQuote != 0 {
			if r == '\\' {
				escaped = true
				continue
			}
			if r == inQuote {
				inQuote = 0
			}
			continue
		}
		if r == '"' || r == '\'' {
			inQuote = r
			continue
		}
		if r == '/' && i+1 < len(line) && line[i+1] == '/' {
			return line[:i]
		}
	}
	return line
}

func gradleLineHasQuotedIntegerAssignment(line, prop string) bool {
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRuneInString(line[i:])
		if !isGradleIdentStart(r) {
			i += size
			continue
		}
		start := i
		i += size
		for i < len(line) {
			r, size = utf8.DecodeRuneInString(line[i:])
			if !isGradleIdentPart(r) {
				break
			}
			i += size
		}
		if line[start:i] != prop {
			continue
		}
		if start > 0 {
			prev, _ := utf8.DecodeLastRuneInString(line[:start])
			if isGradleIdentPart(prev) {
				continue
			}
		}
		rest := strings.TrimSpace(line[i:])
		if rest == "" {
			return false
		}
		if rest[0] == '=' || rest[0] == '(' {
			rest = strings.TrimSpace(rest[1:])
		}
		if rest == "" || (rest[0] != '"' && rest[0] != '\'') {
			return false
		}
		quote := rest[0]
		end := 1
		for end < len(rest) && rest[end] != quote {
			end++
		}
		if end == 1 || end >= len(rest) {
			return false
		}
		value := rest[1:end]
		for _, ch := range value {
			if !unicode.IsDigit(ch) {
				return false
			}
		}
		return true
	}
	return false
}

func isGradleIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isGradleIdentPart(r rune) bool {
	return isGradleIdentStart(r) || unicode.IsDigit(r)
}

// ---------------------------------------------------------------------------
// Rule: RemoteVersion
// ---------------------------------------------------------------------------

// RemoteVersionRule flags dependencies using "+" or "latest.release" as their
// version, which leads to non-deterministic builds.
type RemoteVersionRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *RemoteVersionRule) Confidence() float64 { return 0.75 }

func (r *RemoteVersionRule) check(ctx *v2.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	for _, dep := range cfg.Dependencies {
		if dep.Version == "+" || dep.Version == "latest.release" || dep.Version == "latest.integration" {
			coord := dep.Group + ":" + dep.Name + ":" + dep.Version
			line := findGradleLineStr(content, coord)
			if line == 0 {
				line = 1
			}
			ctx.Emit(gradleFinding(path, line, r.BaseRule,
				fmt.Sprintf("Dependency %s:%s uses non-deterministic version `%s`. Pin to a specific version.",
					dep.Group, dep.Name, dep.Version)))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: DynamicVersion
// ---------------------------------------------------------------------------

// DynamicVersionRule flags dependencies using partial wildcard versions such as
// "1.+" or "2.3.+", which create non-reproducible builds.
type DynamicVersionRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *DynamicVersionRule) Confidence() float64 { return 0.75 }

func (r *DynamicVersionRule) check(ctx *v2.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	for _, dep := range cfg.Dependencies {
		// Skip pure "+" (handled by RemoteVersion) and empty versions.
		if dep.Version == "" || dep.Version == "+" || dep.Version == "latest.release" || dep.Version == "latest.integration" {
			continue
		}
		if strings.Contains(dep.Version, "+") {
			coord := dep.Group + ":" + dep.Name + ":" + dep.Version
			line := findGradleLineStr(content, coord)
			if line == 0 {
				line = 1
			}
			ctx.Emit(gradleFinding(path, line, r.BaseRule,
				fmt.Sprintf("Dependency %s:%s uses dynamic version `%s`. Pin to a specific version for reproducible builds.",
					dep.Group, dep.Name, dep.Version)))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: OldTargetApi
// ---------------------------------------------------------------------------

// GradleOldTargetApiRule flags targetSdkVersion below a configurable threshold.
// Default threshold: 33 (Android 13).
type GradleOldTargetApiRule struct {
	GradleBase
	AndroidRule
	Threshold int // minimum acceptable targetSdkVersion
}

const defaultOldTargetApiThreshold = 33

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradleOldTargetApiRule) Confidence() float64 { return 0.75 }

func (r *GradleOldTargetApiRule) check(ctx *v2.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	threshold := r.Threshold
	if threshold == 0 {
		threshold = defaultOldTargetApiThreshold
	}
	if cfg.TargetSdkVersion == 0 || cfg.TargetSdkVersion >= threshold {
		return
	}
	line := findGradleLineStr(content, "targetSdk")
	if line == 0 {
		line = 1
	}
	ctx.Emit(gradleFinding(path, line, r.BaseRule,
		fmt.Sprintf("targetSdkVersion %d is below the recommended minimum of %d. "+
			"Update to comply with Google Play requirements.",
			cfg.TargetSdkVersion, threshold)))
}

// ---------------------------------------------------------------------------
// Rule: DeprecatedDependency
// ---------------------------------------------------------------------------

// DeprecatedDependencyRule flags known deprecated libraries (e.g., support-v4).
// Delegates to the scanner's lintDeprecatedDependency.
type DeprecatedDependencyRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *DeprecatedDependencyRule) Confidence() float64 { return 0.75 }

func (r *DeprecatedDependencyRule) check(ctx *v2.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	lints := android.LintBuildGradle(content, cfg, 0)
	for _, l := range lints {
		if l.Rule == "DeprecatedDependency" {
			line := l.Line
			if line == 0 {
				// Try to find the dependency in the content for a better line number
				line = 1
			}
			ctx.Emit(gradleFinding(path, line, r.BaseRule, l.Message))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: MavenLocal
// ---------------------------------------------------------------------------

// MavenLocalRule flags usage of mavenLocal() repository, which can cause
// unreproducible builds.
type MavenLocalRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *MavenLocalRule) Confidence() float64 { return 0.75 }

func (r *MavenLocalRule) check(ctx *v2.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	line := findGradleLineStr(content, "mavenLocal()")
	if line == 0 {
		return
	}
	ctx.Emit(gradleFinding(path, line, r.BaseRule,
		"mavenLocal() can cause unreproducible builds; prefer a remote repository or includeBuild."))
}

// ---------------------------------------------------------------------------
// Rule: MinSdkTooLow
// ---------------------------------------------------------------------------

// MinSdkTooLowRule flags minSdkVersion below a configurable threshold.
// Default threshold: 21 (Android 5.0 Lollipop).
type MinSdkTooLowRule struct {
	GradleBase
	AndroidRule
	Threshold int // minimum acceptable minSdkVersion
}

const defaultMinSdkTooLowThreshold = 21

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *MinSdkTooLowRule) Confidence() float64 { return 0.75 }

func (r *MinSdkTooLowRule) check(ctx *v2.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	threshold := r.Threshold
	if threshold == 0 {
		threshold = defaultMinSdkTooLowThreshold
	}
	if cfg.MinSdkVersion == 0 || cfg.MinSdkVersion >= threshold {
		return
	}
	line := findGradleLineStr(content, "minSdk")
	if line == 0 {
		line = 1
	}
	ctx.Emit(gradleFinding(path, line, r.BaseRule,
		fmt.Sprintf("minSdk %d is below the recommended minimum of %d.",
			cfg.MinSdkVersion, threshold)))
}

// ---------------------------------------------------------------------------
// Rule: GradleDeprecated
// ---------------------------------------------------------------------------

// GradleDeprecatedRule detects deprecated Gradle configuration names that have
// been replaced in modern Gradle (e.g. `compile` -> `implementation`).
type GradleDeprecatedRule struct {
	GradleBase
	AndroidRule
}

// deprecatedConfigs maps deprecated configuration names to their replacements.
var deprecatedConfigs = map[string]string{
	"compile":     "implementation or api",
	"testCompile": "testImplementation",
	"provided":    "compileOnly",
	"apk":         "runtimeOnly",
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradleDeprecatedRule) Confidence() float64 { return 0.75 }

func (r *GradleDeprecatedRule) check(ctx *v2.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	for i, line := range strings.Split(content, "\n") {
		if isGradleCommentLine(line) {
			continue
		}
		for name, replacement := range deprecatedConfigs {
			idx := strings.Index(line, name)
			if idx < 0 {
				continue
			}
			// The name must be at a word boundary followed by `(`, `"`, or `'`
			// so that `compileOnly` doesn't match the `compile` prefix.
			rest := strings.TrimSpace(line[idx+len(name):])
			if len(rest) == 0 || (rest[0] != '(' && rest[0] != '"' && rest[0] != '\'') {
				continue
			}
			// Reject if preceded by a letter or `_` (e.g. `testCompile` won't
			// falsely trigger `compile` since we check the exact name, but be
			// careful not to match in the middle of longer words).
			if idx > 0 {
				prev := line[idx-1]
				if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') || prev == '_' {
					continue
				}
			}
			ctx.Emit(gradleFinding(path, i+1, r.BaseRule,
				fmt.Sprintf("'%s' is deprecated; use '%s' instead.", name, replacement)))
			break
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: GradleGetter
// ---------------------------------------------------------------------------

// GradleGetterRule detects deprecated Groovy-style DSL property setters in
// Gradle Kotlin DSL files. For example, `compileSdkVersion 30` should be
// `compileSdk = 30` in .kts files.
type GradleGetterRule struct {
	GradleBase
	AndroidRule
}

// groovyStyleDSL maps deprecated Groovy-style DSL names to their KTS replacements.
var groovyStyleDSL = map[string]string{
	"compileSdkVersion": "compileSdk",
	"buildToolsVersion": "buildToolsVersion (remove or use buildToolsVersion = \"...\")",
	"minSdkVersion":     "minSdk",
	"targetSdkVersion":  "targetSdk",
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradleGetterRule) Confidence() float64 { return 0.75 }

func (r *GradleGetterRule) check(ctx *v2.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	// Only flag in .kts files where the Kotlin DSL should be used.
	if !strings.HasSuffix(path, ".kts") {
		return
	}
	for i, line := range strings.Split(content, "\n") {
		if isGradleCommentLine(line) {
			continue
		}
		for name, replacement := range groovyStyleDSL {
			idx := strings.Index(line, name)
			if idx < 0 {
				continue
			}
			// Must be followed by whitespace (space/tab) then a non-`=`
			// character — that's the Groovy positional-setter pattern.
			// `compileSdkVersion = 34` (Kotlin) must NOT trigger.
			after := line[idx+len(name):]
			if len(after) == 0 || (after[0] != ' ' && after[0] != '\t') {
				continue
			}
			rest := strings.TrimSpace(after)
			if len(rest) == 0 || rest[0] == '=' {
				continue
			}
			ctx.Emit(gradleFinding(path, i+1, r.BaseRule,
				fmt.Sprintf("Groovy-style '%s' should be replaced with '%s' in Kotlin DSL.", name, replacement)))
			break
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: GradlePath
// ---------------------------------------------------------------------------

// GradlePathRule detects problematic dependency paths in Gradle build files:
// absolute paths in files()/fileTree() and backslashes in paths.
type GradlePathRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradlePathRule) Confidence() float64 { return 0.75 }

// gradlePathCallContains reports whether a line contains a files() or
// fileTree() call whose string argument contains needle. We locate the
// argument by scanning past the opening `("` or `('` token and
// checking the substring from there.
func gradlePathCallContains(line, needle string) bool {
	for _, fn := range []string{"files(", "fileTree("} {
		idx := strings.Index(line, fn)
		if idx < 0 {
			continue
		}
		rest := line[idx+len(fn):]
		rest = strings.TrimSpace(rest)
		if len(rest) == 0 || (rest[0] != '"' && rest[0] != '\'') {
			continue
		}
		if strings.Contains(rest, needle) {
			return true
		}
	}
	return false
}

func (r *GradlePathRule) check(ctx *v2.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	for i, line := range strings.Split(content, "\n") {
		if isGradleCommentLine(line) {
			continue
		}
		for _, fn := range []string{"files(", "fileTree("} {
			idx := strings.Index(line, fn)
			if idx < 0 {
				continue
			}
			rest := strings.TrimSpace(line[idx+len(fn):])
			if len(rest) >= 2 && (rest[0] == '"' || rest[0] == '\'') && rest[1] == '/' {
				ctx.Emit(gradleFinding(path, i+1, r.BaseRule,
					"Avoid absolute paths in files()/fileTree(); use project-relative paths."))
				break
			}
		}
		if gradlePathCallContains(line, `\`) {
			ctx.Emit(gradleFinding(path, i+1, r.BaseRule,
				"Avoid backslashes in dependency paths; use forward slashes for cross-platform compatibility."))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: GradleOverrides
// ---------------------------------------------------------------------------

// GradleOverridesRule flags Gradle build files that set both minSdk and
// targetSdk, which overrides any values in AndroidManifest.xml and is a common
// source of confusion.
type GradleOverridesRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradleOverridesRule) Confidence() float64 { return 0.75 }

func (r *GradleOverridesRule) check(ctx *v2.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	if cfg.MinSdkVersion > 0 && cfg.TargetSdkVersion > 0 {
		// Report on the targetSdk line (second override).
		line := findGradleLineStr(content, "targetSdk")
		if line == 0 {
			line = 1
		}
		ctx.Emit(gradleFinding(path, line, r.BaseRule,
			"Both minSdk and targetSdk are set in the Gradle build file, overriding any values in AndroidManifest.xml."))
		return
	}
}

// ---------------------------------------------------------------------------
// Rule: GradleIdeError
// ---------------------------------------------------------------------------

// GradleIdeErrorRule detects Gradle IDE error patterns, such as using the
// legacy `apply plugin:` syntax in .kts files instead of the `plugins { }` block.
type GradleIdeErrorRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradleIdeErrorRule) Confidence() float64 { return 0.75 }

func (r *GradleIdeErrorRule) check(ctx *v2.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	if !strings.HasSuffix(path, ".kts") {
		return
	}
	for i, line := range strings.Split(content, "\n") {
		if isGradleCommentLine(line) {
			continue
		}
		trimmed := strings.TrimSpace(line)
		// Groovy-style: `apply plugin: "..."` — invalid Kotlin syntax
		// Kotlin-legacy: `apply(plugin = "...")` — valid Kotlin but deprecated
		if strings.HasPrefix(trimmed, "apply plugin:") ||
			(strings.HasPrefix(trimmed, "apply(") && strings.Contains(trimmed, "plugin")) {
			ctx.Emit(gradleFinding(path, i+1, r.BaseRule,
				"Use the plugins { } block instead of 'apply plugin:' in Kotlin DSL (.kts) files."))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: AndroidGradlePluginVersion
// ---------------------------------------------------------------------------

// AndroidGradlePluginVersionRule flags AGP versions older than 7.0, which are
// no longer supported and may be missing critical fixes.
type AndroidGradlePluginVersionRule struct {
	GradleBase
	AndroidRule
}

var agpVersionRe = regexp.MustCompile(`com\.android\.tools\.build:gradle:(\d+)\.(\d+)\.(\d+)`)

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *AndroidGradlePluginVersionRule) Confidence() float64 { return 0.75 }

func (r *AndroidGradlePluginVersionRule) check(ctx *v2.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	matches := agpVersionRe.FindStringSubmatch(content)
	if matches == nil {
		return
	}
	major := 0
	fmt.Sscanf(matches[1], "%d", &major)
	if major >= 7 {
		return
	}
	line := findGradleLine(content, agpVersionRe)
	if line == 0 {
		line = 1
	}
	version := matches[1] + "." + matches[2] + "." + matches[3]
	ctx.Emit(gradleFinding(path, line, r.BaseRule,
		fmt.Sprintf("Android Gradle Plugin version %s is too old. Upgrade to at least 7.0.0.", version)))
}

// ---------------------------------------------------------------------------
// Rule: NewerVersionAvailable
// ---------------------------------------------------------------------------

// NewerVersionAvailableRule flags dependencies using known-outdated versions
// of major libraries based on a minimum recommended version table.
type NewerVersionAvailableRule struct {
	GradleBase
	AndroidRule
	RecommendedVersions []libMinVersion
}

// minRecommended maps group:name to minimum recommended version.
type libMinVersion struct {
	Group    string
	Name     string
	MinMajor int
	MinMinor int
	MinPatch int
	Display  string
}

var recommendedVersions = []libMinVersion{
	{"androidx.appcompat", "appcompat", 1, 6, 0, "1.6.0"},
	{"com.google.android.material", "material", 1, 9, 0, "1.9.0"},
	{"org.jetbrains.kotlin", "kotlin-stdlib", 1, 9, 0, "1.9.0"},
	{"org.jetbrains.kotlin", "kotlin-stdlib-jdk8", 1, 9, 0, "1.9.0"},
	{"org.jetbrains.kotlin", "kotlin-stdlib-jdk7", 1, 9, 0, "1.9.0"},
	{"androidx.core", "core-ktx", 1, 10, 0, "1.10.0"},
	{"androidx.activity", "activity-compose", 1, 7, 0, "1.7.0"},
	{"androidx.lifecycle", "lifecycle-runtime-ktx", 2, 6, 0, "2.6.0"},
	{"androidx.compose.ui", "ui", 1, 5, 0, "1.5.0"},
	{"androidx.recyclerview", "recyclerview", 1, 3, 0, "1.3.0"},
}

func parseRecommendedVersionSpec(spec string) (libMinVersion, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return libMinVersion{}, fmt.Errorf("empty recommendation spec")
	}

	var coords, version string
	switch {
	case strings.Contains(spec, "="):
		parts := strings.SplitN(spec, "=", 2)
		coords, version = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	case strings.Count(spec, ":") >= 2:
		parts := strings.SplitN(spec, ":", 3)
		coords = strings.TrimSpace(parts[0] + ":" + parts[1])
		version = strings.TrimSpace(parts[2])
	default:
		return libMinVersion{}, fmt.Errorf("expected group:name=version or group:name:version")
	}

	coordParts := strings.SplitN(coords, ":", 2)
	if len(coordParts) != 2 || coordParts[0] == "" || coordParts[1] == "" {
		return libMinVersion{}, fmt.Errorf("invalid coordinates %q", coords)
	}
	if version == "" {
		return libMinVersion{}, fmt.Errorf("missing version")
	}

	major, minor, patch := parseVersion(version)
	return libMinVersion{
		Group:    coordParts[0],
		Name:     coordParts[1],
		MinMajor: major,
		MinMinor: minor,
		MinPatch: patch,
		Display:  version,
	}, nil
}

func parseRecommendedVersionSpecs(specs []string) []libMinVersion {
	if len(specs) == 0 {
		return nil
	}
	parsed := make([]libMinVersion, 0, len(specs))
	for _, spec := range specs {
		rec, err := parseRecommendedVersionSpec(spec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "krit: invalid NewerVersionAvailable.recommendedVersions entry %q: %v\n", spec, err)
			continue
		}
		parsed = append(parsed, rec)
	}
	return parsed
}

func parseVersion(v string) (int, int, int) {
	var major, minor, patch int
	parts := strings.SplitN(v, ".", 3)
	if len(parts) >= 1 {
		fmt.Sscanf(parts[0], "%d", &major)
	}
	if len(parts) >= 2 {
		fmt.Sscanf(parts[1], "%d", &minor)
	}
	if len(parts) >= 3 {
		// Handle versions like "1.2.3-beta01"
		patchStr := parts[2]
		for i, c := range patchStr {
			if c < '0' || c > '9' {
				patchStr = patchStr[:i]
				break
			}
		}
		fmt.Sscanf(patchStr, "%d", &patch)
	}
	return major, minor, patch
}

func versionLessThan(major, minor, patch, minMajor, minMinor, minPatch int) bool {
	if major != minMajor {
		return major < minMajor
	}
	if minor != minMinor {
		return minor < minMinor
	}
	return patch < minPatch
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *NewerVersionAvailableRule) Confidence() float64 { return 0.75 }

func (r *NewerVersionAvailableRule) check(ctx *v2.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	recs := r.RecommendedVersions
	if len(recs) == 0 {
		recs = recommendedVersions
	}
	for _, dep := range cfg.Dependencies {
		if dep.Version == "" || dep.Version == "+" || strings.Contains(dep.Version, "+") {
			continue
		}
		for _, rec := range recs {
			if dep.Group == rec.Group && dep.Name == rec.Name {
				major, minor, patch := parseVersion(dep.Version)
				if versionLessThan(major, minor, patch, rec.MinMajor, rec.MinMinor, rec.MinPatch) {
					coord := dep.Group + ":" + dep.Name + ":" + dep.Version
					line := findGradleLineStr(content, coord)
					if line == 0 {
						line = 1
					}
					ctx.Emit(gradleFinding(path, line, r.BaseRule,
						fmt.Sprintf("A newer version of %s:%s is available. Update from %s to at least %s.",
							dep.Group, dep.Name, dep.Version, rec.Display)))
				}
				break
			}
		}
	}
}
