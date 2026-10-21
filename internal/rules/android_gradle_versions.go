package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

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
func (r *GradlePluginCompatibilityRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *GradlePluginCompatibilityRule) check(ctx *api.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	lints := android.LintBuildGradle(content, cfg, 0) // threshold 0 = skip MinSdkTooLow
	for _, l := range lints {
		if l.Rule == "GradleCompatibility" {
			line := l.Line
			if line == 0 {
				line = 1
			}
			ctx.Emit(baseFinding(path, line, r.BaseRule, l.Message))
		}
	}
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
func (r *RemoteVersionRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *RemoteVersionRule) check(ctx *api.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	for _, dep := range cfg.Dependencies {
		if dep.Version == "+" || dep.Version == "latest.release" || dep.Version == "latest.integration" {
			coord := dep.Group + ":" + dep.Name + ":" + dep.Version
			line := findGradleLineStr(content, coord)
			if line == 0 {
				line = 1
			}
			ctx.Emit(baseFinding(path, line, r.BaseRule,
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
func (r *DynamicVersionRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *DynamicVersionRule) check(ctx *api.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	depLines := gradleDependencyBlockLines(content)
	emitted := make(map[string]bool)
	emit := func(line int, label, version string) {
		key := fmt.Sprintf("%d|%s|%s", line, label, version)
		if emitted[key] {
			return
		}
		emitted[key] = true
		ctx.Emit(baseFinding(path, line, r.BaseRule,
			fmt.Sprintf("Dependency %s uses dynamic version `%s`. Pin to a specific version for reproducible builds.",
				label, version)))
	}
	findCoordLine := func(coord string) int {
		for _, ref := range depLines {
			if strings.Contains(stripGradleLineComment(ref.text), coord) {
				return ref.num
			}
		}
		return 0
	}
	for _, dep := range cfg.Dependencies {
		if !isDynamicVersionString(dep.Version) {
			continue
		}
		// Skip per CLAUDE.md "Rule Implementation Guardrails" — be lexical-state
		// aware. Coords found only in comments or outside a dependencies block
		// are not real declarations.
		line := findCoordLine(dep.Group + ":" + dep.Name + ":" + dep.Version)
		if line == 0 {
			continue
		}
		emit(line, dep.Group+":"+dep.Name, dep.Version)
	}
	// KTS named-arg form: implementation(group = "g", name = "n", version = "1.+").
	// The string-coordinate parser misses these.
	for _, ref := range depLines {
		stripped := stripGradleLineComment(ref.text)
		if !ktsDepConfigCallRe.MatchString(stripped) {
			continue
		}
		m := ktsNamedArgVersionRe.FindStringSubmatch(stripped)
		if m == nil {
			continue
		}
		v := m[1]
		if !isDynamicVersionString(v) {
			continue
		}
		emit(ref.num, ktsNamedArgLabel(stripped), v)
	}
}

// ktsNamedArgLabel extracts "group:name" from a KTS named-arg dependency call
// for use in the finding message. Falls back to "<unknown>" if either field
// is absent.
func ktsNamedArgLabel(line string) string {
	g := ktsNamedArgGroupRe.FindStringSubmatch(line)
	n := ktsNamedArgNameRe.FindStringSubmatch(line)
	group := "<unknown>"
	name := "<unknown>"
	if len(g) == 2 {
		group = g[1]
	}
	if len(n) == 2 {
		name = n[1]
	}
	return group + ":" + name
}

// isDynamicVersionString reports whether v is a non-deterministic Gradle
// dependency version that DynamicVersionRule should flag. Pure "+",
// "latest.release", and "latest.integration" are excluded because
// RemoteVersionRule covers them.
func isDynamicVersionString(v string) bool {
	if v == "" || v == "+" || v == "latest.release" || v == "latest.integration" {
		return false
	}
	if strings.Contains(v, "+") {
		return true
	}
	// Maven version ranges: [1.0,2.0), (1.0,2.0], [1.0,), (,2.0].
	if strings.ContainsAny(v, "[]()") {
		return true
	}
	// Other "latest.*" qualifiers (snapshot, milestone, etc.).
	if strings.HasPrefix(v, "latest.") {
		return true
	}
	return false
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
func (r *DeprecatedDependencyRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *DeprecatedDependencyRule) check(ctx *api.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	lints := android.LintBuildGradle(content, cfg, 0)
	for _, l := range lints {
		if l.Rule == "DeprecatedDependency" {
			line := l.Line
			if line == 0 {
				// Try to find the dependency in the content for a better line number
				line = 1
			}
			ctx.Emit(baseFinding(path, line, r.BaseRule, l.Message))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: DuplicatePlatformClasses
// ---------------------------------------------------------------------------

// platformDuplicateDeps maps `group:name` coordinates of libraries that ship
// classes already provided by the Android platform (android.jar) to a short
// remediation hint. Bundling these duplicates platform classes under the same
// fully-qualified names, which can lead to unexpected runtime crashes. The set
// is stable and well known.
var platformDuplicateDeps = map[string]string{
	"xpp3:xpp3":                             "the XmlPull API is already part of the platform",
	"commons-logging:commons-logging":       "android.util.Log is already part of the platform",
	"xerces:xmlParserAPIs":                  "the XML parser APIs are already part of the platform",
	"org.json:json":                         "org.json is already part of the platform",
	"org.khronos:opengl-api":                "the OpenGL ES APIs are already part of the platform",
	"com.google.android:android":            "this is the entire Android platform packaged as a dependency",
	"org.apache.httpcomponents:httpclient":  "use HttpUrlConnection or a library such as okhttp instead",
	"commons-httpclient:commons-httpclient": "use HttpUrlConnection or a library such as okhttp instead",
}

// DuplicatePlatformClassesRule flags dependencies that duplicate classes
// already provided by the Android platform.
type DuplicatePlatformClassesRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule:
// dependency coordinates are read from the parsed Gradle config, but build
// scripts vary in shape across plugin versions and Kotlin/Groovy syntax.
// Classified per roadmap/17.
func (r *DuplicatePlatformClassesRule) Confidence() float64 { return api.ConfidenceMedium }

// findGradleDependencyLine locates the 1-based line where a dependency
// coordinate appears. Unlike findGradleLineStr it does NOT strip string
// literals, because dependency coordinates live inside them; it still skips
// whole-line comments and any portion after a real `//` line comment.
func findGradleDependencyLine(content, coord string) int {
	for i, line := range strings.Split(content, "\n") {
		if isGradleCommentLine(line) {
			continue
		}
		scan := line
		if c := gradleLineCommentStart(line); c >= 0 {
			scan = line[:c]
		}
		if strings.Contains(scan, coord) {
			return i + 1
		}
	}
	return 0
}

func (r *DuplicatePlatformClassesRule) check(ctx *api.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	for _, dep := range cfg.Dependencies {
		coord := dep.Group + ":" + dep.Name
		hint, ok := platformDuplicateDeps[coord]
		if !ok {
			continue
		}
		// Require the coordinate to appear on a real (non-comment) line.
		// The Gradle config parser also extracts coordinates from
		// commented-out declarations, so a line of 0 here means the only
		// occurrence is inside a comment — skip it to avoid a false positive.
		line := findGradleDependencyLine(content, coord)
		if line == 0 {
			continue
		}
		ctx.Emit(baseFinding(path, line, r.BaseRule,
			fmt.Sprintf("`%s` duplicates classes already provided by the Android platform "+
				"and can cause unexpected crashes (%s). Remove it, find a version that no "+
				"longer bundles these classes, or repackage it with a tool such as jarjar.",
				coord, hint)))
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
func (r *AndroidGradlePluginVersionRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *AndroidGradlePluginVersionRule) check(ctx *api.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	matches := agpVersionRe.FindStringSubmatch(content)
	if matches == nil {
		return
	}
	major := 0
	_, _ = fmt.Sscanf(matches[1], "%d", &major)
	if major >= 7 {
		return
	}
	line := findGradleLine(content, agpVersionRe)
	if line == 0 {
		line = 1
	}
	version := matches[1] + "." + matches[2] + "." + matches[3]
	ctx.Emit(baseFinding(path, line, r.BaseRule,
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
			reporter().Warnf("krit: invalid NewerVersionAvailable.recommendedVersions entry %q: %v\n", spec, err)
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
		_, _ = fmt.Sscanf(parts[0], "%d", &major)
	}
	if len(parts) >= 2 {
		_, _ = fmt.Sscanf(parts[1], "%d", &minor)
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
		_, _ = fmt.Sscanf(patchStr, "%d", &patch)
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
func (r *NewerVersionAvailableRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NewerVersionAvailableRule) check(ctx *api.Context) {
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
					ctx.Emit(baseFinding(path, line, r.BaseRule,
						fmt.Sprintf("A newer version of %s:%s is available. Update from %s to at least %s.",
							dep.Group, dep.Name, dep.Version, rec.Display)))
				}
				break
			}
		}
	}
}
