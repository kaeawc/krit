package android

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// BuildConfig holds parsed configuration from a Gradle build file.
type BuildConfig struct {
	MinSdkVersion     int
	TargetSdkVersion  int
	CompileSdkVersion int
	Dependencies      []Dependency
	Plugins           []string
	IsAndroid         bool
	BuildFeatures     BuildFeatures
}

// Dependency represents a single external dependency declaration.
type Dependency struct {
	Group         string
	Name          string
	Version       string
	Configuration string
}

// BuildFeatures tracks Android build feature flags.
type BuildFeatures struct {
	Compose     bool
	ViewBinding bool
	DataBinding bool
	BuildConfig bool
	AidL        bool
}

// GradleLint represents a single lint finding from Gradle file analysis.
type GradleLint struct {
	Rule    string
	Message string
	Line    int
}

// --- regex patterns ---

// SDK version patterns: Kotlin DSL and Groovy DSL.
var (
	minSdkRe     = regexp.MustCompile(`(?m)^\s*minSdk(?:Version)?\s*[=(]?\s*(\d+)`)
	targetSdkRe  = regexp.MustCompile(`(?m)^\s*targetSdk(?:Version)?\s*[=(]?\s*(\d+)`)
	compileSdkRe = regexp.MustCompile(`(?m)^\s*compileSdk(?:Version)?\s*[=(]?\s*(\d+)`)
)

// Plugin patterns.
var (
	// id("com.android.application") or id("com.android.library") etc.
	pluginIdRe = regexp.MustCompile(`id\s*\(\s*["']([^"']+)["']\s*\)`)
	// alias(libs.plugins.androidApplication)
	pluginAliasRe = regexp.MustCompile(`alias\s*\(\s*([^)]+)\s*\)`)
	// apply plugin: 'com.android.application'
	applyPluginRe = regexp.MustCompile(`apply\s+plugin:\s*["']([^"']+)["']`)
)

// Dependency patterns for external (Maven) dependencies.
var (
	// implementation("group:name:version")
	depStringNotationRe = regexp.MustCompile(`(\w+)\s*\(\s*["']([^"':]+):([^"':]+):([^"']+)["']\s*\)`)
	// implementation("group:name") — no version (managed via BOM or catalog)
	depNoVersionRe = regexp.MustCompile(`(\w+)\s*\(\s*["']([^"':]+):([^"':]+)["']\s*\)`)
)

// Build features.
var (
	composeRe     = regexp.MustCompile(`(?m)^\s*compose\s*[=(]\s*true`)
	viewBindingRe = regexp.MustCompile(`(?m)^\s*viewBinding\s*[=(]\s*true`)
	dataBindingRe = regexp.MustCompile(`(?m)^\s*dataBinding\s*[=(]\s*true`)
	buildConfigRe = regexp.MustCompile(`(?m)^\s*buildConfig\s*[=(]\s*true`)
	aidlRe        = regexp.MustCompile(`(?m)^\s*aidl\s*[=(]\s*true`)
)

// Lint‐specific patterns.
var (
	mavenLocalRe = regexp.MustCompile(`(?m)^\s*mavenLocal\s*\(\s*\)`)
	agpVersionRe = regexp.MustCompile(`id\s*\(\s*["']com\.android\.[a-z]+["']\s*\)\s*version\s*["']([^"']+)["']`)
)

// Android plugin prefixes that mark a project as Android.
var androidPluginPrefixes = []string{
	"com.android.application",
	"com.android.library",
	"com.android.test",
	"com.android.dynamic-feature",
}

// Known deprecated dependencies (group:name -> recommended replacement).
var deprecatedDeps = map[string]string{
	"com.android.support:appcompat-v7":              "androidx.appcompat:appcompat",
	"com.android.support:design":                    "com.google.android.material:material",
	"com.android.support:support-v4":                "androidx.core:core / androidx.legacy:legacy-support-v4",
	"com.android.support:recyclerview-v7":           "androidx.recyclerview:recyclerview",
	"com.android.support:cardview-v7":               "androidx.cardview:cardview",
	"com.android.support:support-annotations":       "androidx.annotation:annotation",
	"com.android.support.constraint:constraint-layout": "androidx.constraintlayout:constraintlayout",
	"org.jetbrains.kotlinx:kotlinx-coroutines-core-common": "org.jetbrains.kotlinx:kotlinx-coroutines-core",
	"com.squareup.retrofit2:adapter-rxjava":         "com.squareup.retrofit2:adapter-rxjava3",
	"io.reactivex:rxjava":                           "io.reactivex.rxjava3:rxjava",
	"io.reactivex.rxjava2:rxjava":                   "io.reactivex.rxjava3:rxjava",
	"com.google.dagger:dagger":                      "com.google.dagger:hilt-android (consider migrating)",
	"junit:junit":                                   "org.junit.jupiter:junit-jupiter (JUnit 5)",
}

// ParseBuildGradle reads and parses a Gradle build file at the given path.
func ParseBuildGradle(path string) (*BuildConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read build file: %w", err)
	}
	return ParseBuildGradleContent(string(data))
}

// ParseBuildGradleContent parses Gradle build file content and returns a BuildConfig.
func ParseBuildGradleContent(content string) (*BuildConfig, error) {
	cfg := &BuildConfig{}

	// SDK versions.
	cfg.MinSdkVersion = extractInt(minSdkRe, content)
	cfg.TargetSdkVersion = extractInt(targetSdkRe, content)
	cfg.CompileSdkVersion = extractInt(compileSdkRe, content)

	// Plugins.
	cfg.Plugins = parsePlugins(content)

	// Determine if this is an Android project.
	cfg.IsAndroid = isAndroidProject(cfg.Plugins)

	// External dependencies.
	cfg.Dependencies = parseExternalDeps(content)

	// Build features.
	cfg.BuildFeatures = parseBuildFeatures(content)

	return cfg, nil
}

// extractInt finds the first match of re in content and returns the captured int, or 0.
func extractInt(re *regexp.Regexp, content string) int {
	m := re.FindStringSubmatch(content)
	if m == nil {
		return 0
	}
	v, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return v
}

// parsePlugins extracts all plugin identifiers from the content.
func parsePlugins(content string) []string {
	seen := make(map[string]bool)
	var plugins []string

	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		plugins = append(plugins, p)
	}

	for _, m := range pluginIdRe.FindAllStringSubmatch(content, -1) {
		add(m[1])
	}
	for _, m := range pluginAliasRe.FindAllStringSubmatch(content, -1) {
		add(m[1])
	}
	for _, m := range applyPluginRe.FindAllStringSubmatch(content, -1) {
		add(m[1])
	}

	return plugins
}

// isAndroidProject checks whether the plugin list contains an Android plugin.
func isAndroidProject(plugins []string) bool {
	for _, p := range plugins {
		for _, prefix := range androidPluginPrefixes {
			if p == prefix || strings.HasPrefix(p, prefix+".") {
				return true
			}
		}
	}
	return false
}

// parseExternalDeps extracts Maven-coordinate dependencies from the content.
func parseExternalDeps(content string) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)

	// With version.
	for _, m := range depStringNotationRe.FindAllStringSubmatch(content, -1) {
		key := m[1] + "|" + m[2] + ":" + m[3] + ":" + m[4]
		if seen[key] {
			continue
		}
		seen[key] = true
		deps = append(deps, Dependency{
			Configuration: m[1],
			Group:         m[2],
			Name:          m[3],
			Version:       m[4],
		})
	}

	// Without version (BOM/catalog managed).
	for _, m := range depNoVersionRe.FindAllStringSubmatch(content, -1) {
		key := m[1] + "|" + m[2] + ":" + m[3]
		if seen[key] {
			continue
		}
		// Skip if this was already captured with a version.
		if seen[m[1]+"|"+m[2]+":"+m[3]+":"] {
			continue
		}
		seen[key] = true
		deps = append(deps, Dependency{
			Configuration: m[1],
			Group:         m[2],
			Name:          m[3],
		})
	}

	return deps
}

// parseBuildFeatures detects Android buildFeatures flags.
func parseBuildFeatures(content string) BuildFeatures {
	return BuildFeatures{
		Compose:     composeRe.MatchString(content),
		ViewBinding: viewBindingRe.MatchString(content),
		DataBinding: dataBindingRe.MatchString(content),
		BuildConfig: buildConfigRe.MatchString(content),
		AidL:        aidlRe.MatchString(content),
	}
}

// --- Lint checks ---

// LintBuildGradle runs all Gradle lint rules against the given content and config.
// minSdkThreshold is the minimum acceptable minSdk value for MinSdkTooLow.
func LintBuildGradle(content string, cfg *BuildConfig, minSdkThreshold int) []GradleLint {
	var findings []GradleLint
	findings = append(findings, lintMinSdkTooLow(content, cfg, minSdkThreshold)...)
	findings = append(findings, lintDeprecatedDependency(cfg)...)
	findings = append(findings, lintMavenLocal(content)...)
	findings = append(findings, lintGradleCompatibility(content)...)
	return findings
}

// lintMinSdkTooLow flags minSdk below the configured threshold.
func lintMinSdkTooLow(content string, cfg *BuildConfig, threshold int) []GradleLint {
	if cfg.MinSdkVersion == 0 || cfg.MinSdkVersion >= threshold {
		return nil
	}
	line := findLineNumber(content, minSdkRe)
	return []GradleLint{{
		Rule:    "MinSdkTooLow",
		Message: fmt.Sprintf("minSdk %d is below the recommended minimum of %d", cfg.MinSdkVersion, threshold),
		Line:    line,
	}}
}

// lintDeprecatedDependency flags known deprecated libraries.
func lintDeprecatedDependency(cfg *BuildConfig) []GradleLint {
	var findings []GradleLint
	for _, dep := range cfg.Dependencies {
		key := dep.Group + ":" + dep.Name
		if replacement, ok := deprecatedDeps[key]; ok {
			findings = append(findings, GradleLint{
				Rule:    "DeprecatedDependency",
				Message: fmt.Sprintf("%s is deprecated; migrate to %s", key, replacement),
			})
		}
	}
	return findings
}

// lintMavenLocal flags usage of mavenLocal() repository.
func lintMavenLocal(content string) []GradleLint {
	if !mavenLocalRe.MatchString(content) {
		return nil
	}
	line := findLineNumber(content, mavenLocalRe)
	return []GradleLint{{
		Rule:    "MavenLocal",
		Message: "mavenLocal() can cause unreproducible builds; prefer a remote repository or includeBuild",
		Line:    line,
	}}
}

// Known AGP version -> minimum Gradle version compatibility table.
// Source: https://developer.android.com/build/releases/gradle-plugin
var agpGradleCompat = map[string]string{
	"8.7": "8.9",
	"8.6": "8.7",
	"8.5": "8.7",
	"8.4": "8.6",
	"8.3": "8.4",
	"8.2": "8.2",
	"8.1": "8.0",
	"8.0": "8.0",
	"7.4": "7.5",
	"7.3": "7.4",
	"7.2": "7.3.3",
	"7.1": "7.2",
	"7.0": "7.0",
}

// gradleWrapperVersionRe extracts the Gradle version from a distributionUrl in
// gradle-wrapper.properties style, but we look for gradle version comments or
// the wrapper properties if embedded. For now we detect from the build file itself
// if there is a wrapper task or comment.
var gradleVersionCommentRe = regexp.MustCompile(`Gradle\s+(\d+\.\d+)`)

// lintGradleCompatibility checks AGP version vs Gradle version when both are detectable.
func lintGradleCompatibility(content string) []GradleLint {
	agpMatch := agpVersionRe.FindStringSubmatch(content)
	if agpMatch == nil {
		return nil
	}
	agpFull := agpMatch[1]
	// Extract major.minor from AGP version.
	agpMajorMinor := majorMinor(agpFull)
	if agpMajorMinor == "" {
		return nil
	}

	requiredGradle, ok := agpGradleCompat[agpMajorMinor]
	if !ok {
		return nil
	}

	// Try to detect Gradle version from a comment or embedded reference.
	gradleMatch := gradleVersionCommentRe.FindStringSubmatch(content)
	if gradleMatch == nil {
		return nil
	}
	gradleVersion := gradleMatch[1]

	if versionLessThan(gradleVersion, requiredGradle) {
		line := findLineNumber(content, gradleVersionCommentRe)
		return []GradleLint{{
			Rule:    "GradleCompatibility",
			Message: fmt.Sprintf("AGP %s requires Gradle >= %s but found %s", agpFull, requiredGradle, gradleVersion),
			Line:    line,
		}}
	}

	return nil
}

// majorMinor extracts "X.Y" from a version string like "X.Y.Z".
func majorMinor(version string) string {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "." + parts[1]
}

// versionLessThan returns true if a < b using simple major.minor comparison.
func versionLessThan(a, b string) bool {
	aParts := strings.SplitN(a, ".", 3)
	bParts := strings.SplitN(b, ".", 3)
	if len(aParts) < 2 || len(bParts) < 2 {
		return false
	}
	aMajor, _ := strconv.Atoi(aParts[0])
	aMinor, _ := strconv.Atoi(aParts[1])
	bMajor, _ := strconv.Atoi(bParts[0])
	bMinor, _ := strconv.Atoi(bParts[1])

	if aMajor != bMajor {
		return aMajor < bMajor
	}
	return aMinor < bMinor
}

// findLineNumber returns the 1-based line number of the first match of re in content.
func findLineNumber(content string, re *regexp.Regexp) int {
	loc := re.FindStringIndex(content)
	if loc == nil {
		return 0
	}
	return strings.Count(content[:loc[0]], "\n") + 1
}
