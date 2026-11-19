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

// defaultLocaleInThrowOrLogArg mirrors Lint: the call sits anywhere under a throw, or the nearest call it is an
// argument of is a single-letter android.util.Log call. A call it is only the receiver of (`String.format(...).trim()`)
// is not that call: UAST has it as the selector of a qualified expression, not as a parent call.
func defaultLocaleInThrowOrLogArg(file *scanner.File, call uint32) bool {
	logChecked := false
	for child := call; ; {
		p, ok := file.FlatParent(child)
		if !ok {
			return false
		}
		switch file.FlatType(p) {
		case "throw_statement":
			return true
		case "jump_expression":
			if strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(p)), "throw") {
				return true
			}
		case "call_expression", "method_invocation":
			if !logChecked && !defaultLocaleIsReceiverOf(file, p, child) {
				logChecked = true
				if defaultLocaleIsLogCall(file, p) {
					return true
				}
			}
		}
		child = p
	}
}

// defaultLocaleIsReceiverOf reports whether child is the callee part of call (its receiver chain), not an argument.
func defaultLocaleIsReceiverOf(file *scanner.File, call, child uint32) bool {
	if file.FlatType(call) == "method_invocation" {
		args, _ := file.FlatFindChild(call, "argument_list")
		return child != args
	}
	return child == file.FlatFirstChild(call)
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
