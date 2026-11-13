package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// Rule: StringInteger
// ---------------------------------------------------------------------------

// StringIntegerRule flags string values where integers are expected in Gradle
// build files — for example, minSdk = "24" instead of minSdk = 24.
type StringIntegerRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *StringIntegerRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StringIntegerRule) check(ctx *api.Context) {
	if !strings.ContainsAny(ctx.GradleContent, `"'`) {
		return
	}
	for _, st := range lintGradleStatements(ctx.GradleContent) {
		if !lintSDKPropertyInScope(st) {
			continue
		}
		literal, _, ok := lintQuotedLiteral(st.Value)
		if !ok || !lintIsDigits(literal) {
			continue
		}
		ctx.Emit(baseFinding(ctx.GradlePath, st.Line, r.BaseRule,
			fmt.Sprintf("Use an integer rather than a string here (replace `%s` with just `%s`)", st.Value, literal)))
	}
}

// lintSDKPropertyInScope reports whether st sets an SDK level where Lint checks it: min and target SDK in defaultConfig,
// compile SDK in the android closure.
func lintSDKPropertyInScope(st lintGradleStmt) bool {
	switch st.Name {
	case "minSdk", "minSdkVersion", "targetSdk", "targetSdkVersion":
		return st.Parent == "defaultConfig"
	case "compileSdk", "compileSdkVersion":
		return strings.HasPrefix(st.Parent, "android")
	}
	return false
}

var gradleLibraryAliasRe = regexp.MustCompile(`alias\s*\(\s*[\w.]*[lL]ibrary\s*\)`)

// gradleBuildIsLibrary reports a build script that applies the Android library plugin and not the
// application plugin. A library's targetSdk is not what Google Play checks.
func gradleBuildIsLibrary(content string) bool {
	if strings.Contains(content, "com.android.application") || strings.Contains(content, "applicationId") {
		return false
	}
	return strings.Contains(content, "com.android.library") || gradleLibraryAliasRe.MatchString(content)
}

// ---------------------------------------------------------------------------
// Rule: OldTargetApi
// ---------------------------------------------------------------------------

// GradleOldTargetAPIRule flags targetSdkVersion below a configurable threshold.
// Default threshold: 33 (Android 13).
type GradleOldTargetAPIRule struct {
	GradleBase
	AndroidRule
	Threshold int // minimum acceptable targetSdkVersion
}

const defaultOldTargetAPIThreshold = 33

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradleOldTargetAPIRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *GradleOldTargetAPIRule) check(ctx *api.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	threshold := r.Threshold
	if threshold == 0 {
		threshold = defaultOldTargetAPIThreshold
	}
	if cfg.TargetSdkVersion == 0 || cfg.TargetSdkVersion >= threshold || gradleBuildIsLibrary(content) {
		return
	}
	line := findGradleLineStr(content, "targetSdk")
	if line == 0 {
		line = 1
	}
	ctx.Emit(baseFinding(path, line, r.BaseRule,
		fmt.Sprintf("targetSdkVersion %d is below the recommended minimum of %d. "+
			"Update to comply with Google Play requirements.",
			cfg.TargetSdkVersion, threshold)))
}

// ---------------------------------------------------------------------------
// Rule: ExpiredTargetSdkVersion
// ---------------------------------------------------------------------------

// ExpiredTargetSdkVersionRule flags targetSdkVersion below the Play Store's
// enforced floor. Unlike OldTargetApi (which is a soft "you should update"
// warning against the current API level), this is a Fatal compliance rule
// that fires when the configured value is below a value the Play Store no
// longer accepts for new apps or updates. The floor is configurable via
// `floor` so projects on tighter or looser policies can override the default.
type ExpiredTargetSdkVersionRule struct {
	GradleBase
	AndroidRule
	Floor int // minimum acceptable targetSdkVersion under enforced policy
}

const defaultExpiredTargetSdkFloor = 34

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule:
// the targetSdkVersion is read from the parsed Gradle DSL, which varies in
// shape across plugin versions and Kotlin/Groovy syntax. Classified per
// roadmap/17.
func (r *ExpiredTargetSdkVersionRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ExpiredTargetSdkVersionRule) check(ctx *api.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	floor := r.Floor
	if floor == 0 {
		floor = defaultExpiredTargetSdkFloor
	}
	if cfg.TargetSdkVersion == 0 || cfg.TargetSdkVersion >= floor || gradleBuildIsLibrary(content) {
		return
	}
	line := findGradleLineStr(content, "targetSdk")
	if line == 0 {
		line = 1
	}
	ctx.Emit(baseFinding(path, line, r.BaseRule,
		fmt.Sprintf("Google Play requires that apps target API level %d or higher.", floor)))
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
func (r *MinSdkTooLowRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MinSdkTooLowRule) check(ctx *api.Context) {
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
	ctx.Emit(baseFinding(path, line, r.BaseRule,
		fmt.Sprintf("minSdk %d is below the recommended minimum of %d.",
			cfg.MinSdkVersion, threshold)))
}
