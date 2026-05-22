package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// WebViewAllowFileAccessRule flags WebSettings.setAllowFileAccess(true) and
// the property-assignment equivalent webView.settings.allowFileAccess = true.
// File URL access is disabled by default since API 30; explicit enablement is
// usually a security mistake.
type WebViewAllowFileAccessRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *WebViewAllowFileAccessRule) Confidence() float64 { return api.ConfidenceHigh }

const webViewAllowFileAccessMessage = "Avoid enabling WebSettings.allowFileAccess; file URL access is disabled by default since API 30 and enabling it allows javascript-served file:// URLs to read other local files."

func (r *WebViewAllowFileAccessRule) check(ctx *api.Context) {
	checkWebSettingsBoolToggle(ctx, webSettingsBoolToggleSpec{
		Setter:   "setAllowFileAccess",
		Property: "allowFileAccess",
		RuleSet:  r.RuleSetName,
		Rule:     r.RuleName,
		Severity: r.Sev,
		Message:  webViewAllowFileAccessMessage,
		Conf:     r.Confidence(),
	})
}

// callBoolArgIsTrue reports whether `call` is a single-argument call
// (Kotlin call_expression or Java method_invocation) whose only
// argument is the literal `true` after unwrapping parentheses. Every
// WebSettings rule that consumes this helper targets a Setter with
// exactly one boolean parameter (`setAllowFileAccess(boolean)` and
// friends), so insisting on arity-1 here avoids fooling the rules
// with a same-named multi-arg overload like
// `setAllowFileAccess(domain, true)`.
func callBoolArgIsTrue(file *scanner.File, call uint32) bool {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		if args == 0 || file.FlatNamedChildCount(args) != 1 {
			return false
		}
		first := file.FlatNamedChild(args, 0)
		if first == 0 {
			return false
		}
		// Kotlin value_argument wraps an expression child.
		inner := first
		if file.FlatType(first) == "value_argument" {
			if c := file.FlatNamedChild(first, 0); c != 0 {
				inner = c
			}
		}
		return isLiteralBoolTrue(file, inner)
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok {
			return false
		}
		if file.FlatNamedChildCount(args) != 1 {
			return false
		}
		first := file.FlatNamedChild(args, 0)
		return strings.TrimSpace(file.FlatNodeText(first)) == "true"
	}
	return false
}
