package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ComposeCompilerRemovedFlagsRule flags Compose compiler Gradle plugin options and feature flags that Kotlin
// removed or is removing.
//
// Sources (kotlinlang.org):
//   - compatibility-guide-24.html ("Deprecate obsolete Compose compiler Gradle plugin options"): in 2.4.0 the options
//     generateFunctionKeyMetaClasses, enableIntrinsicRemember, enableNonSkippingGroupOptimization,
//     enableStrongSkippingMode, stabilityConfigurationFile and the flags ComposeFeatureFlag.StrongSkipping and
//     ComposeFeatureFlag.IntrinsicRemember report errors. Use featureFlags and stabilityConfigurationFiles instead.
//     The three enable* options warned since 2.0.20 and stabilityConfigurationFile since 2.1.0.
//   - whatsnew24.html: StrongSkipping, IntrinsicRemember and their DSL properties are DeprecationLevel.ERROR and are
//     removed in 2.5.0 because they graduated and are on by default; OptimizeNonSkippingGroups and PausableComposition
//     are deprecated in 2.4.0 and removed in 2.6.0.
//   - whatsnew22.html: the StrongSkipping and IntrinsicRemember flags are deprecated in 2.2.0.
//
// The finding severity follows the project's Kotlin version: an error from 2.4.0, a warning for earlier versions that
// already deprecate the construct, and a medium-confidence warning when the version is unknown. The two flags that
// 2.4.0 only deprecates are reported from 2.4.0 on and skipped otherwise.
type ComposeCompilerRemovedFlagsRule struct {
	GradleBase
	BaseRule
}

func (r *ComposeCompilerRemovedFlagsRule) Confidence() float64 { return api.ConfidenceHigh }

// composeConstruct describes one removed option or flag.
type composeConstruct struct {
	name string
	// warnMajor.warnMinor.warnPatch is the first version that deprecates the construct; zero when no source states it.
	warnMajor, warnMinor, warnPatch int
	// deprecatedOnly marks constructs that 2.4.0 deprecates without making them an error; they are not
	// reported for earlier versions.
	deprecatedOnly bool
	// enabledByDefault marks options whose `= true` is redundant and safe to delete.
	enabledByDefault bool
	advice           string
}

var composeOptionConstructs = []composeConstruct{
	{name: "enableStrongSkippingMode", warnMajor: 2, warnMinor: 0, warnPatch: 20, enabledByDefault: true,
		advice: "Strong skipping is on by default, so delete the option; disabling it goes away with the `ComposeFeatureFlag.StrongSkipping` opt-out in Kotlin 2.5.0"},
	{name: "enableIntrinsicRemember", warnMajor: 2, warnMinor: 0, warnPatch: 20, enabledByDefault: true,
		advice: "Intrinsic remember is on by default, so delete the option"},
	{name: "enableNonSkippingGroupOptimization", warnMajor: 2, warnMinor: 0, warnPatch: 20,
		advice: "Use `featureFlags` instead"},
	{name: "stabilityConfigurationFile", warnMajor: 2, warnMinor: 1, warnPatch: 0,
		advice: "Use `stabilityConfigurationFiles` instead"},
	{name: "generateFunctionKeyMetaClasses",
		advice: "Use `featureFlags` instead"},
}

var composeFlagConstructs = []composeConstruct{
	{name: "StrongSkipping", warnMajor: 2, warnMinor: 2, warnPatch: 0,
		advice: "Strong skipping graduated and is on by default; remove the flag, which Kotlin 2.5.0 deletes"},
	{name: "IntrinsicRemember", warnMajor: 2, warnMinor: 2, warnPatch: 0,
		advice: "Intrinsic remember graduated and is on by default; remove the flag, which Kotlin 2.5.0 deletes"},
	{name: "OptimizeNonSkippingGroups", deprecatedOnly: true,
		advice: "It is deprecated in Kotlin 2.4.0 and removed in 2.6.0"},
	{name: "PausableComposition", deprecatedOnly: true,
		advice: "It is deprecated in Kotlin 2.4.0 and removed in 2.6.0"},
}

var (
	composeCompilerBlockRe = regexp.MustCompile(`\bcomposeCompiler\s*\{`)
	composeOptionRe        = regexp.MustCompile(`\b(enableStrongSkippingMode|enableIntrinsicRemember|enableNonSkippingGroupOptimization|stabilityConfigurationFile|generateFunctionKeyMetaClasses)\b`)
	composeQualifiedOption = regexp.MustCompile(`\bcomposeCompiler\s*\.\s*(enableStrongSkippingMode|enableIntrinsicRemember|enableNonSkippingGroupOptimization|stabilityConfigurationFile|generateFunctionKeyMetaClasses)\b`)
	composeFlagRe          = regexp.MustCompile(`\bComposeFeatureFlag\s*\.\s*(StrongSkipping|IntrinsicRemember|OptimizeNonSkippingGroups|PausableComposition)\b`)
	composeTrueAssignRe    = regexp.MustCompile(`^(?:composeCompiler\s*\.\s*)?\w+\s*(?:=\s*true|\.set\(\s*true\s*\))$`)
)

func (r *ComposeCompilerRemovedFlagsRule) check(ctx *api.Context) {
	if isGradleSettingsScript(ctx.GradlePath) {
		return
	}
	src := newGradleSource(ctx.GradleContent)
	hits := composeHits(src)
	if len(hits) == 0 {
		return
	}
	kotlin := gradleToolchainFor(ctx).kotlin
	for _, h := range hits {
		severity, confidence, status, ok := composeStatus(h.construct, kotlin)
		if !ok {
			continue
		}
		line, col := src.lineCol(h.start)
		name := h.construct.name
		if h.flag {
			name = "ComposeFeatureFlag." + name
		}
		f := scanner.Finding{
			File: ctx.GradlePath, Line: line, Col: col, StartByte: h.start, EndByte: h.end,
			Severity: severity, Confidence: confidence,
			Message: fmt.Sprintf("`%s` %s. %s.", name, status, h.construct.advice),
		}
		if h.construct.enabledByDefault && !h.flag {
			f.Fix = composeDeleteTrueFix(src, h.start)
		}
		ctx.Emit(f)
	}
}

type composeHit struct {
	construct  composeConstruct
	flag       bool
	start, end int
}

func composeHits(src *gradleSource) []composeHit {
	var hits []composeHit
	seen := map[int]bool{}
	add := func(start, end int, c composeConstruct, flag bool) {
		if !seen[start] {
			seen[start] = true
			hits = append(hits, composeHit{construct: c, flag: flag, start: start, end: end})
		}
	}
	optionByName := func(name string) composeConstruct {
		for _, c := range composeOptionConstructs {
			if c.name == name {
				return c
			}
		}
		return composeConstruct{}
	}
	for _, m := range composeCompilerBlockRe.FindAllStringIndex(src.code, -1) {
		open := m[1] - 1
		closeIdx := src.matchingBrace(open)
		if closeIdx < 0 {
			continue
		}
		body := src.code[open:closeIdx]
		for _, o := range composeOptionRe.FindAllStringSubmatchIndex(body, -1) {
			add(open+o[2], open+o[3], optionByName(body[o[2]:o[3]]), false)
		}
	}
	for _, o := range composeQualifiedOption.FindAllStringSubmatchIndex(src.code, -1) {
		add(o[2], o[3], optionByName(src.code[o[2]:o[3]]), false)
	}
	for _, o := range composeFlagRe.FindAllStringSubmatchIndex(src.code, -1) {
		name := src.code[o[2]:o[3]]
		for _, c := range composeFlagConstructs {
			if c.name == name {
				add(o[0], o[3], c, true)
			}
		}
	}
	return hits
}

// composeStatus maps the construct and the Kotlin version to a severity, confidence and status phrase. ok is false
// when the construct should not be reported for that version.
func composeStatus(c composeConstruct, kotlin gradleVersion) (severity string, confidence float64, status string, ok bool) {
	if c.deprecatedOnly {
		if kotlin.atLeast(2, 4, 0) {
			return "warning", api.ConfidenceHigh, "is deprecated since Kotlin 2.4.0", true
		}
		return "", 0, "", false
	}
	switch {
	case kotlin.atLeast(2, 4, 0):
		return "error", api.ConfidenceHigh, "is an error since Kotlin 2.4.0", true
	case !kotlin.known:
		return "warning", api.ConfidenceMedium, "is an error since Kotlin 2.4.0; the project's Kotlin version is not known", true
	case c.warnMajor != 0 && kotlin.atLeast(c.warnMajor, c.warnMinor, c.warnPatch):
		return "warning", api.ConfidenceHigh, fmt.Sprintf("is deprecated since Kotlin %d.%d.%d and an error since 2.4.0", c.warnMajor, c.warnMinor, c.warnPatch), true
	}
	return "", 0, "", false
}

// composeDeleteTrueFix deletes the line of an `option = true` statement.
func composeDeleteTrueFix(src *gradleSource, off int) *scanner.Fix {
	lineStart, lineEnd := src.lineBounds(off)
	if !composeTrueAssignRe.MatchString(strings.TrimSpace(src.text[lineStart:lineEnd])) {
		return nil
	}
	from, to := src.lineBoundsWithNewline(off)
	return &scanner.Fix{ByteMode: true, StartByte: from, EndByte: to}
}
