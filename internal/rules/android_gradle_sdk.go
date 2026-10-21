package rules

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

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
func (r *StringIntegerRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StringIntegerRule) check(ctx *api.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	for i, line := range strings.Split(content, "\n") {
		if gradleLineHasQuotedIntegerSDKProperty(line) {
			ctx.Emit(baseFinding(path, i+1, r.BaseRule,
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
	if cfg.TargetSdkVersion == 0 || cfg.TargetSdkVersion >= threshold {
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
	if cfg.TargetSdkVersion == 0 || cfg.TargetSdkVersion >= floor {
		return
	}
	line := findGradleLineStr(content, "targetSdk")
	if line == 0 {
		line = 1
	}
	ctx.Emit(baseFinding(path, line, r.BaseRule,
		fmt.Sprintf("targetSdkVersion %d is below the enforced minimum of %d. "+
			"Update to a supported value before publishing.",
			cfg.TargetSdkVersion, floor)))
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
