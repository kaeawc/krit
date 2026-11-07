package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ObsoleteKotlinVersionFlagsRule flags compiler arguments, language versions and
// gradle.properties keys that a newer Kotlin no longer needs or accepts.
//
// Sources (kotlinlang.org), one per entry in obsoleteCompilerFlags and obsoleteProperties:
//   - whatsnew21.html and whatsnew22.html: guard conditions, non-local break and continue and
//     multi-dollar interpolation need -Xwhen-guards, -Xnon-local-break-continue and
//     -Xmulti-dollar-interpolation in 2.1.0 and are stable in 2.2.0.
//   - whatsnew22.html and whatsnew24.html: context parameters need -Xcontext-parameters in 2.2.0 and are
//     stable in 2.4.0; so are explicit backing fields (-Xexplicit-backing-fields, whatsnew23.html) and the new
//     annotation use-site target defaults (-Xannotation-default-target=param-property, whatsnew22.html).
//   - whatsnew2320.html: context receivers are no longer supported in 2.3.20.
//   - compatibility-guide-22.html, -23.html and -24.html: -language-version 1.6 and 1.7 are errors from 2.2.0, 1.8
//     from 2.3.0 and 1.9 from 2.4.0 (the K1 compiler is gone).
//   - whatsnew20.html: kotlin.experimental.tryK2 became kotlin.experimental.tryNext in 2.0.0.
//   - compatibility-guide-22.html and -23.html: the kotlin.mpp.* properties listed in obsoleteProperties.
//
// -Xjvm-default is left to JvmDefaultFlag. The rule reports a construct only when the project's Kotlin version,
// lowered to the script's own language version, is known and has reached the version that made it obsolete.
type ObsoleteKotlinVersionFlagsRule struct {
	GradleBase
	BaseRule
}

func (r *ObsoleteKotlinVersionFlagsRule) Confidence() float64 { return api.ConfidenceHigh }

// obsoleteFlag describes a compiler argument that a Kotlin version made redundant or unsupported.
type obsoleteFlag struct {
	arg     string
	major   int
	minor   int
	patch   int
	removed bool   // the compiler no longer supports the feature; the flag cannot just be deleted
	note    string // what changed in that version
}

var obsoleteCompilerFlags = []obsoleteFlag{
	{"-Xwhen-guards", 2, 2, 0, false, "guard conditions in `when` are stable in Kotlin 2.2.0"},
	{"-Xnon-local-break-continue", 2, 2, 0, false, "non-local `break` and `continue` are stable in Kotlin 2.2.0"},
	{"-Xmulti-dollar-interpolation", 2, 2, 0, false, "multi-dollar string interpolation is stable in Kotlin 2.2.0"},
	{"-Xcontext-parameters", 2, 4, 0, false, "context parameters are stable in Kotlin 2.4.0"},
	{"-Xexplicit-backing-fields", 2, 4, 0, false, "explicit backing fields are stable in Kotlin 2.4.0"},
	{"-Xannotation-default-target=param-property", 2, 4, 0, false, "the new use-site target defaults for annotations are stable in Kotlin 2.4.0"},
	{"-Xcontext-receivers", 2, 3, 20, true, "context receivers are no longer supported since Kotlin 2.3.20; migrate to context parameters"},
}

// obsoleteProperty describes a gradle.properties key that a Kotlin version removed or replaced.
type obsoleteProperty struct {
	key   string
	value string // when set, only this value is reported
	major int
	minor int
	patch int
	note  string
	fix   bool
}

var obsoleteProperties = []obsoleteProperty{
	{"kotlin.experimental.tryK2", "", 2, 0, 0, "the K2 compiler is the default since Kotlin 2.0.0 and the property became kotlin.experimental.tryNext", true},
	{"kotlin.mpp.enableOptimisticNumberCommonization", "", 2, 2, 20, "the property errors since Kotlin 2.2.0 and is removed in 2.2.20", true},
	{"kotlin.mpp.enablePlatformIntegerCommonization", "", 2, 2, 20, "the property errors since Kotlin 2.2.0 and is removed in 2.2.20", true},
	{"kotlin.mpp.resourcesResolutionStrategy", "", 2, 3, 0, "the property is removed in Kotlin 2.3.0", true},
	{"kotlin.mpp.enableKotlinToolingMetadataArtifact", "", 2, 4, 0, "the property is removed in Kotlin 2.4.0 and the artifact is always generated for multiplatform projects", true},
	{"kotlin.mpp.import.enableKgpDependencyResolution", "false", 2, 3, 0, "opting out is deprecated since Kotlin 2.3.0", false},
}

var (
	obsoleteFlagLiteralRe    = regexp.MustCompile(`["'](-X[A-Za-z0-9-]+(?:=[A-Za-z0-9-]+)?)["']`)
	langVersionArgRe         = regexp.MustCompile(`["']-language-version(?:=(\d+)\.(\d+))?["'](?:\s*,\s*["'](\d+)\.(\d+)["'])?`)
	langVersionEnumRe        = regexp.MustCompile(`\blanguageVersion\s*(?:=|\.set\()\s*(?:[\w.]*\.)?KotlinVersion\.KOTLIN_(\d+)_(\d+)\b`)
	langVersionStringRe      = regexp.MustCompile(`\blanguageVersion\s*=\s*["'](\d+)\.(\d+)["']`)
	obsoleteEmptyFreeArgsRe  = regexp.MustCompile(`^[\w.]*freeCompilerArgs\s*(?:\.add\(\s*\)|\.addAll\(\s*(?:listOf\(\s*\))?\s*\)|\+=\s*(?:listOf\(\s*\))?)$`)
	obsoleteBareListElementR = regexp.MustCompile(`^,?$`)
)

func (r *ObsoleteKotlinVersionFlagsRule) check(ctx *api.Context) {
	if isGradleSettingsScript(ctx.GradlePath) {
		r.checkProperties(ctx)
		return
	}
	src := newGradleSource(ctx.GradleContent)
	candidates := findObsoleteScriptConstructs(src)
	if len(candidates) == 0 {
		return
	}
	kotlin := gradleToolchainFor(ctx).kotlin
	if !kotlin.known {
		return
	}
	// A script that pins an older language version keeps its feature flags until that version catches up.
	effective := kotlin
	if lang := scriptLanguageVersion(src); lang.known && !lang.atLeast(kotlin.major, kotlin.minor, 0) {
		effective = lang
	}
	var deletable []obsoleteCandidate
	for _, c := range candidates {
		if c.deletable && effective.atLeast(c.major, c.minor, c.patch) {
			deletable = append(deletable, c)
		}
	}
	fixes := deleteListItemFixes(src, deletable)
	for _, c := range candidates {
		version := effective
		if c.langError {
			version = kotlin
		}
		if !version.atLeast(c.major, c.minor, c.patch) {
			continue
		}
		line, col := src.lineCol(c.start)
		f := scanner.Finding{
			File: ctx.GradlePath, Line: line, Col: col, StartByte: c.start, EndByte: c.end,
			Message: c.message,
		}
		if c.langError {
			f.Severity = "error"
		}
		f.Fix = fixes[c.start]
		ctx.Emit(f)
	}
}

// obsoleteCandidate is a construct in a script and the Kotlin version from which it is obsolete.
type obsoleteCandidate struct {
	start, end   int
	major, minor int
	patch        int
	message      string
	deletable    bool
	// langError marks an unsupported language version: it is an error, and it is compared with the compiler version.
	langError bool
}

func findObsoleteScriptConstructs(src *gradleSource) []obsoleteCandidate {
	var out []obsoleteCandidate
	for _, m := range obsoleteFlagLiteralRe.FindAllStringSubmatchIndex(src.text, -1) {
		arg := src.text[m[2]:m[3]]
		for _, flag := range obsoleteCompilerFlags {
			if flag.arg != arg {
				continue
			}
			msg := fmt.Sprintf("`%s` is redundant: %s.", flag.arg, flag.note)
			if flag.removed {
				msg = fmt.Sprintf("`%s` is obsolete: %s.", flag.arg, flag.note)
			}
			out = append(out, obsoleteCandidate{
				start: m[0], end: m[1], major: flag.major, minor: flag.minor, patch: flag.patch,
				message: msg, deletable: !flag.removed,
			})
		}
	}
	for _, m := range langVersionArgRe.FindAllStringSubmatchIndex(src.text, -1) {
		maj, mnr, ok := languageVersionGroups(src.text, m, 6)
		if c, bad := languageVersionCandidate(m[0], m[1], maj, mnr, ok); bad {
			out = append(out, c)
		}
	}
	for _, re := range []*regexp.Regexp{langVersionEnumRe, langVersionStringRe} {
		for _, m := range re.FindAllStringSubmatchIndex(src.text, -1) {
			maj, mnr, ok := languageVersionGroups(src.text, m, -1)
			if c, bad := languageVersionCandidate(m[0], m[1], maj, mnr, ok); bad {
				out = append(out, c)
			}
		}
	}
	return out
}

// languageVersionFirstGroup is the submatch group index of the major version in every language-version regex.
const languageVersionFirstGroup = 2

// languageVersionGroups reads a major.minor pair from the submatch groups starting at languageVersionFirstGroup
// (and, when second >= 0, from a second pair that follows a separate "-language-version" argument).
func languageVersionGroups(text string, m []int, second int) (major, minor int, ok bool) {
	pair := func(g int) (int, int, bool) {
		if g < 0 || g+3 >= len(m) || m[g] < 0 || m[g+1] < 0 || m[g+2] < 0 || m[g+3] < 0 {
			return 0, 0, false
		}
		a, errA := strconv.Atoi(text[m[g]:m[g+1]])
		b, errB := strconv.Atoi(text[m[g+2]:m[g+3]])
		return a, b, errA == nil && errB == nil
	}
	if a, b, found := pair(languageVersionFirstGroup); found {
		return a, b, true
	}
	return pair(second)
}

// languageVersionCandidate classifies a Kotlin 1.x language version by the release that dropped it.
func languageVersionCandidate(start, end, major, minor int, ok bool) (obsoleteCandidate, bool) {
	if !ok || major != 1 {
		return obsoleteCandidate{}, false
	}
	c := obsoleteCandidate{start: start, end: end, langError: true}
	switch {
	case minor <= 7:
		c.major, c.minor = 2, 2
		c.message = fmt.Sprintf("Language version 1.%d is not supported: the compiler rejects `-language-version` 1.6 and 1.7 since Kotlin 2.2.0.", minor)
	case minor == 8:
		c.major, c.minor = 2, 3
		c.message = "Language version 1.8 is not supported: the compiler rejects `-language-version=1.8` since Kotlin 2.3.0."
	case minor == 9:
		c.major, c.minor = 2, 4
		c.message = "Language version 1.9 is not supported: Kotlin 2.4.0 drops `-language-version=1.9` together with the K1 compiler."
	default:
		return obsoleteCandidate{}, false
	}
	return c, true
}

// scriptLanguageVersion returns the language version a script pins, or an unknown version.
func scriptLanguageVersion(src *gradleSource) gradleVersion {
	var best gradleVersion
	consider := func(maj, mnr int, ok bool) {
		if !ok {
			return
		}
		v := gradleVersion{major: maj, minor: mnr, known: true}
		if !best.known || !v.atLeast(best.major, best.minor, 0) {
			best = v
		}
	}
	for _, m := range langVersionArgRe.FindAllStringSubmatchIndex(src.text, -1) {
		consider(languageVersionGroups(src.text, m, 6))
	}
	for _, re := range []*regexp.Regexp{langVersionEnumRe, langVersionStringRe} {
		for _, m := range re.FindAllStringSubmatchIndex(src.text, -1) {
			consider(languageVersionGroups(src.text, m, -1))
		}
	}
	return best
}

// deleteListItemFixes returns one fix per source line for the deletable string literals in cands, keyed by the
// start offset of the first literal on that line. Several literals on one line share a single edit so the edits
// never overlap. A line whose literals are not plain list elements gets no fix.
func deleteListItemFixes(src *gradleSource, cands []obsoleteCandidate) map[int]*scanner.Fix {
	groups := map[int][]obsoleteCandidate{}
	var order []int
	for _, c := range cands {
		lineStart, lineEnd := src.lineBounds(c.start)
		if c.end > lineEnd {
			continue
		}
		if _, seen := groups[lineStart]; !seen {
			order = append(order, lineStart)
		}
		groups[lineStart] = append(groups[lineStart], c)
	}
	fixes := map[int]*scanner.Fix{}
	for _, lineStart := range order {
		group := groups[lineStart]
		_, lineEnd := src.lineBounds(lineStart)
		line := src.text[lineStart:lineEnd]
		alone := false
		for i := len(group) - 1; i >= 0; i-- {
			var removedAlone bool
			line, removedAlone = removeListItem(line, group[i].start-lineStart, group[i].end-lineStart)
			alone = alone || removedAlone
		}
		rest := strings.TrimSpace(line)
		switch {
		case obsoleteEmptyFreeArgsRe.MatchString(rest) || obsoleteBareListElementR.MatchString(rest):
			from, to := src.lineBoundsWithNewline(lineStart)
			fixes[group[0].start] = &scanner.Fix{ByteMode: true, StartByte: from, EndByte: to}
		case !alone:
			fixes[group[0].start] = &scanner.Fix{ByteMode: true, StartByte: lineStart, EndByte: lineEnd, Replacement: line}
		}
	}
	return fixes
}

// removeListItem removes the literal line[from:to] and its separator from a comma-separated list on one line.
// alone reports that the literal had no neighbor, so the caller must check what is left of the line.
func removeListItem(line string, from, to int) (out string, alone bool) {
	trimmedAfter := strings.TrimLeft(line[to:], " \t")
	if strings.HasPrefix(trimmedAfter, ",") {
		if next := strings.TrimLeft(trimmedAfter[1:], " \t"); next != "" {
			return line[:from] + next, false
		}
	}
	if before := strings.TrimRight(line[:from], " \t"); strings.HasSuffix(before, ",") {
		return before[:len(before)-1] + line[to:], false
	}
	return line[:from] + line[to:], true
}

// checkProperties reports obsolete keys in the gradle.properties next to a settings script.
func (r *ObsoleteKotlinVersionFlagsRule) checkProperties(ctx *api.Context) {
	propsPath := filepath.Join(filepath.Dir(ctx.GradlePath), "gradle.properties")
	props := readGradleProps(propsPath)
	if props == nil {
		return
	}
	kotlin := gradleToolchainFor(ctx).kotlin
	if !kotlin.known {
		return
	}
	for _, e := range props.entries {
		for _, p := range obsoleteProperties {
			if e.key != p.key || (p.value != "" && e.value != p.value) || !kotlin.atLeast(p.major, p.minor, p.patch) {
				continue
			}
			if gradlePropSuppressed(e, r.RuleName) {
				continue
			}
			f := scanner.Finding{
				File: propsPath, Line: e.line, Col: e.keyStart - e.lineStart + 1,
				StartByte: e.keyStart, EndByte: e.keyEnd,
				Message: fmt.Sprintf("`%s` is obsolete: %s.", p.key, p.note),
			}
			if p.fix {
				f.Fix = &scanner.Fix{ByteMode: true, StartByte: e.lineStart, EndByte: e.lineEnd}
			}
			ctx.Emit(f)
		}
	}
}
