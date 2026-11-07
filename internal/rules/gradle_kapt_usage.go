package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// KaptUsageRule flags kapt plugin applications and `kapt(...)` dependency configurations and, for processors that
// have a KSP variant, suggests the `ksp` configuration.
//
// Sources:
//   - kotlinlang.org/docs/ksp-kapt-migration.html: swap `kapt(...)` for `ksp(...)` and drop the kapt plugin once every
//     processor is migrated; kapt and KSP can run side by side.
//   - kotlinlang.org/docs/ksp-overview.html (supported libraries): Room, Moshi and Glide are officially supported.
//   - dagger.dev/dev-guide/ksp: Dagger and Hilt KSP support is stable as of Dagger 2.60 and KSP 2.3.9, with
//     `ksp("com.google.dagger:dagger-compiler")` and `ksp("com.google.dagger:hilt-compiler")`.
//   - kotlinlang.org/docs/kapt.html: set `kapt.include.compile.classpath=false` in gradle.properties so kapt does not
//     discover processors on the compile classpath.
//   - developer.android.com/build/migrate-to-built-in-kotlin: the kotlin-kapt plugin is incompatible with AGP 9
//     built-in Kotlin; use `com.android.legacy-kapt` until the move to KSP.
//
// Only the four processor families above are named; other processors get a generic pointer to the supported-libraries
// list, since their KSP status is not verified here.
type KaptUsageRule struct {
	GradleBase
	BaseRule
}

func (r *KaptUsageRule) Confidence() float64 { return api.ConfidenceHigh }

// kspProcessor is a kapt processor with a documented KSP variant.
type kspProcessor struct {
	group, name string
	library     string
	note        string
}

var kspProcessors = []kspProcessor{
	{"androidx.room", "room-compiler", "Room", "Room supports KSP"},
	{"com.squareup.moshi", "moshi-kotlin-codegen", "Moshi", "Moshi codegen supports KSP"},
	{"com.github.bumptech.glide", "compiler", "Glide", "Glide supports KSP"},
	{"com.google.dagger", "dagger-compiler", "Dagger", "Dagger's KSP support is stable as of Dagger 2.60 and KSP 2.3.9"},
	{"com.google.dagger", "hilt-compiler", "Hilt", "Hilt's KSP support is stable as of Dagger 2.60 and KSP 2.3.9"},
	{"com.google.dagger", "hilt-android-compiler", "Hilt", "Hilt's KSP support is stable as of Dagger 2.60 and KSP 2.3.9"},
}

var (
	kaptDependenciesBlockRe = regexp.MustCompile(`\bdependencies\s*\{`)
	kaptConfigurationRe     = regexp.MustCompile(`(?:^|[^\w.$"'])(kapt(?:[A-Z]\w*)?)[ \t]*[("']`)
	kaptCoordinateRe        = regexp.MustCompile(`^\s*\(?\s*["']([^"':]+):([^"':]+)(?::[^"']*)?["']`)
	kaptCatalogRe           = regexp.MustCompile(`^\s*\(?\s*libs\.([A-Za-z0-9_.]+)`)
)

const kaptMigrationGuide = "https://kotlinlang.org/docs/ksp-kapt-migration.html"

func (r *KaptUsageRule) check(ctx *api.Context) {
	if isGradleSettingsScript(ctx.GradlePath) {
		return
	}
	src := newGradleSource(ctx.GradleContent)
	if !strings.Contains(src.text, "kapt") {
		return
	}
	for _, app := range findPluginApplications(ctx, src, kaptPluginSpec) {
		line, col := src.lineCol(app.start)
		ctx.Emit(scanner.Finding{
			File: ctx.GradlePath, Line: line, Col: col, StartByte: app.start, EndByte: app.end,
			Message: r.pluginMessage(ctx),
		})
	}
	for _, m := range kaptDependenciesBlockRe.FindAllStringIndex(src.code, -1) {
		open := m[1] - 1
		closeIdx := src.matchingBrace(open)
		if closeIdx < 0 {
			continue
		}
		body := src.code[open:closeIdx]
		for _, c := range kaptConfigurationRe.FindAllStringSubmatchIndex(body, -1) {
			start, end := open+c[2], open+c[3]
			line, col := src.lineCol(start)
			ctx.Emit(scanner.Finding{
				File: ctx.GradlePath, Line: line, Col: col, StartByte: start, EndByte: end,
				Message: r.dependencyMessage(ctx, src, src.code[start:end], end),
			})
		}
	}
}

func (r *KaptUsageRule) pluginMessage(ctx *api.Context) string {
	msg := "The kapt plugin runs annotation processors on Java stubs. Move processors that have a KSP variant to the `ksp` configuration " +
		"and remove the plugin once none is left (" + kaptMigrationGuide + ")."
	if agp := gradleToolchainFor(ctx).agp; agp.atLeast(9, 0, 0) && !builtInKotlinOptedOut(ctx) {
		msg += " AGP 9 built-in Kotlin rejects the `kotlin-kapt` plugin: use `com.android.legacy-kapt` with the same version as AGP until then."
	}
	if v, ok := gradlePropertyValue(ctx.GradlePath, "kapt.include.compile.classpath"); !ok || !strings.EqualFold(v, "false") {
		msg += " Set `kapt.include.compile.classpath=false` in gradle.properties so kapt stops discovering processors on the compile classpath."
	}
	return msg
}

func builtInKotlinOptedOut(ctx *api.Context) bool {
	v, ok := gradlePropertyValue(ctx.GradlePath, "android.builtInKotlin")
	return ok && strings.EqualFold(v, "false")
}

// dependencyMessage names the processor's KSP variant when it is one of kspProcessors.
func (r *KaptUsageRule) dependencyMessage(ctx *api.Context, src *gradleSource, configuration string, end int) string {
	ksp := "ksp" + strings.TrimPrefix(configuration, "kapt")
	group, name := kaptCoordinate(ctx, src, end)
	for _, p := range kspProcessors {
		if p.group == group && p.name == name {
			return fmt.Sprintf("`%s` runs the %s processor through kapt. %s: use `%s` instead (%s).", configuration, p.library, p.note, ksp, kaptMigrationGuide)
		}
	}
	return fmt.Sprintf("`%s` runs this processor through kapt. Check the supported libraries at https://kotlinlang.org/docs/ksp-overview.html and use `%s` when it has a KSP variant (%s).", configuration, ksp, kaptMigrationGuide)
}

// kaptCoordinate reads the group and name of the dependency declared right after a configuration name, from a
// `"group:name:version"` literal or a `libs.<accessor>` catalog reference on the same line.
func kaptCoordinate(ctx *api.Context, src *gradleSource, end int) (group, name string) {
	rest := src.text[end:]
	if i := strings.IndexByte(rest, '\n'); i >= 0 {
		rest = rest[:i]
	}
	if m := kaptCoordinateRe.FindStringSubmatch(rest); m != nil {
		return m[1], m[2]
	}
	if m := kaptCatalogRe.FindStringSubmatch(rest); m != nil {
		if g, n, ok := gradleCatalogLibrary(ctx, m[1]); ok {
			return g, n
		}
	}
	return "", ""
}
