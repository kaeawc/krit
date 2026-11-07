package rules

import (
	"fmt"
	"regexp"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// JvmDefaultFlagRule flags the deprecated `-Xjvm-default` compiler argument.
//
// Sources (kotlinlang.org):
//   - whatsnew22.html and compatibility-guide-22.html: the stable `-jvm-default` option
//     (modes enable, no-compatibility, disable; default enable) replaces the deprecated
//     `-Xjvm-default` in Kotlin 2.2.0.
//   - gradle-compiler-options.html: the `jvmDefault` property and the mapping
//     all-compatibility to ENABLE, all to NO_COMPATIBILITY, disable to DISABLE.
//
// The rule reports only when the project's Kotlin version is known to be 2.2.0 or newer,
// because older compilers do not accept `-jvm-default`.
type JvmDefaultFlagRule struct {
	GradleBase
	BaseRule
}

func (r *JvmDefaultFlagRule) Confidence() float64 { return api.ConfidenceHigh }

var jvmDefaultFlagRe = regexp.MustCompile(`["']-Xjvm-default(?:=([A-Za-z-]+))?["']`)

// jvmDefaultModes maps the legacy -Xjvm-default values to the -jvm-default value and the JvmDefaultMode constant.
var jvmDefaultModes = map[string][2]string{
	"all-compatibility": {"enable", "ENABLE"},
	"all":               {"no-compatibility", "NO_COMPATIBILITY"},
	"disable":           {"disable", "DISABLE"},
}

func (r *JvmDefaultFlagRule) check(ctx *api.Context) {
	if isGradleSettingsScript(ctx.GradlePath) {
		return
	}
	src := newGradleSource(ctx.GradleContent)
	locs := jvmDefaultFlagRe.FindAllStringSubmatchIndex(src.text, -1)
	if len(locs) == 0 || !gradleToolchainFor(ctx).kotlin.atLeast(2, 2, 0) {
		return
	}
	for _, m := range locs {
		start, end := m[0], m[1]
		value := ""
		if m[2] >= 0 {
			value = src.text[m[2]:m[3]]
		}
		line, col := src.lineCol(start)
		f := scanner.Finding{
			File: ctx.GradlePath, Line: line, Col: col, StartByte: start, EndByte: end,
			Message: jvmDefaultMessage(src.text[start+1:end-1], value),
		}
		if mode, ok := jvmDefaultModes[value]; ok {
			quote := src.text[start : start+1]
			f.Fix = &scanner.Fix{
				ByteMode: true, StartByte: start, EndByte: end,
				Replacement: quote + "-jvm-default=" + mode[0] + quote,
			}
		}
		ctx.Emit(f)
	}
}

func jvmDefaultMessage(arg, value string) string {
	base := fmt.Sprintf("`%s` is deprecated since Kotlin 2.2.0, which adds the stable `-jvm-default` option.", arg)
	if mode, ok := jvmDefaultModes[value]; ok {
		return base + fmt.Sprintf(" Use `jvmDefault = JvmDefaultMode.%s` in `compilerOptions`, or `-jvm-default=%s`.", mode[1], mode[0])
	}
	return base + " Use `jvmDefault` in `compilerOptions` with the matching `JvmDefaultMode`; the Kotlin docs give no mapping for this value."
}
