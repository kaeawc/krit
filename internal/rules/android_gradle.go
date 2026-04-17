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

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// Gradle-rule structural contract
// ---------------------------------------------------------------------------

// GradleFamily is the structural type a rule must satisfy to be
// registered via RegisterGradle and stored in GradleRules.
// (Replaces the old `gradle rule` interface.)
type GradleFamily = interface {
	Rule
	AndroidDependencyProvider
	CheckGradle(path string, content string, cfg *android.BuildConfig) []scanner.Finding
}

// GradleBase provides a default nil implementation for Check so that
// gradle rule implementations satisfy the Rule interface without stubs.
type GradleBase struct{}

func (GradleBase) Check(file *scanner.File) []scanner.Finding { return nil }
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
func findGradleLine(content string, re *regexp.Regexp) int {
	loc := re.FindStringIndex(content)
	if loc == nil {
		return 0
	}
	return strings.Count(content[:loc[0]], "\n") + 1
}

// findGradleLineStr returns the 1-based line number of the first occurrence of substr in content, or 0.
func findGradleLineStr(content, substr string) int {
	idx := strings.Index(content, substr)
	if idx < 0 {
		return 0
	}
	return strings.Count(content[:idx], "\n") + 1
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

func (r *GradlePluginCompatibilityRule) CheckGradle(path string, content string, cfg *android.BuildConfig) []scanner.Finding {
	lints := android.LintBuildGradle(content, cfg, 0) // threshold 0 = skip MinSdkTooLow
	var findings []scanner.Finding
	for _, l := range lints {
		if l.Rule == "GradleCompatibility" {
			line := l.Line
			if line == 0 {
				line = 1
			}
			findings = append(findings, gradleFinding(path, line, r.BaseRule, l.Message))
		}
	}
	return findings
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

// stringIntegerRe matches SDK version settings assigned to quoted numeric values.
var stringIntegerRe = regexp.MustCompile(`(?m)^\s*(?:minSdk|targetSdk|compileSdk)(?:Version)?\s*[=(]\s*["'](\d+)["']`)

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *StringIntegerRule) Confidence() float64 { return 0.75 }

func (r *StringIntegerRule) CheckGradle(path string, content string, _ *android.BuildConfig) []scanner.Finding {
	var findings []scanner.Finding
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if stringIntegerRe.MatchString(line) {
			findings = append(findings, gradleFinding(path, i+1, r.BaseRule,
				"SDK version should be an integer, not a string. Remove quotes."))
		}
	}
	return findings
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

func (r *RemoteVersionRule) CheckGradle(path string, content string, cfg *android.BuildConfig) []scanner.Finding {
	var findings []scanner.Finding
	for _, dep := range cfg.Dependencies {
		if dep.Version == "+" || dep.Version == "latest.release" || dep.Version == "latest.integration" {
			coord := dep.Group + ":" + dep.Name + ":" + dep.Version
			line := findGradleLineStr(content, coord)
			if line == 0 {
				line = 1
			}
			findings = append(findings, gradleFinding(path, line, r.BaseRule,
				fmt.Sprintf("Dependency %s:%s uses non-deterministic version `%s`. Pin to a specific version.",
					dep.Group, dep.Name, dep.Version)))
		}
	}
	return findings
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

func (r *DynamicVersionRule) CheckGradle(path string, content string, cfg *android.BuildConfig) []scanner.Finding {
	var findings []scanner.Finding
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
			findings = append(findings, gradleFinding(path, line, r.BaseRule,
				fmt.Sprintf("Dependency %s:%s uses dynamic version `%s`. Pin to a specific version for reproducible builds.",
					dep.Group, dep.Name, dep.Version)))
		}
	}
	return findings
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

var targetSdkLineRe = regexp.MustCompile(`(?m)^\s*targetSdk(?:Version)?\s*[=(]`)

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradleOldTargetApiRule) Confidence() float64 { return 0.75 }

func (r *GradleOldTargetApiRule) CheckGradle(path string, content string, cfg *android.BuildConfig) []scanner.Finding {
	threshold := r.Threshold
	if threshold == 0 {
		threshold = defaultOldTargetApiThreshold
	}
	if cfg.TargetSdkVersion == 0 || cfg.TargetSdkVersion >= threshold {
		return nil
	}
	line := findGradleLine(content, targetSdkLineRe)
	if line == 0 {
		line = 1
	}
	return []scanner.Finding{gradleFinding(path, line, r.BaseRule,
		fmt.Sprintf("targetSdkVersion %d is below the recommended minimum of %d. "+
			"Update to comply with Google Play requirements.",
			cfg.TargetSdkVersion, threshold))}
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

func (r *DeprecatedDependencyRule) CheckGradle(path string, content string, cfg *android.BuildConfig) []scanner.Finding {
	lints := android.LintBuildGradle(content, cfg, 0)
	var findings []scanner.Finding
	for _, l := range lints {
		if l.Rule == "DeprecatedDependency" {
			line := l.Line
			if line == 0 {
				// Try to find the dependency in the content for a better line number
				line = 1
			}
			findings = append(findings, gradleFinding(path, line, r.BaseRule, l.Message))
		}
	}
	return findings
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

var mavenLocalLineRe = regexp.MustCompile(`(?m)^\s*mavenLocal\s*\(\s*\)`)

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *MavenLocalRule) Confidence() float64 { return 0.75 }

func (r *MavenLocalRule) CheckGradle(path string, content string, _ *android.BuildConfig) []scanner.Finding {
	if !mavenLocalLineRe.MatchString(content) {
		return nil
	}
	line := findGradleLine(content, mavenLocalLineRe)
	if line == 0 {
		line = 1
	}
	return []scanner.Finding{gradleFinding(path, line, r.BaseRule,
		"mavenLocal() can cause unreproducible builds; prefer a remote repository or includeBuild.")}
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

var minSdkLineRe = regexp.MustCompile(`(?m)^\s*minSdk(?:Version)?\s*[=(]`)

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *MinSdkTooLowRule) Confidence() float64 { return 0.75 }

func (r *MinSdkTooLowRule) CheckGradle(path string, content string, cfg *android.BuildConfig) []scanner.Finding {
	threshold := r.Threshold
	if threshold == 0 {
		threshold = defaultMinSdkTooLowThreshold
	}
	if cfg.MinSdkVersion == 0 || cfg.MinSdkVersion >= threshold {
		return nil
	}
	line := findGradleLine(content, minSdkLineRe)
	if line == 0 {
		line = 1
	}
	return []scanner.Finding{gradleFinding(path, line, r.BaseRule,
		fmt.Sprintf("minSdk %d is below the recommended minimum of %d.",
			cfg.MinSdkVersion, threshold))}
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

// deprecatedConfigRe matches lines that use a deprecated configuration as a
// function call, e.g. `compile("...")` or `compile "..."`.
var deprecatedConfigRe = regexp.MustCompile(`(?m)^\s*(compile|testCompile|provided|apk)\s*[\("']`)

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradleDeprecatedRule) Confidence() float64 { return 0.75 }

func (r *GradleDeprecatedRule) CheckGradle(path string, content string, _ *android.BuildConfig) []scanner.Finding {
	var findings []scanner.Finding
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		m := deprecatedConfigRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		cfg := m[1]
		replacement := deprecatedConfigs[cfg]
		findings = append(findings, gradleFinding(path, i+1, r.BaseRule,
			fmt.Sprintf("'%s' is deprecated; use '%s' instead.", cfg, replacement)))
	}
	return findings
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

// groovyDSLRe matches Groovy-style setter calls (name followed by space then value, not assignment).
var groovyDSLRe = regexp.MustCompile(`(?m)^\s*(compileSdkVersion|buildToolsVersion|minSdkVersion|targetSdkVersion)\s+[^=]`)

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradleGetterRule) Confidence() float64 { return 0.75 }

func (r *GradleGetterRule) CheckGradle(path string, content string, _ *android.BuildConfig) []scanner.Finding {
	// Only flag in .kts files where the Kotlin DSL should be used.
	if !strings.HasSuffix(path, ".kts") {
		return nil
	}
	var findings []scanner.Finding
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		m := groovyDSLRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		old := m[1]
		replacement := groovyStyleDSL[old]
		findings = append(findings, gradleFinding(path, i+1, r.BaseRule,
			fmt.Sprintf("Groovy-style '%s' should be replaced with '%s' in Kotlin DSL.", old, replacement)))
	}
	return findings
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

var (
	absolutePathRe  = regexp.MustCompile(`(?:files|fileTree)\s*\(\s*["']/`)
	backslashPathRe = regexp.MustCompile(`(?:files|fileTree)\s*\(\s*["'][^"']*\\`)
)

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradlePathRule) Confidence() float64 { return 0.75 }

func (r *GradlePathRule) CheckGradle(path string, content string, _ *android.BuildConfig) []scanner.Finding {
	var findings []scanner.Finding
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if absolutePathRe.MatchString(line) {
			findings = append(findings, gradleFinding(path, i+1, r.BaseRule,
				"Avoid absolute paths in files()/fileTree(); use project-relative paths."))
		} else if backslashPathRe.MatchString(line) {
			findings = append(findings, gradleFinding(path, i+1, r.BaseRule,
				"Avoid backslashes in dependency paths; use forward slashes for cross-platform compatibility."))
		}
	}
	return findings
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

func (r *GradleOverridesRule) CheckGradle(path string, content string, cfg *android.BuildConfig) []scanner.Finding {
	if cfg.MinSdkVersion > 0 && cfg.TargetSdkVersion > 0 {
		// Report on the targetSdk line (second override).
		line := findGradleLine(content, targetSdkLineRe)
		if line == 0 {
			line = 1
		}
		return []scanner.Finding{gradleFinding(path, line, r.BaseRule,
			"Both minSdk and targetSdk are set in the Gradle build file, overriding any values in AndroidManifest.xml.")}
	}
	return nil
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

var applyPluginLineRe = regexp.MustCompile(`(?m)^\s*apply\s+plugin:|^\s*apply\s*\(\s*plugin\s*=`)

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradleIdeErrorRule) Confidence() float64 { return 0.75 }

func (r *GradleIdeErrorRule) CheckGradle(path string, content string, _ *android.BuildConfig) []scanner.Finding {
	if !strings.HasSuffix(path, ".kts") {
		return nil
	}
	var findings []scanner.Finding
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if applyPluginLineRe.MatchString(line) {
			findings = append(findings, gradleFinding(path, i+1, r.BaseRule,
				"Use the plugins { } block instead of 'apply plugin:' in Kotlin DSL (.kts) files."))
		}
	}
	return findings
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

func (r *AndroidGradlePluginVersionRule) CheckGradle(path string, content string, _ *android.BuildConfig) []scanner.Finding {
	matches := agpVersionRe.FindStringSubmatch(content)
	if matches == nil {
		return nil
	}
	major := 0
	fmt.Sscanf(matches[1], "%d", &major)
	if major >= 7 {
		return nil
	}
	line := findGradleLine(content, agpVersionRe)
	if line == 0 {
		line = 1
	}
	version := matches[1] + "." + matches[2] + "." + matches[3]
	return []scanner.Finding{gradleFinding(path, line, r.BaseRule,
		fmt.Sprintf("Android Gradle Plugin version %s is too old. Upgrade to at least 7.0.0.", version))}
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

func (r *NewerVersionAvailableRule) CheckGradle(path string, content string, cfg *android.BuildConfig) []scanner.Finding {
	recs := r.RecommendedVersions
	if len(recs) == 0 {
		recs = recommendedVersions
	}
	var findings []scanner.Finding
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
					findings = append(findings, gradleFinding(path, line, r.BaseRule,
						fmt.Sprintf("A newer version of %s:%s is available. Update from %s to at least %s.",
							dep.Group, dep.Name, dep.Version, rec.Display)))
				}
				break
			}
		}
	}
	return findings
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// GradleRules holds all registered Gradle rules for use by the scanner.
var GradleRules []GradleFamily

// RegisterGradle adds a Gradle rule to both the Gradle registry and the
// global rule registry (for config/suppression compatibility).
func RegisterGradle(r GradleFamily) {
	GradleRules = append(GradleRules, r)
	Register(r)
}
