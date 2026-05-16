package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// WebViewDebuggingEnabledRule flags WebView remote debugging when it is
// enabled unconditionally in app code.
type WebViewDebuggingEnabledRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *WebViewDebuggingEnabledRule) Confidence() float64 { return 0.8 }

const webViewDebuggingEnabledMessage = "Guard WebView.setWebContentsDebuggingEnabled(true) behind BuildConfig.DEBUG or ApplicationInfo.FLAG_DEBUGGABLE."

func (r *WebViewDebuggingEnabledRule) check(ctx *api.Context) {
	file := ctx.File
	if javaAwareCallName(file, ctx.Idx) != "setWebContentsDebuggingEnabled" {
		return
	}
	if !webViewDebuggingEnabledSingleTrueArg(file, ctx.Idx) {
		return
	}
	if !webViewDebuggingReceiverIsAndroidWebView(file, ctx.Idx) {
		return
	}
	if webViewDebuggingCallIsDebugGuarded(file, ctx.Idx) {
		return
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1, webViewDebuggingEnabledMessage)
	f.Confidence = r.Confidence()
	ctx.Emit(f)
}

func webViewDebuggingEnabledSingleTrueArg(file *scanner.File, call uint32) bool {
	if !callBoolArgIsTrue(file, call) {
		return false
	}
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		return flatPositionalValueArgument(file, args, 1) == 0
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		return ok && file.FlatNamedChildCount(args) == 1
	default:
		return false
	}
}

func webViewDebuggingReceiverIsAndroidWebView(file *scanner.File, call uint32) bool {
	switch file.FlatType(call) {
	case "call_expression":
		return webViewDebuggingReceiverTextIsAndroidWebView(file, kotlinCallReceiverChain(file, call))
	case "method_invocation":
		return webViewDebuggingReceiverTextIsAndroidWebView(file, javaMethodReceiverText(file, call))
	default:
		return false
	}
}

func webViewDebuggingReceiverTextIsAndroidWebView(file *scanner.File, receiver string) bool {
	receiver = strings.TrimSpace(receiver)
	if receiver == "android.webkit.WebView" {
		return true
	}
	return receiver == "WebView" && sourceImportsOrMentions(file, "android.webkit.WebView")
}

func webViewDebuggingCallIsDebugGuarded(file *scanner.File, call uint32) bool {
	for cur, ok := file.FlatParent(call); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "method_declaration", "lambda_literal", "anonymous_function":
			return false
		case "if_expression", "if_statement":
			condition, thenBody := webViewDebuggingIfConditionAndThenBody(file, cur)
			if condition != 0 && thenBody != 0 &&
				webViewDebuggingConditionAllowsDebug(file, condition) &&
				flatNodeContains(file, thenBody, call) {
				return true
			}
		case "when_entry":
			if webViewDebuggingWhenEntryConditionAllowsDebug(file, cur, call) {
				return true
			}
		}
	}
	return false
}

func webViewDebuggingIfConditionAndThenBody(file *scanner.File, idx uint32) (uint32, uint32) {
	if file.FlatType(idx) == "if_expression" {
		return ifConditionAndThenBodyFlat(file, idx)
	}
	var named []uint32
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			named = append(named, child)
		}
	}
	if len(named) < 2 {
		return 0, 0
	}
	return named[0], named[1]
}

func webViewDebuggingConditionAllowsDebug(file *scanner.File, condition uint32) bool {
	text := strings.TrimSpace(file.FlatNodeText(condition))
	return strings.Contains(text, "BuildConfig.DEBUG") ||
		strings.Contains(text, "ApplicationInfo.FLAG_DEBUGGABLE") ||
		strings.Contains(text, "FLAG_DEBUGGABLE")
}

func webViewDebuggingWhenEntryConditionAllowsDebug(file *scanner.File, entry, call uint32) bool {
	callStart := file.FlatStartByte(call)
	for child := file.FlatFirstChild(entry); child != 0; child = file.FlatNextSib(child) {
		if file.FlatStartByte(child) >= callStart {
			break
		}
		if webViewDebuggingConditionAllowsDebug(file, child) {
			return true
		}
	}
	return false
}

func flatNodeContains(file *scanner.File, outer, inner uint32) bool {
	return file != nil && outer != 0 && inner != 0 &&
		file.FlatStartByte(outer) <= file.FlatStartByte(inner) &&
		file.FlatEndByte(inner) <= file.FlatEndByte(outer)
}
