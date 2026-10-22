package rules

import (
	"fmt"
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

// deprecatedConfigs maps deprecated configuration names to their replacements.
var deprecatedConfigs = map[string]string{
	"compile":     "implementation or api",
	"testCompile": "testImplementation",
	"provided":    "compileOnly",
	"apk":         "runtimeOnly",
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
		for name, replacement := range deprecatedConfigs {
			idx := strings.Index(line, name)
			if idx < 0 {
				continue
			}
			// The name must be at a word boundary followed by `(`, `"`, or `'`
			// so that `compileOnly` doesn't match the `compile` prefix.
			rest := strings.TrimSpace(line[idx+len(name):])
			if len(rest) == 0 || (rest[0] != '(' && rest[0] != '"' && rest[0] != '\'') {
				continue
			}
			// Reject if preceded by a letter or `_` (e.g. `testCompile` won't
			// falsely trigger `compile` since we check the exact name, but be
			// careful not to match in the middle of longer words).
			if idx > 0 {
				prev := line[idx-1]
				if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') || prev == '_' {
					continue
				}
			}
			ctx.Emit(baseFinding(path, i+1, r.BaseRule,
				fmt.Sprintf("'%s' is deprecated; use '%s' instead.", name, replacement)))
			break
		}
	}
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

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradleGetterRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *GradleGetterRule) check(ctx *api.Context) {
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

// gradlePathCallContains reports whether a line contains a files() or
// fileTree() call whose string argument contains needle. We locate the
// argument by scanning past the opening `("` or `('` token and
// checking the substring from there.
func gradlePathCallContains(line, needle string) bool {
	for _, fn := range []string{"files(", "fileTree("} {
		idx := strings.Index(line, fn)
		if idx < 0 {
			continue
		}
		rest := line[idx+len(fn):]
		rest = strings.TrimSpace(rest)
		if len(rest) == 0 || (rest[0] != '"' && rest[0] != '\'') {
			continue
		}
		if strings.Contains(rest, needle) {
			return true
		}
	}
	return false
}

func (r *GradlePathRule) check(ctx *api.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	for i, line := range strings.Split(content, "\n") {
		if isGradleCommentLine(line) {
			continue
		}
		for _, fn := range []string{"files(", "fileTree("} {
			idx := strings.Index(line, fn)
			if idx < 0 {
				continue
			}
			rest := strings.TrimSpace(line[idx+len(fn):])
			if len(rest) >= 2 && (rest[0] == '"' || rest[0] == '\'') && rest[1] == '/' {
				ctx.Emit(baseFinding(path, i+1, r.BaseRule,
					"Avoid absolute paths in files()/fileTree(); use project-relative paths."))
				break
			}
		}
		if gradlePathCallContains(line, `\`) {
			ctx.Emit(baseFinding(path, i+1, r.BaseRule,
				"Avoid backslashes in dependency paths; use forward slashes for cross-platform compatibility."))
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
	if cfg.MinSdkVersion > 0 && cfg.TargetSdkVersion > 0 {
		// Report on the targetSdk line (second override).
		line := findGradleLineStr(content, "targetSdk")
		if line == 0 {
			line = 1
		}
		ctx.Emit(baseFinding(path, line, r.BaseRule,
			"Both minSdk and targetSdk are set in the Gradle build file, overriding any values in AndroidManifest.xml."))
		return
	}
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

// Confidence reports a tier-2 (medium) base confidence. Android Gradle rule. Detection scans Groovy/Kotlin DSL build scripts via
// line/regex matching; build-script shape varies by project and plugin
// version. Classified per roadmap/17.
func (r *GradleIdeErrorRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *GradleIdeErrorRule) check(ctx *api.Context) {
	path, content, _ := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	if !strings.HasSuffix(path, ".kts") {
		return
	}
	for i, line := range strings.Split(content, "\n") {
		if isGradleCommentLine(line) {
			continue
		}
		trimmed := strings.TrimSpace(line)
		// Groovy-style: `apply plugin: "..."` — invalid Kotlin syntax
		// Kotlin-legacy: `apply(plugin = "...")` — valid Kotlin but deprecated
		if strings.HasPrefix(trimmed, "apply plugin:") ||
			(strings.HasPrefix(trimmed, "apply(") && strings.Contains(trimmed, "plugin")) {
			ctx.Emit(baseFinding(path, i+1, r.BaseRule,
				"Use the plugins { } block instead of 'apply plugin:' in Kotlin DSL (.kts) files."))
		}
	}
}
