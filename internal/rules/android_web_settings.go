package rules

import (
	"regexp"
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

// isWebSettingsReceiverAt extends isWebSettingsReceiver with scope-aware
// proofs for the node at idx: a bare identifier declared as WebSettings (or
// initialised from `.settings` / `getSettings()`), `it` inside also/let on a
// settings receiver, and an empty receiver inside apply/run/with on one.
func isWebSettingsReceiverAt(file *scanner.File, idx uint32, chain string) bool {
	if isWebSettingsReceiver(file, chain) {
		return true
	}
	if file == nil ||
		(!sourceImportsOrMentions(file, "android.webkit.WebSettings") &&
			!sourceImportsOrMentions(file, "android.webkit.WebView")) {
		return false
	}
	chain = strings.TrimSpace(chain)
	if chain == "" {
		return implicitWebSettingsScope(file, idx, false)
	}
	if strings.ContainsAny(chain, ".()") {
		return false
	}
	if chain == "it" && implicitWebSettingsScope(file, idx, true) {
		return true
	}
	return webSettingsDeclaredAs(file, chain)
}

// webSettingsDeclaredAs reports whether the file declares name as a WebSettings
// variable, either by type or by an initialiser reading the WebView settings.
func webSettingsDeclaredAs(file *scanner.File, name string) bool {
	quoted := regexp.QuoteMeta(name)
	re := regexp.MustCompile(`(?m)(?:\b` + quoted + `\s*:\s*(?:android\.webkit\.)?WebSettings\b)|(?:\bWebSettings\s+` + quoted + `\b)|(?:\b(?:val|var)\s+` + quoted + `\s*(?::[^=\n]*)?=\s*[^\n]*(?:\.settings|getSettings\(\))\s*$)`)
	return re.Match(file.Content)
}

// implicitWebSettingsScope walks the enclosing scope functions of idx looking
// for one whose receiver is a WebSettings. explicitIt selects also/let (the
// receiver is `it`); otherwise apply/run/with (the receiver is `this`).
func implicitWebSettingsScope(file *scanner.File, idx uint32, explicitIt bool) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "function_declaration", "class_declaration", "source_file":
			return false
		case "call_expression":
			if flatCallTrailingLambda(file, parent) == 0 {
				continue
			}
			name := flatCallExpressionName(file, parent)
			callee := parent
			if name == "" {
				// `with(x) { ... }` parses as a call whose callee is the inner `with(x)` call.
				if inner, found := file.FlatFindChild(parent, "call_expression"); found {
					name, callee = flatCallExpressionName(file, inner), inner
				}
			}
			var receiver string
			switch {
			case !explicitIt && (name == "apply" || name == "run"), explicitIt && (name == "also" || name == "let"):
				receiver = kotlinCallReceiverChain(file, parent)
			case !explicitIt && name == "with":
				_, args := flatCallExpressionParts(file, callee)
				if args != 0 && file.FlatNamedChildCount(args) > 0 {
					receiver = strings.TrimSpace(file.FlatNodeText(file.FlatNamedChild(args, 0)))
				}
			default:
				continue
			}
			// The first enclosing matching scope function decides.
			return receiver != "" && isWebSettingsReceiverAt(file, parent, receiver)
		}
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
		emitWebSettingsBoolFinding(ctx, ctx.Idx, spec.RuleSet, spec.Rule, spec.Severity, spec.Message, spec.Conf)
	}

	switch file.FlatType(ctx.Idx) {
	case "call_expression":
		if flatCallExpressionName(file, ctx.Idx) != spec.Setter {
			return
		}
		if !callBoolArgIsTrue(file, ctx.Idx) {
			return
		}
		if !isWebSettingsReceiverAt(file, ctx.Idx, kotlinCallReceiverChain(file, ctx.Idx)) {
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
		if !isWebSettingsReceiverAt(file, ctx.Idx, javaMethodReceiverText(file, ctx.Idx)) {
			return
		}
		emit()

	case "assignment":
		head, tail := chainSplitTrailing(kotlinAssignmentTargetChain(file, ctx.Idx))
		if tail != spec.Property {
			return
		}
		if !isWebSettingsReceiverAt(file, ctx.Idx, head) {
			return
		}
		valueIdx := findKotlinAssignmentValue(file, ctx.Idx)
		if valueIdx == 0 || !isLiteralBoolTrue(file, valueIdx) {
			return
		}
		emit()
	}
}

// emitWebSettingsBoolFinding emits a finding spanning the whole setter call or assignment node.
func emitWebSettingsBoolFinding(ctx *api.Context, node uint32, ruleSet, rule, severity, message string, confidence float64) {
	ctx.Emit(scanner.Finding{
		File:       ctx.File.Path,
		Line:       ctx.File.FlatRow(node) + 1,
		Col:        ctx.File.FlatCol(node) + 1,
		StartByte:  int(ctx.File.FlatStartByte(node)),
		EndByte:    int(ctx.File.FlatEndByte(node)),
		RuleSet:    ruleSet,
		Rule:       rule,
		Severity:   severity,
		Message:    message,
		Confidence: confidence,
	})
}
