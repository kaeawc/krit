package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// isWebSettingsReceiver reports whether the navigation chain or method
// receiver behind a WebSettings setter call (or property assignment)
// resolves to an android.webkit.WebSettings instance.
//
// Detection is structural with import gating, not type-resolved:
//   - Kotlin: the navigation chain has a `.settings` segment immediately
//     before the setter/property — e.g. `webView.settings.setX(true)` or
//     `webView.settings.foo = true`.
//   - Java: the receiver text contains `getSettings()` —
//     e.g. `webView.getSettings().setX(true)`.
//   - Either language: the receiver root identifier is named like a
//     WebSettings reference (`settings`, `webSettings`, `ws`) AND the file
//     imports `android.webkit.WebSettings`.
//
// The file must import android.webkit.WebSettings or android.webkit.WebView
// for any positive result; this guards against unrelated classes that
// happen to expose a `settings` property.
func isWebSettingsReceiver(file *scanner.File, receiverChain string) bool {
	if file == nil || receiverChain == "" {
		return false
	}
	if !sourceImportsOrMentions(file, "android.webkit.WebSettings") &&
		!sourceImportsOrMentions(file, "android.webkit.WebView") {
		return false
	}

	chain := strings.TrimSpace(receiverChain)
	// Java: getSettings() somewhere in the receiver chain.
	if strings.Contains(chain, "getSettings()") {
		return true
	}
	// Kotlin: chain ends with `.settings` or is bare `settings`.
	if chain == "settings" || strings.HasSuffix(chain, ".settings") {
		return true
	}
	// Variable named like a WebSettings.
	root := chain
	if dot := strings.Index(chain, "."); dot >= 0 {
		root = chain[:dot]
	}
	switch root {
	case "settings", "webSettings", "ws", "mSettings", "mWebSettings":
		return true
	}
	return false
}

// kotlinCallReceiverChain returns the dotted receiver chain leading up to a
// Kotlin call_expression's outermost selector. For `a.b.c.foo()` it returns
// "a.b.c"; for `foo()` it returns "". The chain text is the source slice and
// preserves any function calls inside (e.g. `getSettings()`).
func kotlinCallReceiverChain(file *scanner.File, call uint32) string {
	if file == nil || file.FlatType(call) != "call_expression" {
		return ""
	}
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return ""
	}
	receiver := file.FlatNamedChild(navExpr, 0)
	if receiver == 0 {
		return ""
	}
	return strings.TrimSpace(file.FlatNodeText(receiver))
}

// kotlinAssignmentTargetChain returns the dotted chain on the left side of a
// Kotlin assignment node. For `a.b.c = true` it returns "a.b.c"; for
// `x = true` it returns "x".
func kotlinAssignmentTargetChain(file *scanner.File, assignment uint32) string {
	if file == nil || file.FlatType(assignment) != "assignment" {
		return ""
	}
	for child := file.FlatFirstChild(assignment); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "directly_assignable_expression", "navigation_expression", "simple_identifier":
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return ""
}

// chainSplitTrailing splits a dotted chain into (head, tail) at the final
// dot. For "a.b.c" it returns ("a.b", "c"); for "x" it returns ("", "x").
func chainSplitTrailing(chain string) (head, tail string) {
	dot := strings.LastIndex(chain, ".")
	if dot < 0 {
		return "", chain
	}
	return chain[:dot], chain[dot+1:]
}

// isLiteralBoolTrue reports whether the node text is `true` after unwrapping
// parentheses.
func isLiteralBoolTrue(file *scanner.File, idx uint32) bool {
	idx = flatUnwrapParenExpr(file, idx)
	return strings.TrimSpace(file.FlatNodeText(idx)) == "true"
}

// findKotlinAssignmentValue returns the right-hand expression node of a
// Kotlin assignment, or 0 if none.
func findKotlinAssignmentValue(file *scanner.File, assignment uint32) uint32 {
	if file == nil || file.FlatType(assignment) != "assignment" {
		return 0
	}
	// In tree-sitter-kotlin, `a = b` is an `assignment` with two named
	// children: the assignable target and the value expression. We already
	// extract the target via kotlinAssignmentTargetChain — find the second
	// named child, skipping the assignment operator.
	count := file.FlatNamedChildCount(assignment)
	if count < 2 {
		return 0
	}
	return file.FlatNamedChild(assignment, count-1)
}

// webSettingsBoolToggleSpec configures checkWebSettingsBoolToggle for a
// specific WebSettings boolean property/setter pair (e.g. allowContentAccess /
// setAllowContentAccess). All matching call_expression, method_invocation,
// and assignment node shapes are routed through one detector to keep the
// individual rule files thin.
type webSettingsBoolToggleSpec struct {
	Setter   string
	Property string
	RuleSet  string
	Rule     string
	Severity string
	Message  string
	Conf     float64
}

// checkWebSettingsBoolToggle implements the shared detection used by every
// WebSettings boolean-toggle rule (allowFileAccess, allowContentAccess, ...).
// It dispatches on the current node type and applies the same receiver-proof,
// literal-true, and emit logic each rule needs.
func checkWebSettingsBoolToggle(ctx *api.Context, spec webSettingsBoolToggleSpec) {
	file := ctx.File

	emit := func() {
		line := file.FlatRow(ctx.Idx) + 1
		col := file.FlatCol(ctx.Idx) + 1
		emitWebSettingsBoolFinding(ctx, line, col, spec.RuleSet, spec.Rule, spec.Severity, spec.Message, spec.Conf)
	}

	switch file.FlatType(ctx.Idx) {
	case "call_expression":
		if flatCallExpressionName(file, ctx.Idx) != spec.Setter {
			return
		}
		if !callBoolArgIsTrue(file, ctx.Idx) {
			return
		}
		if !isWebSettingsReceiver(file, kotlinCallReceiverChain(file, ctx.Idx)) {
			return
		}
		emit()

	case "method_invocation":
		if javaAwareCallName(file, ctx.Idx) != spec.Setter {
			return
		}
		if !callBoolArgIsTrue(file, ctx.Idx) {
			return
		}
		if !isWebSettingsReceiver(file, javaMethodReceiverText(file, ctx.Idx)) {
			return
		}
		emit()

	case "assignment":
		head, tail := chainSplitTrailing(kotlinAssignmentTargetChain(file, ctx.Idx))
		if tail != spec.Property {
			return
		}
		if !isWebSettingsReceiver(file, head) {
			return
		}
		valueIdx := findKotlinAssignmentValue(file, ctx.Idx)
		if valueIdx == 0 || !isLiteralBoolTrue(file, valueIdx) {
			return
		}
		emit()
	}
}

// emitWebSettingsBoolFinding helps WebSettings rules emit a uniform finding.
func emitWebSettingsBoolFinding(ctx *api.Context, line, col int, ruleSet, rule, severity, message string, confidence float64) {
	ctx.Emit(scanner.Finding{
		File:       ctx.File.Path,
		Line:       line,
		Col:        col,
		RuleSet:    ruleSet,
		Rule:       rule,
		Severity:   severity,
		Message:    message,
		Confidence: confidence,
	})
}
