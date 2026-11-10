package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// defaultLocaleSpecRe is the format specifier pattern Lint's StringFormatDetector uses; group 6 is the conversion.
var defaultLocaleSpecRe = regexp.MustCompile(`%(\d+\$)?([-+#, 0(<]*)?(\d+)?(\.\d+)?([tT])?([a-zA-Z%])`)

// defaultLocaleFormatIsLocaleSpecific reports whether the format has a d, e, E, f, g, G, t or T conversion.
func defaultLocaleFormatIsLocaleSpecific(format string) bool {
	for _, m := range defaultLocaleSpecRe.FindAllStringSubmatch(format, -1) {
		if m[5] != "" || strings.ContainsAny(m[6], "deEfgGtT") {
			return true
		}
	}
	return false
}

// defaultLocaleFormatArgReportable applies Lint's LocaleDetector.checkFormat filters: the format string must be
// a constant with a locale-specific conversion, and the call must not sit in a throw or a Log call argument.
func defaultLocaleFormatArgReportable(file *scanner.File, formatExpr, call uint32) bool {
	if formatExpr == 0 {
		return false
	}
	format, ok := lintEvalString(file, formatExpr)
	if !ok || !defaultLocaleFormatIsLocaleSpecific(format) {
		return false
	}
	return !defaultLocaleInThrowOrLogArg(file, call)
}

func defaultLocaleInThrowOrLogArg(file *scanner.File, call uint32) bool {
	logChecked := false
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "throw_statement":
			return true
		case "jump_expression":
			if strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(p)), "throw") {
				return true
			}
		case "call_expression", "method_invocation":
			if !logChecked {
				logChecked = true
				if defaultLocaleIsLogCall(file, p) {
					return true
				}
			}
		}
	}
	return false
}

// defaultLocaleIsLogCall reports a single-letter android.util.Log call such as Log.d.
func defaultLocaleIsLogCall(file *scanner.File, call uint32) bool {
	name := wrongViewCastCallName(file, call)
	if len(name) != 1 {
		return false
	}
	recv := wrongViewCastCallReceiverName(file, call)
	return recv == "Log" || recv == "android.util.Log"
}

// defaultLocaleKotlinFormatArg returns the format string argument of a Kotlin String.format call.
func defaultLocaleKotlinFormatArg(file *scanner.File, args uint32) uint32 {
	first := true
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		label := flatValueArgumentLabel(file, arg)
		if label == "format" || (first && label == "") {
			return flatValueArgumentExpression(file, arg)
		}
		first = false
	}
	return 0
}
