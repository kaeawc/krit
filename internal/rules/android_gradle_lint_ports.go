package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/versioncatalog"
)

// Android Lint GradleDetector checks that read only the text of a build script. Behaviour follows Lint 32.4.1.

// lintAnnotationProcessors are the coordinates (group:artifact) Lint knows to be annotation processors.
var lintAnnotationProcessors = lintStringSet(
	"com.jakewharton:butterknife-compiler",
	"com.github.bumptech.glide:compiler",
	"androidx.databinding:databinding-compiler",
	"com.google.dagger:dagger-compiler",
	"com.google.auto.service:auto-service",
	"android.arch.persistence.room:compiler",
	"android.arch.lifecycle:compiler",
	"io.realm:realm-annotations-processor",
	"com.google.dagger:dagger-android-processor",
	"androidx.room:room-compiler",
	"com.android.databinding:compiler",
	"androidx.lifecycle:lifecycle-compiler",
	"org.projectlombok:lombok",
	"com.google.auto.value:auto-value",
	"org.parceler:parceler",
	"com.github.hotchemi:permissionsdispatcher-processor",
	"com.alibaba:arouter-compiler",
	"org.androidannotations:androidannotations",
	"com.github.Raizlabs.DBFlow:dbflow-processor",
	"frankiesardo:icepick-processor",
	"org.greenrobot:eventbus-annotation-processor",
	"com.ryanharter.auto.value:auto-value-gson",
	"io.objectbox:objectbox-processor",
	"com.arello-mobile:moxy-compiler",
	"com.squareup.dagger:dagger-compiler",
	"io.realm:realm-android",
	"com.bluelinelabs:logansquare-compiler",
	"com.tencent.tinker:tinker-android-anno",
	"com.raizlabs.android:DBFlow-Compiler",
	"com.google.auto.factory:auto-factory",
	"com.airbnb:deeplinkdispatch-processor",
	"com.alipay.android.tools:androidannotations",
	"org.permissionsdispatcher:permissionsdispatcher-processor",
	"com.airbnb.android:epoxy-processor",
	"org.immutables:value",
	"com.github.stephanenicolas.toothpick:toothpick-compiler",
	"com.mindorks.android:placeholderview-compiler",
	"com.github.frankiesardo:auto-parcel-processor",
	"com.hannesdorfmann.fragmentargs:processor",
	"com.evernote:android-state-processor",
	"org.mapstruct:mapstruct-processor",
	"com.iqiyi.component.router:qyrouter-compiler",
	"com.iqiyi.component.mm:mm-compiler",
	"dk.ilios:realmfieldnameshelper",
	"com.lianjia.common.android.router2:compiler",
	"com.smile.gifshow.annotation:invoker_processor",
	"com.f2prateek.dart:dart-processor",
	"com.sankuai.waimai.router:compiler",
	"org.qiyi.card:card-action-compiler",
	"com.iqiyi.video:eventbus-annotation-processor",
	"ly.img.android.pesdk:build-processor",
	"org.apache.logging.log4j:log4j-core",
	"com.github.jokermonn:permissions4m",
	"com.arialyy.aria:aria-compiler",
	"com.smile.gifshow.annotation:provide_processor",
	"com.smile.gifshow.annotation:preference_processor",
	"com.smile.gifshow.annotation:plugin_processor",
	"org.inferred:freebuilder",
	"com.smile.gifshow.annotation:router_processor",
)

// lintCommonBoms are the coordinates (group:artifact) of well known bills of materials.
var lintCommonBoms = lintStringSet(
	"androidx.compose:compose-bom",
	"com.google.firebase:firebase-bom",
	"org.jetbrains.kotlin:kotlin-bom",
	"org.jetbrains.kotlinx:kotlinx-coroutines-bom",
	"io.ktor:ktor-bom",
	"com.squareup.okio:okio-bom",
	"com.squareup.okhttp3:okhttp-bom",
	"com.squareup.wire:wire-bom",
	"com.fasterxml.jackson:jackson-bom",
	"io.grpc:grpc-bom",
	"org.http4k:http4k-bom",
	"org.http4k:http4k-connect-bom",
	"org.junit:junit-bom",
	"io.kotest:kotest-bom",
	"io.cucumber:cucumber-bom",
	"io.arrow-kt:arrow-stack",
	"io.sentry:sentry-bom",
	"dev.chrisbanes.compose:compose-bom",
	"org.ow2.asm:asm-bom",
	"software.amazon.awssdk:bom",
	"com.walletconnect:android-bom",
)

func lintStringSet(items ...string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

// lintGradleScanCacheSize bounds how many parsed build scripts are kept for the rules that run on the same file.
const lintGradleScanCacheSize = 32

var lintGradleScanCache struct {
	sync.Mutex
	byContent map[string][]lintGradleStmt
	order     []string
}

// lintGradleStatements returns the statements of content, parsed once for all the rules of this file.
func lintGradleStatements(content string) []lintGradleStmt {
	c := &lintGradleScanCache
	c.Lock()
	stmts, ok := c.byContent[content]
	c.Unlock()
	if ok {
		return stmts
	}
	stmts = scanLintGradleStatements(content)
	c.Lock()
	defer c.Unlock()
	if c.byContent == nil {
		c.byContent = make(map[string][]lintGradleStmt, lintGradleScanCacheSize)
	}
	if _, dup := c.byContent[content]; !dup {
		if len(c.order) >= lintGradleScanCacheSize {
			delete(c.byContent, c.order[0])
			c.order = c.order[1:]
		}
		c.byContent[content] = stmts
		c.order = append(c.order, content)
	}
	return stmts
}

// lintGradleDep is a dependency declaration inside a dependencies closure.
type lintGradleDep struct {
	Stmt lintGradleStmt
	// Coord is the literal group:artifact[:version] of the declaration, unwrapped from platform() calls.
	Coord string
	// Quote is the quote of a bare string literal; zero for wrapped and map forms.
	Quote byte
	// Platform is true when the value is wrapped in platform() or enforcedPlatform().
	Platform bool
}

// lintGradleDependencies returns the declarations whose value is a literal coordinate, as Lint resolves them.
func lintGradleDependencies(stmts []lintGradleStmt) []lintGradleDep {
	var deps []lintGradleDep
	for _, st := range stmts {
		if st.Parent != "dependencies" || st.Value == "" {
			continue
		}
		if dep, ok := parseLintGradleDependency(st); ok {
			deps = append(deps, dep)
		}
	}
	return deps
}

// lintDependencyWrappers are the calls Lint looks through when it reads a coordinate.
var lintDependencyWrappers = []string{"platform", "testFixtures", "enforcedPlatform"}

func parseLintGradleDependency(st lintGradleStmt) (lintGradleDep, bool) {
	dep := lintGradleDep{Stmt: st}
	value := st.Value
	if coord, quote, ok := lintQuotedLiteral(value); ok {
		dep.Coord, dep.Quote = coord, quote
		return dep, true
	}
	for _, wrapper := range lintDependencyWrappers {
		if !strings.HasPrefix(value, wrapper+"(") || !strings.HasSuffix(value, ")") {
			continue
		}
		inner := strings.TrimSpace(value[len(wrapper)+1 : len(value)-1])
		if coord, _, ok := lintQuotedLiteral(inner); ok {
			dep.Coord = coord
			dep.Platform = wrapper != "testFixtures"
			return dep, true
		}
	}
	if coord, ok := lintNamedCoordinate(value); ok {
		dep.Coord = coord
		return dep, true
	}
	return dep, false
}

// lintQuotedLiteral returns the body of text when it is exactly one quoted string.
func lintQuotedLiteral(text string) (body string, quote byte, ok bool) {
	if len(text) < 2 || (text[0] != '\'' && text[0] != '"') {
		return "", 0, false
	}
	quote = text[0]
	for i := 1; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case quote:
			if i != len(text)-1 {
				return "", 0, false
			}
			return text[1:i], quote, true
		}
	}
	return "", 0, false
}

// lintNamedCoordinate reads the Groovy map notation `group: 'g', name: 'n', version: 'v'`.
func lintNamedCoordinate(value string) (string, bool) {
	if !strings.Contains(value, "group") || !strings.Contains(value, "name") {
		return "", false
	}
	fields := map[string]string{}
	for _, part := range splitGradleArguments(value) {
		key, rest, found := strings.Cut(part, ":")
		if !found {
			return "", false
		}
		literal, _, ok := lintQuotedLiteral(strings.TrimSpace(rest))
		if !ok {
			continue
		}
		fields[strings.TrimSpace(key)] = literal
	}
	group, name := fields["group"], fields["name"]
	if group == "" || name == "" {
		return "", false
	}
	if version := fields["version"]; version != "" {
		return group + ":" + name + ":" + version, true
	}
	return group + ":" + name, true
}

// splitGradleArguments splits text at commas outside quotes, brackets and parentheses.
func splitGradleArguments(text string) []string {
	var parts []string
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(text); i++ {
		c := text[i]
		if quote != 0 {
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(text[start:i]))
				start = i + 1
			}
		}
	}
	return append(parts, strings.TrimSpace(text[start:]))
}

// lintCoordinateVersion returns the version of a group:artifact:version coordinate, without a packaging suffix.
func lintCoordinateVersion(coord string) string {
	parts := strings.Split(coord, ":")
	if len(parts) < 3 {
		return ""
	}
	version, _, _ := strings.Cut(parts[2], "@")
	return version
}

// lintGradleConfigMatches reports whether a configuration is name or a variant such as debug<Suffix>.
func lintGradleConfigMatches(config, name, suffix string) bool {
	return config == name || strings.HasSuffix(config, suffix)
}

func registerLintGradleRule(rule AndroidRule, impl any, confidence float64, check func(*api.Context)) {
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Description(), Sev: api.Severity(rule.Sev),
		Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: confidence, Implementation: impl,
		Check: check,
	})
}

// ---------------------------------------------------------------------------
// NotInterpolated
// ---------------------------------------------------------------------------

// NotInterpolatedRule flags a single-quoted dependency string that contains a version variable, which Groovy does not
// interpolate.
type NotInterpolatedRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *NotInterpolatedRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NotInterpolatedRule) check(ctx *api.Context) {
	if !strings.Contains(ctx.GradleContent, "$") {
		return
	}
	for _, dep := range lintGradleDependencies(lintGradleStatements(ctx.GradleContent)) {
		if dep.Quote != '\'' || !strings.Contains(lintCoordinateVersion(dep.Coord), "$") {
			continue
		}
		ctx.Emit(baseFinding(ctx.GradlePath, dep.Stmt.Line, r.BaseRule,
			"It looks like you are trying to substitute a version variable, but using single quotes ('). For Groovy string interpolation you must use double quotes (\")."))
	}
}

// ---------------------------------------------------------------------------
// BomWithoutPlatform
// ---------------------------------------------------------------------------

// BomWithoutPlatformRule flags a well known BOM declared as a plain implementation or api dependency.
type BomWithoutPlatformRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *BomWithoutPlatformRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *BomWithoutPlatformRule) check(ctx *api.Context) {
	if !strings.Contains(ctx.GradleContent, "bom") && !strings.Contains(ctx.GradleContent, "arrow-stack") {
		return
	}
	for _, dep := range lintGradleDependencies(lintGradleStatements(ctx.GradleContent)) {
		config := dep.Stmt.Name
		if !lintGradleConfigMatches(config, "implementation", "Implementation") && !lintGradleConfigMatches(config, "api", "Api") {
			continue
		}
		if strings.HasPrefix(dep.Stmt.Value, "platform(") || strings.HasPrefix(dep.Stmt.Value, "enforcedPlatform(") {
			continue
		}
		if !lintCommonBoms[lintSubstringBeforeLast(dep.Coord, ':')] {
			continue
		}
		ctx.Emit(baseFinding(ctx.GradlePath, dep.Stmt.Line, r.BaseRule,
			"BOM should be added with a call to platform()"))
	}
}

// lintSubstringBeforeLast mirrors Kotlin's substringBeforeLast, which returns s when sep is absent.
func lintSubstringBeforeLast(s string, sep byte) string {
	if i := strings.LastIndexByte(s, sep); i >= 0 {
		return s[:i]
	}
	return s
}

// ---------------------------------------------------------------------------
// AnnotationProcessorOnCompilePath
// ---------------------------------------------------------------------------

// AnnotationProcessorOnCompilePathRule flags a known annotation processor declared on a compile classpath.
type AnnotationProcessorOnCompilePathRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *AnnotationProcessorOnCompilePathRule) Confidence() float64 { return api.ConfidenceMedium }

// lintCompileConfigurations are the configurations Lint moves processors off, in its order.
var lintCompileConfigurations = []struct{ name, suffix string }{
	{"api", "Api"}, {"compile", "Compile"}, {"implementation", "Implementation"}, {"compileOnly", "CompileOnly"},
}

func (r *AnnotationProcessorOnCompilePathRule) check(ctx *api.Context) {
	for _, dep := range lintGradleDependencies(lintGradleStatements(ctx.GradleContent)) {
		if !lintCommonAnnotationProcessor(dep.Coord) {
			continue
		}
		config := dep.Stmt.Name
		for _, c := range lintCompileConfigurations {
			if !lintGradleConfigMatches(config, c.name, c.suffix) {
				continue
			}
			replacement := "annotationProcessor"
			if config != c.name {
				replacement = strings.TrimSuffix(config, c.suffix) + "AnnotationProcessor"
			}
			ctx.Emit(baseFinding(ctx.GradlePath, dep.Stmt.Line, r.BaseRule,
				fmt.Sprintf("Add annotation processor to processor path using %s instead of %s", replacement, config)))
			break
		}
	}
}

// lintCommonAnnotationProcessor reports whether coord is group:artifact:version of a known processor.
func lintCommonAnnotationProcessor(coord string) bool {
	i := strings.LastIndexByte(coord, ':')
	return i >= 0 && lintAnnotationProcessors[coord[:i]]
}

// ---------------------------------------------------------------------------
// UseOfBundledGooglePlayServices
// ---------------------------------------------------------------------------

// UseOfBundledGooglePlayServicesRule flags the monolithic com.google.android.gms:play-services artifact.
type UseOfBundledGooglePlayServicesRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *UseOfBundledGooglePlayServicesRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UseOfBundledGooglePlayServicesRule) check(ctx *api.Context) {
	if !strings.Contains(ctx.GradleContent, "com.google.android.gms") {
		return
	}
	for _, dep := range lintGradleDependencies(lintGradleStatements(ctx.GradleContent)) {
		parts := strings.Split(dep.Coord, ":")
		if len(parts) < 2 || parts[0] != "com.google.android.gms" || parts[1] != "play-services" {
			continue
		}
		if len(parts) > 2 && !lintAtLeastBundledVersion(parts[2]) {
			continue
		}
		ctx.Emit(baseFinding(ctx.GradlePath, dep.Stmt.Line, r.BaseRule,
			"Avoid using bundled version of Google Play services SDK."))
	}
}

// lintAtLeastBundledVersion reports whether a version can resolve to 6.5.0 or later, when play-services was split up.
func lintAtLeastBundledVersion(version string) bool {
	version, _, _ = strings.Cut(version, "@")
	if version == "" || strings.Contains(version, "$") {
		return false
	}
	version = strings.TrimSuffix(strings.TrimSuffix(version, "+"), ".")
	if version == "" {
		return true
	}
	want := [3]int{6, 5, 0}
	for i, field := range strings.SplitN(version, ".", 4) {
		n, err := strconv.Atoi(field)
		if err != nil || i > 2 {
			return err == nil
		}
		if n != want[i] {
			return n > want[i]
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// DevModeObsolete
// ---------------------------------------------------------------------------

// DevModeObsoleteRule flags a `dev` flavor that sets minSdkVersion 21 only to speed up multi-dexing.
type DevModeObsoleteRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *DevModeObsoleteRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *DevModeObsoleteRule) check(ctx *api.Context) {
	if !strings.Contains(ctx.GradleContent, "dev") {
		return
	}
	for _, st := range lintGradleStatements(ctx.GradleContent) {
		if (st.Name == "minSdkVersion" || st.Name == "minSdk") && st.Parent == "dev" && st.Value == "21" {
			ctx.Emit(baseFinding(ctx.GradlePath, st.Line, r.BaseRule,
				"You no longer need a `dev` mode to enable multi-dexing during development, and this can break API version checks"))
		}
	}
}

// ---------------------------------------------------------------------------
// HighAppVersionCode
// ---------------------------------------------------------------------------

// HighAppVersionCodeRule flags a versionCode close to the largest value Google Play accepts.
type HighAppVersionCodeRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *HighAppVersionCodeRule) Confidence() float64 { return api.ConfidenceMedium }

// lintHighVersionCode is Lint's threshold for a versionCode that leaves no room to grow.
const lintHighVersionCode = 2000000000

func (r *HighAppVersionCodeRule) check(ctx *api.Context) {
	if !strings.Contains(ctx.GradleContent, "versionCode") {
		return
	}
	for _, st := range lintGradleStatements(ctx.GradleContent) {
		if st.Name != "versionCode" || st.Parent != "defaultConfig" || !lintIsDigits(st.Value) {
			continue
		}
		if n, err := strconv.ParseInt(st.Value, 10, 32); err == nil && n >= lintHighVersionCode {
			ctx.Emit(baseFinding(ctx.GradlePath, st.Line, r.BaseRule,
				"The 'versionCode' is very high and close to the max allowed value"))
		}
	}
}

func lintIsDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// DataBindingWithoutKapt
// ---------------------------------------------------------------------------

// DataBindingWithoutKaptRule flags data binding in a Kotlin module that applies no kotlin-kapt plugin.
type DataBindingWithoutKaptRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *DataBindingWithoutKaptRule) Confidence() float64 { return api.ConfidenceMedium }

// lintAppliedPluginID returns the plugin id a statement applies, for `id 'x'` and
// `alias(libs.plugins.x)` in plugins and `apply plugin: 'x'`.
func lintAppliedPluginID(st lintGradleStmt, gradlePath string) string {
	switch {
	case st.Name == "id" && st.Parent == "plugins":
		return leadingQuotedLiteral(st.Value)
	case st.Name == "alias" && st.Parent == "plugins":
		return lintCatalogPluginID(gradlePath, leadingAccessor(st.Value))
	case st.Name == "apply" && st.Parent == "":
		_, rest, found := strings.Cut(st.Value, "plugin")
		if !found {
			return ""
		}
		rest = strings.TrimLeft(rest, " \t")
		if rest == "" || (rest[0] != ':' && rest[0] != '=') {
			return ""
		}
		return leadingQuotedLiteral(strings.TrimLeft(rest[1:], " \t"))
	}
	return ""
}

// leadingAccessor returns the dotted reference that starts text, such as `libs.plugins.kotlin.kapt`.
func leadingAccessor(text string) string {
	end := 0
	for end < len(text) {
		c := text[end]
		if c != '.' && c != '_' && (c < '0' || c > '9') && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			break
		}
		end++
	}
	return text[:end]
}

// lintCatalogPluginPrefix is the only catalog accessor prefix Lint resolves plugin aliases through.
const lintCatalogPluginPrefix = "libs.plugins."

// lintCatalogPluginID resolves `libs.plugins.<key>` the way Lint does: through the nearest
// gradle/libs.versions.toml, matching a [plugins] key with `-` and `_` read as `.`.
func lintCatalogPluginID(gradlePath, accessor string) string {
	key, ok := strings.CutPrefix(accessor, lintCatalogPluginPrefix)
	if !ok || key == "" || gradlePath == "" {
		return ""
	}
	catalog := lintLibsCatalogPath(gradlePath)
	if catalog == "" {
		return ""
	}
	return lintCatalogPluginIDs(catalog)[key]
}

// lintLibsCatalogPath returns the gradle/libs.versions.toml at or above the build script's directory.
func lintLibsCatalogPath(gradlePath string) string {
	if abs, err := filepath.Abs(gradlePath); err == nil {
		gradlePath = abs
	}
	for dir := filepath.Dir(gradlePath); ; {
		candidate := filepath.Join(dir, "gradle", "libs.versions.toml")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// lintCatalogPluginIDs maps each [plugins] key, with `-` and `_` replaced by `.`, to its plugin id.
// Lint skips a table entry without a version, or whose version.ref names no [versions] entry.
func lintCatalogPluginIDs(path string) map[string]string {
	return filefacts.StringFact(fileFactsCache(), path, slotLintCatalogPlugins, func() map[string]string {
		ids := map[string]string{}
		catalog, err := versioncatalog.ParseFile(path)
		if err != nil {
			return ids
		}
		versions := make(map[string]bool, len(catalog.Versions))
		for _, v := range catalog.Versions {
			versions[v.Alias] = true
		}
		for _, p := range catalog.Plugins {
			if !p.Short && !lintCatalogVersionResolves(p.Version, versions) {
				continue
			}
			key := strings.NewReplacer("-", ".", "_", ".").Replace(p.Alias)
			if _, dup := ids[key]; !dup {
				ids[key] = p.ID
			}
		}
		return ids
	})
}

// lintCatalogVersionResolves reports whether a plugin table's version is one Lint can read.
func lintCatalogVersionResolves(v versioncatalog.Version, versions map[string]bool) bool {
	if v.Ref != "" {
		return versions[v.Ref]
	}
	return v.Plain || v.Require != "" || v.Strictly != "" || v.Prefer != "" || len(v.Reject) > 0 || v.RejectAll
}

// leadingQuotedLiteral returns the body of the string literal that starts text, ignoring what follows it.
func leadingQuotedLiteral(text string) string {
	if text == "" || (text[0] != '\'' && text[0] != '"') {
		return ""
	}
	end := strings.IndexByte(text[1:], text[0])
	if end < 0 {
		return ""
	}
	return text[1 : 1+end]
}

func (r *DataBindingWithoutKaptRule) check(ctx *api.Context) {
	if !strings.Contains(ctx.GradleContent, "dataBinding") && !strings.Contains(ctx.GradleContent, "isEnabled") {
		return
	}
	kotlin, kapt := false, false
	for _, st := range lintGradleStatements(ctx.GradleContent) {
		switch lintAppliedPluginID(st, ctx.GradlePath) {
		case "kotlin-android", "org.jetbrains.kotlin.android":
			kotlin = true
		case "kotlin-kapt", "org.jetbrains.kotlin.kapt":
			kapt = true
		}
		dataBinding := st.Parent == "dataBinding" && (st.Name == "enabled" || st.Name == "isEnabled") ||
			st.Parent == "buildFeatures" && st.Name == "dataBinding"
		if dataBinding && st.Value == "true" && kotlin && !kapt {
			ctx.Emit(baseFinding(ctx.GradlePath, st.Line, r.BaseRule,
				"If you plan to use data binding in a Kotlin project, you should apply the kotlin-kapt plugin."))
		}
	}
}

func registerLintGradleScriptChecks() {
	notInterpolated := &NotInterpolatedRule{AndroidRule: lintAndroidRule("NotInterpolated", "Incorrect Interpolation", ALCCorrectness, ALSError, 8)}
	registerLintGradleRule(notInterpolated.AndroidRule, notInterpolated, notInterpolated.Confidence(), notInterpolated.check)
	bom := &BomWithoutPlatformRule{AndroidRule: lintAndroidRule("BomWithoutPlatform", "Using a BOM without platform call", ALCCorrectness, ALSWarning, 4)}
	registerLintGradleRule(bom.AndroidRule, bom, bom.Confidence(), bom.check)
	processor := &AnnotationProcessorOnCompilePathRule{AndroidRule: lintAndroidRule("AnnotationProcessorOnCompilePath",
		"Annotation Processor on Compile Classpath", ALCPerformance, ALSWarning, 8)}
	registerLintGradleRule(processor.AndroidRule, processor, processor.Confidence(), processor.check)
	gms := &UseOfBundledGooglePlayServicesRule{AndroidRule: lintAndroidRule("UseOfBundledGooglePlayServices",
		"Use of bundled version of Google Play services", ALCPerformance, ALSWarning, 4)}
	registerLintGradleRule(gms.AndroidRule, gms, gms.Confidence(), gms.check)
	dev := &DevModeObsoleteRule{AndroidRule: lintAndroidRule("DevModeObsolete", "Dev Mode Obsolete", ALCPerformance, ALSWarning, 2)}
	registerLintGradleRule(dev.AndroidRule, dev, dev.Confidence(), dev.check)
	high := &HighAppVersionCodeRule{AndroidRule: lintAndroidRule("HighAppVersionCode", "VersionCode too high", ALCCorrectness, ALSError, 8)}
	registerLintGradleRule(high.AndroidRule, high, high.Confidence(), high.check)
	kapt := &DataBindingWithoutKaptRule{AndroidRule: lintAndroidRule("DataBindingWithoutKapt",
		"Data Binding without Annotation Processing", ALCCorrectness, ALSWarning, 1)}
	registerLintGradleRule(kapt.AndroidRule, kapt, kapt.Confidence(), kapt.check)
}
