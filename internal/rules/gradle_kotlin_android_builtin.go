package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// KotlinAndroidPluginWithBuiltInKotlinRule flags the `org.jetbrains.kotlin.android` plugin in a project
// whose Android Gradle plugin (AGP) has built-in Kotlin.
//
// Sources:
//   - developer.android.com/build/releases/agp-9-0-0-release-notes#android-gradle-plugin-built-in-kotlin:
//     AGP 9.0 enables built-in Kotlin by default, so the `org.jetbrains.kotlin.android` (or `kotlin-android`)
//     plugin is no longer applied.
//   - developer.android.com/build/migrate-to-built-in-kotlin: applying the plugin fails with "The
//     'org.jetbrains.kotlin.android' plugin is no longer required for Kotlin support since AGP 9.0."; the
//     opt-out is `android.builtInKotlin=false` together with `android.newDsl=false`; a single module opts in with
//     the `com.android.built-in-kotlin` plugin; `kotlin-kapt` becomes `com.android.legacy-kapt`.
//   - kotlinlang.org/docs/compatibility-guide-23.html: Kotlin 2.3.0 deprecates the plugin for AGP 9.0.0 and later.
//
// The rule needs a known AGP version of 9.0 or newer. It skips scripts when `android.builtInKotlin=false` is set
// in a gradle.properties that applies to the script, unless the script itself applies `com.android.built-in-kotlin`.
// Plugin declarations with `apply false` only put the plugin on the classpath and are not reported.
type KotlinAndroidPluginWithBuiltInKotlinRule struct {
	GradleBase
	BaseRule
}

func (r *KotlinAndroidPluginWithBuiltInKotlinRule) Confidence() float64 { return api.ConfidenceHigh }

var (
	builtInKotlinPluginRe  = regexp.MustCompile(`["']com\.android\.built-in-kotlin["']`)
	kaptPluginSpec         = newPluginSpec([]string{"kotlin-kapt", "org.jetbrains.kotlin.kapt"}, "kapt")
	kotlinAndroidSpec      = newPluginSpec([]string{"org.jetbrains.kotlin.android", "kotlin-android"}, "android")
	kotlinMigrationBlockRe = regexp.MustCompile(`\bkotlinOptions\b|\bsourceSets\b`)
)

func (r *KotlinAndroidPluginWithBuiltInKotlinRule) check(ctx *api.Context) {
	path := ctx.GradlePath
	if isGradleSettingsScript(path) {
		return
	}
	src := newGradleSource(ctx.GradleContent)
	if !strings.Contains(src.text, "kotlin") && !strings.Contains(src.text, "alias") {
		return
	}
	agp := gradleToolchainFor(ctx).agp
	if !agp.atLeast(9, 0, 0) {
		return
	}
	matches := findPluginApplications(ctx, src, kotlinAndroidSpec)
	if len(matches) == 0 {
		return
	}
	if v, ok := gradlePropertyValue(path, "android.builtInKotlin"); ok && strings.EqualFold(v, "false") && !builtInKotlinPluginRe.MatchString(src.text) {
		return
	}
	safeToDelete := len(findPluginApplications(ctx, src, kaptPluginSpec)) == 0 && !kotlinMigrationBlockRe.MatchString(src.code)
	for _, m := range matches {
		line, col := src.lineCol(m.start)
		f := scanner.Finding{
			File: path, Line: line, Col: col, StartByte: m.start, EndByte: m.end,
			Message: fmt.Sprintf("The `org.jetbrains.kotlin.android` plugin is no longer required for Kotlin support since AGP 9.0, which has built-in Kotlin, and this project uses AGP %d.%d. "+
				"Remove it, or opt out with `android.builtInKotlin=false` and `android.newDsl=false` in gradle.properties.", agp.major, agp.minor),
		}
		if safeToDelete && m.deletable {
			from, to := src.lineBoundsWithNewline(m.start)
			f.Fix = &scanner.Fix{ByteMode: true, StartByte: from, EndByte: to}
		}
		ctx.Emit(f)
	}
}
