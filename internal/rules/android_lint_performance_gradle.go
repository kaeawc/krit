package rules

import (
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/lazyre"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of Android Lint 32.4.1's Gradle-scope Performance checks: ProguardAndroidTxtDetector
// (ProguardAndroidTxtUsage), R8GradleConfigDetector (NotShrinkingResources) and GradleDetector
// (KaptUsageInsteadOfKsp, LifecycleAnnotationProcessorWithJava8). Lint reads these through its Gradle model;
// krit reads the build script and the project's Gradle facts (AGP version, version catalog).

func registerPerformanceGradleChecks() {
	proguard := &ProguardAndroidTxtUsageRule{AndroidRule: lintAndroidRule("ProguardAndroidTxtUsage",
		"Use proguard-android-optimize.txt to enable optimizations", ALCPerformance, ALSError, 2)}
	api.Register(&api.Rule{
		ID: proguard.RuleName, Category: proguard.RuleSetName, Description: proguard.Description(), Sev: api.Severity(proguard.Sev),
		Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: proguard.Confidence(), Fix: api.FixSemantic,
		Implementation: proguard, Check: proguard.check,
	})
	shrink := &NotShrinkingResourcesRule{AndroidRule: lintAndroidRule("NotShrinkingResources",
		"Use resource shrinking to optimize your resources", ALCPerformance, ALSError, 2)}
	api.Register(&api.Rule{
		ID: shrink.RuleName, Category: shrink.RuleSetName, Description: shrink.Description(), Sev: api.Severity(shrink.Sev),
		Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: shrink.Confidence(), Fix: api.FixSemantic,
		Implementation: shrink, Check: shrink.check,
	})
	ksp := &KaptUsageInsteadOfKspRule{AndroidRule: lintAndroidRule("KaptUsageInsteadOfKsp",
		"Kapt usage should be replaced with KSP", ALCPerformance, ALSWarning, 4)}
	registerLintGradleRule(ksp.AndroidRule, ksp, ksp.Confidence(), ksp.check)
	lifecycle := &LifecycleAnnotationProcessorWithJava8Rule{AndroidRule: lintAndroidRule("LifecycleAnnotationProcessorWithJava8",
		"Lifecycle Annotation Processor with Java 8 Compile Option", ALCPerformance, ALSWarning, 6)}
	registerLintGradleRule(lifecycle.AndroidRule, lifecycle, lifecycle.Confidence(), lifecycle.check)
}

// perfGradleIsLibrary reports a build script of an Android library module, which Lint's R8 and ProGuard checks skip.
func perfGradleIsLibrary(content string) bool {
	return gradleBuildIsLibrary(content) || strings.Contains(content, "com.android.kotlin.multiplatform.library")
}

// perfGradleOffset converts a 1-based line and column of content to a byte offset.
func perfGradleOffset(content string, line, col int) int {
	off := 0
	for l := 1; l < line; l++ {
		i := strings.IndexByte(content[off:], '\n')
		if i < 0 {
			return len(content)
		}
		off += i + 1
	}
	return min(off+col-1, len(content))
}

var agpClasspathRe = lazyre.New(`["']com\.android\.tools\.build:gradle:([0-9][^"'$]*)["']`)

// perfAGPVersion is the project's AGP version from the Gradle facts, or else from a buildscript
// `classpath 'com.android.tools.build:gradle:<version>'` in this script.
func perfAGPVersion(ctx *api.Context) gradleVersion {
	if agp := gradleToolchainFor(ctx).agp; agp.known {
		return agp
	}
	if m := agpClasspathRe.FindStringSubmatch(newGradleSource(ctx.GradleContent).text); m != nil {
		return parseGradleVersion(m[1])
	}
	return gradleVersion{}
}

func perfGradleFinding(ctx *api.Context, line, col int, severity, msg string) scanner.Finding {
	start := perfGradleOffset(ctx.GradleContent, line, col)
	return scanner.Finding{File: ctx.GradlePath, Line: line, Col: col, StartByte: start, EndByte: start,
		Severity: severity, Message: msg}
}

// ---------------------------------------------------------------------------
// ProguardAndroidTxtUsage
// ---------------------------------------------------------------------------

// ProguardAndroidTxtUsageRule mirrors Lint's ProguardAndroidTxtUsage: `getDefaultProguardFile('proguard-android.txt')`
// in an app module, whose `-dontoptimize` turns off most R8 optimizations.
type ProguardAndroidTxtUsageRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports high confidence: the call and its argument are read from the script.
func (r *ProguardAndroidTxtUsageRule) Confidence() float64 { return api.ConfidenceHigh }

var proguardDefaultFileRe = lazyre.New(`\bgetDefaultProguardFile\s*\(`)

func (r *ProguardAndroidTxtUsageRule) check(ctx *api.Context) {
	if isGradleSettingsScript(ctx.GradlePath) || !strings.Contains(ctx.GradleContent, "proguard-android.txt") ||
		perfGradleIsLibrary(ctx.GradleContent) {
		return
	}
	src := newGradleSource(ctx.GradleContent)
	// The Lint command line downgrades the issue to a warning before AGP 9, where the file is still supported.
	severity := ""
	if agp := perfAGPVersion(ctx); agp.known && !agp.atLeast(9, 0, 0) {
		severity = "warning"
	}
	for _, m := range proguardDefaultFileRe.FindAllStringIndex(src.code, -1) {
		open := m[1] - 1
		closeIdx := matchingCloseParen(src.code, open)
		if closeIdx < 0 {
			continue
		}
		args := src.text[open+1 : closeIdx]
		at := strings.Index(args, "proguard-android.txt")
		if at < 0 {
			continue
		}
		line, col := src.lineCol(m[0])
		f := scanner.Finding{File: ctx.GradlePath, Line: line, Col: col, StartByte: m[0], EndByte: closeIdx + 1,
			Severity: severity, Message: "Avoid `getDefaultProguardFile('proguard-android.txt')`"}
		name := open + 1 + at
		f.Fix = &scanner.Fix{ByteMode: true, StartByte: name, EndByte: name + len("proguard-android.txt"),
			Replacement: "proguard-android-optimize.txt"}
		ctx.Emit(f)
	}
}

// ---------------------------------------------------------------------------
// NotShrinkingResources
// ---------------------------------------------------------------------------

// NotShrinkingResourcesRule mirrors Lint's NotShrinkingResources: an app module that enables minification without
// resource shrinking, or sets shrinkResources to false next to minifyEnabled = true.
type NotShrinkingResourcesRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports high confidence: both properties are read from the script.
func (r *NotShrinkingResourcesRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *NotShrinkingResourcesRule) check(ctx *api.Context) {
	if isGradleSettingsScript(ctx.GradlePath) || !strings.Contains(ctx.GradleContent, "inifyEnabled") ||
		perfGradleIsLibrary(ctx.GradleContent) {
		return
	}
	kts := isKotlinDslScript(ctx.GradlePath)
	minifyName, shrinkName := "minifyEnabled", "shrinkResources"
	if kts {
		minifyName, shrinkName = "isMinifyEnabled", "isShrinkResources"
	}
	var lastMinify *lintGradleStmt
	minifyParents := map[string]bool{}
	var shrinkFalse []lintGradleStmt
	seenShrink := false
	for _, st := range lintGradleStatements(ctx.GradleContent) {
		switch st.Name {
		case shrinkName:
			seenShrink = true
			if st.Value == "false" {
				shrinkFalse = append(shrinkFalse, st)
			}
		case minifyName:
			if st.Value == "true" {
				minifyParents[st.Parent] = true
				st := st
				lastMinify = &st
			}
		}
	}
	// The Lint command line reports both cases as warnings.
	for _, st := range shrinkFalse {
		if !minifyParents[st.Parent] {
			continue
		}
		f := perfGradleFinding(ctx, st.Line, st.ValueCol, "warning", "Avoid setting "+shrinkName+" = false")
		f.EndByte = f.StartByte + len("false")
		f.Fix = &scanner.Fix{ByteMode: true, StartByte: f.StartByte, EndByte: f.EndByte, Replacement: "true"}
		ctx.Emit(f)
	}
	// Lint reports a missing shrinkResources only before AGP 10, the last minifyEnabled = true of the file.
	if lastMinify == nil || seenShrink || perfAGPVersion(ctx).atLeast(10, 0, 0) {
		return
	}
	f := perfGradleFinding(ctx, lastMinify.Line, lastMinify.Col, "warning",
		"If enabling minification, also set "+shrinkName+" = true")
	value := perfGradleOffset(ctx.GradleContent, lastMinify.Line, lastMinify.ValueCol) + len("true")
	f.EndByte = value
	f.Fix = &scanner.Fix{ByteMode: true, StartByte: value, EndByte: value,
		Replacement: "\n" + strings.Repeat(" ", lastMinify.Col-1) + shrinkName + " = true"}
	ctx.Emit(f)
}

// ---------------------------------------------------------------------------
// KaptUsageInsteadOfKsp
// ---------------------------------------------------------------------------

// KaptUsageInsteadOfKspRule mirrors Lint's KaptUsageInsteadOfKsp: a `kapt` dependency on an annotation processor
// that Lint knows has a KSP version. krit's KaptUsage covers every kapt use with its own processor list; this rule
// keeps Lint's narrower verdicts under Lint's ID.
type KaptUsageInsteadOfKspRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports high confidence: the configuration and coordinate are read from the script or catalog.
func (r *KaptUsageInsteadOfKspRule) Confidence() float64 { return api.ConfidenceHigh }

// lintKspReplacements are GradleDetector's annotationProcessorsWithKspReplacements keys.
var lintKspReplacements = lintStringSet(
	"com.github.bumptech.glide:compiler",
	"androidx.room:room-compiler",
	"com.squareup.moshi:moshi-kotlin-codegen",
	"com.github.liujingxing.rxhttp:rxhttp-compiler",
	"se.ansman.kotshi:compiler",
	"com.linecorp.lich:savedstate-compiler",
	"io.github.amrdeveloper:easyadapter-compiler",
	"com.airbnb:deeplinkdispatch-processor",
	"com.airbnb.android:epoxy-processor",
	"com.airbnb.android:paris-processor",
)

var kaptCatalogValueRe = lazyre.New(`^libs\.([A-Za-z0-9_.]+)$`)

func (r *KaptUsageInsteadOfKspRule) check(ctx *api.Context) {
	if isGradleSettingsScript(ctx.GradlePath) || !strings.Contains(ctx.GradleContent, "kapt") {
		return
	}
	for _, st := range lintGradleStatements(ctx.GradleContent) {
		if st.Name != "kapt" || st.Parent != "dependencies" || st.Value == "" {
			continue
		}
		module := ""
		if dep, ok := parseLintGradleDependency(st); ok {
			module = lintModuleOf(dep.Coord)
		} else if m := kaptCatalogValueRe.FindStringSubmatch(strings.TrimSpace(st.Value)); m != nil {
			if group, name, ok := gradleCatalogLibrary(ctx, m[1]); ok {
				module = group + ":" + name
			}
		}
		if !lintKspReplacements[module] {
			continue
		}
		ctx.Emit(perfGradleFinding(ctx, st.Line, st.Col, "", "This library supports using KSP instead of kapt, which "+
			"greatly improves performance. Learn more: https://developer.android.com/studio/build/migrate-to-ksp"))
	}
}

// lintModuleOf drops the version (the last segment), leaving group:name, as Lint's substringBeforeLast(':') does.
func lintModuleOf(coord string) string {
	if strings.Count(coord, ":") < 2 {
		return coord
	}
	return coord[:strings.LastIndex(coord, ":")]
}

// ---------------------------------------------------------------------------
// LifecycleAnnotationProcessorWithJava8
// ---------------------------------------------------------------------------

// LifecycleAnnotationProcessorWithJava8Rule mirrors Lint's LifecycleAnnotationProcessorWithJava8: a dependency on
// the Lifecycle annotation processor in a module that compiles Java 8 or later, where lifecycle-common's
// DefaultLifecycleObserver builds faster.
type LifecycleAnnotationProcessorWithJava8Rule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the Java level comes from the script or the AGP default.
func (r *LifecycleAnnotationProcessorWithJava8Rule) Confidence() float64 { return api.ConfidenceMedium }

func (r *LifecycleAnnotationProcessorWithJava8Rule) check(ctx *api.Context) {
	if isGradleSettingsScript(ctx.GradlePath) || !strings.Contains(ctx.GradleContent, "lifecycle") {
		return
	}
	stmts := lintGradleStatements(ctx.GradleContent)
	java8 := false
	checked := false
	for _, st := range stmts {
		if st.Parent != "dependencies" {
			continue
		}
		coord, _, ok := lintQuotedLiteral(strings.TrimSpace(st.Value))
		if !ok || !strings.Contains(coord, "android.arch.lifecycle:compiler") &&
			!strings.Contains(coord, "androidx.lifecycle:lifecycle-compiler") {
			continue
		}
		if !checked {
			java8, checked = perfTargetsJava8(ctx, stmts), true
		}
		if !java8 {
			return
		}
		ctx.Emit(perfGradleFinding(ctx, st.Line, st.ValueCol, "", "Use the Lifecycle Java 8 API provided by the "+
			"`lifecycle-common` library instead of Lifecycle annotations for faster incremental build."))
	}
}

var (
	javaVersionRe        = lazyre.New(`(?:VERSION_1_(\d+)|VERSION_(\d+)|^["']?1\.(\d+)["']?$|^["']?(\d+)["']?$|toVersion\(\s*["']?(?:1\.)?(\d+)["']?\s*\))`)
	javaToolchainLevelRe = lazyre.New(`\d+`)
)

// perfJavaLevel parses a Java version written as JavaVersion.VERSION_1_8, VERSION_17, "1.8", 11 or
// JavaVersion.toVersion(17); 0 when it is not a literal.
func perfJavaLevel(value string) int {
	m := javaVersionRe.FindStringSubmatch(strings.TrimSpace(value))
	if m == nil {
		return 0
	}
	for _, g := range m[1:] {
		if g != "" {
			n, _ := strconv.Atoi(g)
			return n
		}
	}
	return 0
}

// perfTargetsJava8 reports whether the module compiles Java 8 or later: its compileOptions sourceCompatibility, a
// Java toolchain, or else the default of AGP 4.2 and later (Java 8). Lint reads the level from the Gradle model and
// treats an unknown level as Java 7, so an unknown AGP version with no declaration does not count.
func perfTargetsJava8(ctx *api.Context, stmts []lintGradleStmt) bool {
	for _, st := range stmts {
		switch {
		case st.Name == "sourceCompatibility":
			if level := perfJavaLevel(st.Value); level > 0 {
				return level >= 8
			}
		case st.Name == "jvmToolchain" || st.Name == "languageVersion" && strings.Contains(st.Value, "JavaLanguageVersion"):
			if m := javaToolchainLevelRe.FindString(st.Value); m != "" {
				n, _ := strconv.Atoi(m)
				return n >= 8
			}
		}
	}
	return gradleToolchainFor(ctx).agp.atLeast(4, 2, 0)
}
