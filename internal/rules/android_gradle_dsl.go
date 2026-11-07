package rules

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

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
func (r *MavenLocalRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MavenLocalRule) check(ctx *api.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	line := findGradleLineStr(content, "mavenLocal()")
	if line == 0 {
		return
	}
	ctx.Emit(baseFinding(path, line, r.BaseRule,
		"mavenLocal() can cause unreproducible builds; prefer a remote repository or includeBuild."))
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

// deprecatedConfigPattern finds a dependency configuration call such as `compile(...)`, `debugCompile '...'` or `provided "..."`.
var deprecatedConfigPattern = regexp.MustCompile(`(^|[^A-Za-z0-9_.])([a-z][A-Za-z0-9]*)\s*[("']`)

// deprecatedConfigMessage returns Lint's message for a deprecated configuration name, or "".
func deprecatedConfigMessage(name string) string {
	lower := func(prefix, suffix string) string {
		if prefix == "" {
			return strings.ToLower(suffix[:1]) + suffix[1:]
		}
		return prefix + suffix
	}
	switch {
	case name == "compile" || (strings.HasSuffix(name, "Compile") && len(name) > len("Compile")):
		prefix := strings.TrimSuffix(name, "Compile")
		if name == "compile" {
			prefix = ""
		}
		impl := lower(prefix, "Implementation")
		if strings.HasPrefix(name, "test") || strings.HasPrefix(name, "androidTest") {
			return fmt.Sprintf("`%s` is deprecated; replace with `%s`", name, impl)
		}
		return fmt.Sprintf("`%s` is deprecated; replace with either `%s` to maintain current behavior, or `%s` to improve build performance by not sharing this dependency transitively.",
			name, lower(prefix, "Api"), impl)
	case name == "provided" || (strings.HasSuffix(name, "Provided") && len(name) > len("Provided")):
		prefix := strings.TrimSuffix(name, "Provided")
		if name == "provided" {
			prefix = ""
		}
		return fmt.Sprintf("`%s` is deprecated; replace with `%s`", name, lower(prefix, "CompileOnly"))
	case name == "apk" || (strings.HasSuffix(name, "Apk") && len(name) > len("Apk")):
		prefix := strings.TrimSuffix(name, "Apk")
		if name == "apk" {
			prefix = ""
		}
		return fmt.Sprintf("`%s` is deprecated; replace with `%s`", name, lower(prefix, "RuntimeOnly"))
	}
	return ""
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradleDeprecatedRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *GradleDeprecatedRule) check(ctx *api.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	for i, line := range strings.Split(content, "\n") {
		if isGradleCommentLine(line) {
			continue
		}
		code := gradleCodeOutsideStrings(stripGradleLineComment(line))
		for _, m := range deprecatedConfigPattern.FindAllStringSubmatchIndex(code, -1) {
			name := code[m[4]:m[5]]
			if msg := deprecatedConfigMessage(name); msg != "" {
				ctx.Emit(baseFindingAt(path, i+1, m[4]+1, r.BaseRule, msg))
				break
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: GroovySetterInKotlinDsl
// ---------------------------------------------------------------------------

// GroovySetterInKotlinDslRule detects deprecated Groovy-style DSL property setters in
// Gradle Kotlin DSL files. For example, `compileSdkVersion 30` should be
// `compileSdk = 30` in .kts files.
//
// This is a krit-only check. It was named GradleGetter, which is the id Android
// Lint uses for a different check (see GradleGetterRule).
type GroovySetterInKotlinDslRule struct {
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
func (r *GroovySetterInKotlinDslRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *GroovySetterInKotlinDslRule) check(ctx *api.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	// Only flag in .kts files where the Kotlin DSL should be used.
	if !strings.HasSuffix(path, ".kts") {
		return
	}
	if astFile := ctx.GradleAST(); astFile != nil {
		for _, setter := range findGradleGroovySettersAST(astFile) {
			ctx.Emit(baseFinding(path, setter.line, r.BaseRule,
				fmt.Sprintf("Groovy-style '%s' should be replaced with '%s' in Kotlin DSL.", setter.name, setter.replacement)))
		}
		return
	}
	// Fallback: line/regex scan for the rare case where the .kts file did not
	// produce a flat AST.
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
			ctx.Emit(baseFinding(path, i+1, r.BaseRule,
				fmt.Sprintf("Groovy-style '%s' should be replaced with '%s' in Kotlin DSL.", name, replacement)))
			break
		}
	}
}

type gradleGroovySetter struct {
	line        int
	name        string
	replacement string
}

// findGradleGroovySettersAST finds Groovy-style positional DSL setters in a
// Kotlin DSL script via the flat AST. `compileSdkVersion 34` is invalid Kotlin,
// so tree-sitter parses the bare name as a simple_identifier statement whose
// immediate next sibling is an ERROR node (the un-parseable positional
// argument). The valid Kotlin forms `compileSdkVersion(34)` (a call) and
// `compileSdk = 34` (an assignment) parse cleanly and are not matched, and a
// name appearing inside a string or comment never parses as a bare identifier
// statement — so this is strictly more precise than substring scanning.
func findGradleGroovySettersAST(file *scanner.File) []gradleGroovySetter {
	var setters []gradleGroovySetter
	file.FlatWalkNodes(0, "simple_identifier", func(idx uint32) {
		name := file.FlatNodeString(idx, nil)
		replacement, ok := groovyStyleDSL[name]
		if !ok {
			return
		}
		parent, ok := file.FlatParent(idx)
		if !ok || file.FlatType(parent) != "statements" {
			return
		}
		if next := file.FlatNextSib(idx); next == 0 || file.FlatType(next) != "ERROR" {
			return
		}
		setters = append(setters, gradleGroovySetter{
			line:        file.FlatRow(idx) + 1,
			name:        name,
			replacement: replacement,
		})
	})
	return setters
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
func (r *GradlePathRule) Confidence() float64 { return api.ConfidenceMedium }

// gradlePathCallRe matches a files(), fileTree() or file() call whose first argument (or dir:
// argument) is a string literal; group 1 is a double-quoted body and group 2 a single-quoted body.
var gradlePathCallRe = regexp.MustCompile(`\b(?:files|fileTree|file)\s*\(\s*(?:dir\s*[:=]\s*)?(?:"((?:\\.|[^"\\])*)"|'((?:\\.|[^'\\])*)')`)

// gradleWindowsDriveRe matches a Windows drive path such as C:/libs or C:\\libs.
var gradleWindowsDriveRe = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

func (r *GradlePathRule) check(ctx *api.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	for i, line := range strings.Split(content, "\n") {
		if isGradleCommentLine(line) {
			continue
		}
		for _, m := range gradlePathCallRe.FindAllStringSubmatch(stripGradleLineComment(line), -1) {
			arg := m[1] + m[2]
			// An escaped backslash (`\\`) is a real backslash in the path; `\n` and friends are not.
			switch {
			case strings.Contains(arg, `\\`):
				ctx.Emit(baseFinding(path, i+1, r.BaseRule, "Do not use Windows file separators in .gradle files; use / instead"))
			case strings.HasPrefix(arg, "/") || gradleWindowsDriveRe.MatchString(arg):
				ctx.Emit(baseFinding(path, i+1, r.BaseRule, "Avoid using absolute paths in .gradle files"))
			}
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
func (r *GradleOverridesRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *GradleOverridesRule) check(ctx *api.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	if cfg == nil {
		return
	}
	// The build script only overrides values the module's main manifest actually declares.
	manifest, err := os.ReadFile(filepath.Join(filepath.Dir(path), "src", "main", "AndroidManifest.xml"))
	if err != nil {
		return
	}
	for _, o := range []struct {
		attr, key string
		set       bool
	}{
		{"minSdkVersion", "minSdk", cfg.MinSdkVersion > 0},
		{"targetSdkVersion", "targetSdk", cfg.TargetSdkVersion > 0},
	} {
		if !o.set || !bytes.Contains(manifest, []byte("android:"+o.attr+"=")) {
			continue
		}
		line := findGradleLineStr(content, o.key)
		if line == 0 {
			line = 1
		}
		ctx.Emit(baseFinding(path, line, r.BaseRule,
			fmt.Sprintf("This build file sets `%s`, which overrides the `android:%s` value declared in AndroidManifest.xml.", o.key, o.attr)))
	}
}

// ---------------------------------------------------------------------------
// Rule: ApplyPluginInKotlinDsl
// ---------------------------------------------------------------------------

// ApplyPluginInKotlinDslRule detects the legacy `apply plugin:` syntax in .kts
// files, which belongs in the `plugins { }` block.
//
// This is a krit-only check. It was named GradleIdeError, an id Android Lint
// defines (IDE support problems in Gradle files) but never reports in Lint 32;
// the old id stays a suppression alias and a config alias.
type ApplyPluginInKotlinDslRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *ApplyPluginInKotlinDslRule) Confidence() float64 { return api.ConfidenceMedium }

// gradleIdeApplyPluginRe matches a top-level legacy plugin application in a Kotlin DSL script.
var gradleIdeApplyPluginRe = regexp.MustCompile(`^apply(?:\s+plugin\s*:|\s*\(\s*plugin\s*=)`)

func (r *ApplyPluginInKotlinDslRule) check(ctx *api.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	if !strings.HasSuffix(path, ".kts") {
		return
	}
	for i, line := range strings.Split(content, "\n") {
		if isGradleCommentLine(line) {
			continue
		}
		// Only a top-level call can move into plugins { }; nested ones (subprojects, allprojects,
		// afterEvaluate, conditionals) have no plugins-block equivalent.
		if gradleIdeApplyPluginRe.MatchString(line) {
			ctx.Emit(baseFinding(path, i+1, r.BaseRule,
				"Use the plugins { } block instead of 'apply plugin:' in Kotlin DSL (.kts) files."))
		}
	}
}
