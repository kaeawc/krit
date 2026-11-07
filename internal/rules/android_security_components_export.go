package rules

// Android exported-component security rules:
//   - UnprotectedDynamicReceiverRule
//
// The manifest checks for exported services, receivers and providers are in
// android_manifest_lint_exported.go.

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnprotectedDynamicReceiverRule detects dynamic broadcast receivers
// registered for public actions without a broadcast permission.
type UnprotectedDynamicReceiverRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *UnprotectedDynamicReceiverRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnprotectedDynamicReceiverRule) check(ctx *api.Context) {
	file := ctx.File
	if javaAwareCallName(file, ctx.Idx) != "registerReceiver" {
		return
	}
	if !dynamicReceiverHasContextReceiver(file, ctx.Idx) {
		return
	}
	if !dynamicReceiverHasMissingPermission(file, ctx.Idx) {
		return
	}
	if !dynamicReceiverFilterMentionsPublicAction(file, ctx.Idx) {
		return
	}
	ctx.EmitAt(file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"Dynamic broadcast receiver registered for a public action without a broadcast permission. Pass a non-null permission or restrict the receiver.")
}

func dynamicReceiverHasContextReceiver(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 {
		return false
	}
	text := file.FlatNodeText(call)
	if strings.Contains(text, "LocalBroadcastManager") {
		return false
	}
	importsContext := sourceImportsOrMentions(file, "android.content.Context") ||
		sourceImportsOrMentions(file, "android.app.Activity") ||
		sourceImportsOrMentions(file, "android.app.Service") ||
		sourceImportsOrMentions(file, "android.content.ContextWrapper")
	importsContextCompat := sourceImportsOrMentions(file, "androidx.core.content.ContextCompat") ||
		sourceImportsOrMentions(file, "android.support.v4.content.ContextCompat")
	switch file.FlatType(call) {
	case "call_expression":
		navExpr, _ := flatCallExpressionParts(file, call)
		if navExpr == 0 {
			return importsContext
		}
		receiver := file.FlatNamedChild(navExpr, 0)
		if receiver == 0 {
			return false
		}
		receiverText := strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(receiver)), "()")
		switch receiverText {
		case "this", "context", "ctx", "activity", "service":
			return importsContext || importsContextCompat
		case "ContextCompat", "androidx.core.content.ContextCompat":
			return importsContextCompat
		default:
			return (strings.HasSuffix(receiverText, "Context") ||
				strings.HasSuffix(receiverText, "ContextWrapper") ||
				strings.HasSuffix(receiverText, "Activity") ||
				strings.HasSuffix(receiverText, "Service")) && importsContext
		}
	case "method_invocation":
		receiver := strings.TrimSuffix(wrongViewCastCallReceiverName(file, call), "()")
		switch receiver {
		case "":
			return importsContext
		case "this", "context", "ctx", "activity", "service":
			return importsContext || importsContextCompat
		case "ContextCompat", "androidx.core.content.ContextCompat":
			return importsContextCompat
		default:
			return (strings.HasSuffix(receiver, "Context") ||
				strings.HasSuffix(receiver, "ContextWrapper") ||
				strings.HasSuffix(receiver, "Activity") ||
				strings.HasSuffix(receiver, "Service")) && importsContext
		}
	default:
		return false
	}
}

func dynamicReceiverHasMissingPermission(file *scanner.File, call uint32) bool {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return false
		}
		positional := flatValueArgumentTexts(file, args)
		switch len(positional) {
		case 2:
			return true
		case 4:
			return strings.TrimSpace(positional[2]) == "null"
		default:
			return false
		}
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok {
			return false
		}
		positional := flatJavaArgumentTexts(file, args)
		switch len(positional) {
		case 2:
			return true
		case 4:
			return strings.TrimSpace(positional[2]) == "null"
		default:
			return false
		}
	default:
		return false
	}
}

func broadcastReceiverExportedFlagMissing(file *scanner.File, call uint32) bool {
	if !broadcastReceiverExportedFlagMissingShape(file, call) {
		return false
	}
	var positional []string
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		positional = flatValueArgumentTexts(file, args)
	case "method_invocation":
		if args, ok := file.FlatFindChild(call, "argument_list"); ok {
			positional = flatJavaArgumentTexts(file, args)
		}
	}
	return !broadcastReceiverIsNull(positional, broadcastReceiverUsesContextCompat(file, call)) &&
		!broadcastReceiverFlagsOpaque(positional, broadcastReceiverUsesContextCompat(file, call))
}

// broadcastReceiverIsNull reports a registerReceiver(null, filter) call, which
// only reads a sticky broadcast and registers nothing.
func broadcastReceiverIsNull(positional []string, compat bool) bool {
	idx := 0
	if compat {
		idx = 1
	}
	return idx < len(positional) && positional[idx] == "null"
}

// broadcastReceiverFlagsOpaque reports a flags argument that is a variable or
// call, whose value this rule cannot see. Only literal ints and the
// instant-apps flag alone are known to omit the exported flag.
func broadcastReceiverFlagsOpaque(positional []string, compat bool) bool {
	flagsIdx := -1
	switch {
	case !compat && len(positional) == 3, !compat && len(positional) == 5, compat && len(positional) == 4, compat && len(positional) == 6:
		flagsIdx = len(positional) - 1
	}
	if flagsIdx < 0 {
		return false
	}
	flags := positional[flagsIdx]
	if broadcastReceiverFlagTextContainsExportedConstant(flags) || strings.Contains(flags, "RECEIVER_VISIBLE_TO_INSTANT_APPS") {
		return false
	}
	for _, r := range flags {
		if (r < '0' || r > '9') && r != 'x' && r != 'X' && r != '_' && r != 'L' && r != 'l' && r != ' ' && !(r >= 'a' && r <= 'f') && !(r >= 'A' && r <= 'F') {
			return true
		}
	}
	return false
}

func broadcastReceiverExportedFlagMissingShape(file *scanner.File, call uint32) bool {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return false
		}
		positional := flatValueArgumentTexts(file, args)
		if broadcastReceiverUsesContextCompat(file, call) {
			if len(positional) < 4 {
				return !broadcastFilterOnlyProtectedActions(file, call, positional, true)
			}
			return !broadcastReceiverAnyArgHasExportedConstant(positional) &&
				!broadcastFilterOnlyProtectedActions(file, call, positional, true)
		}
		if len(positional) < 3 {
			return !broadcastFilterOnlyProtectedActions(file, call, positional, false)
		}
		return !broadcastReceiverAnyArgHasExportedConstant(positional) &&
			!broadcastFilterOnlyProtectedActions(file, call, positional, false)
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok {
			return false
		}
		positional := flatJavaArgumentTexts(file, args)
		if broadcastReceiverUsesContextCompat(file, call) {
			if len(positional) < 4 {
				return !broadcastFilterOnlyProtectedActions(file, call, positional, true)
			}
			return !broadcastReceiverAnyArgHasExportedConstant(positional) &&
				!broadcastFilterOnlyProtectedActions(file, call, positional, true)
		}
		if len(positional) < 3 {
			return !broadcastFilterOnlyProtectedActions(file, call, positional, false)
		}
		return !broadcastReceiverAnyArgHasExportedConstant(positional) &&
			!broadcastFilterOnlyProtectedActions(file, call, positional, false)
	default:
		return false
	}
}

// broadcastReceiverAnyArgHasExportedConstant accepts the flags argument at any position, which covers the 5-arg overload.
func broadcastReceiverAnyArgHasExportedConstant(args []string) bool {
	for _, arg := range args {
		if broadcastReceiverFlagTextContainsExportedConstant(arg) {
			return true
		}
	}
	return false
}

// fileDeclaresRegisterReceiver reports whether the file declares its own registerReceiver function or function-type parameter.
func fileDeclaresRegisterReceiver(file *scanner.File) bool {
	found := false
	file.FlatWalkAllNodes(0, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "function_declaration":
			found = flatFunctionName(file, n) == "registerReceiver"
		case "method_declaration":
			if id, ok := file.FlatFindChild(n, "identifier"); ok {
				found = file.FlatNodeTextEquals(id, "registerReceiver")
			}
		case "parameter", "class_parameter":
			found = extractIdentifierFlat(file, n) == "registerReceiver" && strings.Contains(file.FlatNodeText(n), "->")
		}
	})
	return found
}

func broadcastReceiverUsesContextCompat(file *scanner.File, call uint32) bool {
	text := file.FlatNodeText(call)
	if strings.Contains(text, "ContextCompat.registerReceiver") ||
		strings.Contains(text, "androidx.core.content.ContextCompat.registerReceiver") ||
		strings.Contains(text, "android.support.v4.content.ContextCompat.registerReceiver") {
		return true
	}
	switch wrongViewCastCallReceiverName(file, call) {
	case "ContextCompat", "androidx.core.content.ContextCompat", "android.support.v4.content.ContextCompat":
		return true
	default:
		return false
	}
}

func broadcastReceiverFlagTextContainsExportedConstant(text string) bool {
	return strings.Contains(text, "RECEIVER_EXPORTED") || strings.Contains(text, "RECEIVER_NOT_EXPORTED")
}

func flatValueArgumentTexts(file *scanner.File, args uint32) []string {
	var out []string
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" || flatHasValueArgumentLabel(file, arg) {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		if expr == 0 {
			// The Kotlin grammar leaves a `null` argument as an unnamed token.
			expr = arg
		}
		out = append(out, strings.TrimSpace(file.FlatNodeText(expr)))
	}
	return out
}

func flatJavaArgumentTexts(file *scanner.File, args uint32) []string {
	var out []string
	for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		out = append(out, strings.TrimSpace(file.FlatNodeText(child)))
	}
	return out
}

var dynamicReceiverPublicActionString = regexp.MustCompile(`"com\.example\.[A-Za-z0-9_.-]+"`)

func dynamicReceiverFilterMentionsPublicAction(file *scanner.File, call uint32) bool {
	filterText := dynamicReceiverFilterText(file, call)
	if filterText == "" {
		return false
	}
	publicActions := []string{
		"Intent.ACTION_SCREEN_ON",
		"Intent.ACTION_BATTERY_CHANGED",
		"Intent.ACTION_USER_PRESENT",
		"Intent.ACTION_BOOT_COMPLETED",
		"android.intent.action.SCREEN_ON",
		"android.intent.action.BATTERY_CHANGED",
		"android.intent.action.USER_PRESENT",
		"android.intent.action.BOOT_COMPLETED",
	}
	for _, action := range publicActions {
		if strings.Contains(filterText, action) {
			return true
		}
	}
	return dynamicReceiverPublicActionString.MatchString(filterText)
}

func dynamicReceiverFilterText(file *scanner.File, call uint32) string {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return ""
		}
		filter := flatPositionalValueArgument(file, args, 1)
		expr := flatValueArgumentExpression(file, filter)
		if expr == 0 {
			return ""
		}
		return file.FlatNodeText(expr)
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok || file.FlatNamedChildCount(args) < 2 {
			return ""
		}
		return file.FlatNodeText(file.FlatNamedChild(args, 1))
	default:
		return ""
	}
}
