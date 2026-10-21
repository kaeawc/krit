package rules

import (
	"path/filepath"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type StartActivityWithUntrustedIntentRule struct {
	FlatDispatchBase
	AndroidRule
}

type BroadcastReceiverExportedFlagMissingRule struct {
	FlatDispatchBase
	AndroidRule
}

type ImplicitPendingIntentRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *StartActivityWithUntrustedIntentRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *BroadcastReceiverExportedFlagMissingRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *ImplicitPendingIntentRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *BroadcastReceiverExportedFlagMissingRule) check(ctx *api.Context) {
	file := ctx.File
	if scanner.IsTestFile(file.Path) || strings.Contains(filepath.ToSlash(file.Path), "/androidTest/") {
		return
	}
	if javaAwareCallName(file, ctx.Idx) != "registerReceiver" {
		return
	}
	if !dynamicReceiverHasContextReceiver(file, ctx.Idx) {
		return
	}
	if !broadcastReceiverExportedFlagMissing(file, ctx.Idx) {
		return
	}
	sdk := addJavascriptInterfaceSDKContextForFile(file)
	confidence := r.Confidence()
	if sdk.targetSdk > 0 && sdk.targetSdk < 34 {
		return
	}
	if sdk.targetSdk == 0 {
		confidence = 0.65
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"Dynamic broadcast receiver registration omits RECEIVER_EXPORTED or RECEIVER_NOT_EXPORTED. Add an explicit exported flag for targetSdk 34+.")
	f.Confidence = confidence
	ctx.Emit(f)
}

func (r *ImplicitPendingIntentRule) check(ctx *api.Context) {
	file := ctx.File
	if !implicitPendingIntentCall(file, ctx.Idx) {
		return
	}
	sdk := addJavascriptInterfaceSDKContextForFile(file)
	confidence := r.Confidence()
	if sdk.targetSdk > 0 && sdk.targetSdk < 31 {
		return
	}
	if sdk.targetSdk == 0 {
		confidence = 0.65
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"PendingIntent flags omit FLAG_IMMUTABLE or FLAG_MUTABLE. Add an explicit mutability flag for Android 12+.")
	f.Confidence = confidence
	ctx.Emit(f)
}

func (r *StartActivityWithUntrustedIntentRule) check(ctx *api.Context) {
	file := ctx.File
	if !startActivityLaunchNames[javaAwareCallName(file, ctx.Idx)] {
		return
	}
	if !sourceImportsOrMentions(file, "android.content.Intent") {
		return
	}
	intentVar := startActivityIntentArgument(file, ctx.Idx)
	if intentVar == "" {
		return
	}
	parseStart, ok := startActivityFindParseURIAssignment(file, ctx.Idx, intentVar)
	if !ok {
		return
	}
	if startActivityHasGuardBetween(file, ctx.Idx, intentVar, parseStart) {
		return
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"Launching an Intent parsed from an untrusted URI without setPackage or component guard can enable intent redirection.")
	f.Confidence = r.Confidence()
	ctx.Emit(f)
}

var startActivityLaunchNames = map[string]bool{
	"startActivity":          true,
	"startActivities":        true,
	"startActivityForResult": true,
}

func startActivityIntentArgument(file *scanner.File, call uint32) string {
	var arg uint32
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		arg = flatValueArgumentExpression(file, flatPositionalValueArgument(file, args, 0))
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok || file.FlatNamedChildCount(args) == 0 {
			return ""
		}
		arg = file.FlatNamedChild(args, 0)
	default:
		return ""
	}
	arg = flatUnwrapParenExpr(file, arg)
	switch file.FlatType(arg) {
	case "simple_identifier", "identifier":
		return strings.TrimSpace(file.FlatNodeText(arg))
	default:
		return ""
	}
}

func startActivityFindParseURIAssignment(file *scanner.File, launch uint32, intentVar string) (uint32, bool) {
	scope, ok := flatEnclosingCallable(file, launch)
	if !ok {
		return 0, false
	}
	launchStart := file.FlatStartByte(launch)
	var parseStart uint32
	for _, nodeType := range []string{"call_expression", "method_invocation"} {
		file.FlatWalkNodes(scope, nodeType, func(call uint32) {
			if parseStart != 0 || call == launch || file.FlatStartByte(call) >= launchStart {
				return
			}
			if !sameEnclosingCallable(file, call, scope) || javaAwareCallName(file, call) != "parseUri" {
				return
			}
			if !startActivityParseURIReceiverIsIntent(file, call) {
				return
			}
			container := startActivityAssignmentContainer(file, call)
			if container == 0 || !startActivityContainerAssignsReceiver(file, container, intentVar) {
				return
			}
			parseStart = file.FlatStartByte(container)
		})
	}
	return parseStart, parseStart != 0
}

func sameEnclosingCallable(file *scanner.File, idx, scope uint32) bool {
	got, ok := flatEnclosingCallable(file, idx)
	return ok && got == scope
}

func startActivityParseURIReceiverIsIntent(file *scanner.File, call uint32) bool {
	receiver := ""
	switch file.FlatType(call) {
	case "call_expression":
		nav, _ := flatCallExpressionParts(file, call)
		if nav == 0 || file.FlatNamedChildCount(nav) == 0 {
			return false
		}
		receiver = file.FlatNodeText(file.FlatNamedChild(nav, 0))
	case "method_invocation":
		receiver = javaMethodReceiverText(file, call)
	}
	receiver = strings.TrimSpace(receiver)
	return receiver == "android.content.Intent" ||
		(receiver == "Intent" && !startActivityFileDeclaresIntent(file) && sourceImportsOrMentions(file, "android.content.Intent"))
}

func startActivityFileDeclaresIntent(file *scanner.File) bool {
	found := false
	for _, nodeType := range []string{"class_declaration", "object_declaration", "type_alias"} {
		file.FlatWalkNodes(0, nodeType, func(node uint32) {
			if found {
				return
			}
			if extractIdentifierFlat(file, node) == "Intent" {
				found = true
			}
		})
	}
	return found
}

func startActivityAssignmentContainer(file *scanner.File, call uint32) uint32 {
	for cur, ok := file.FlatParent(call); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "property_declaration", "variable_declaration", "local_variable_declaration", "assignment":
			return cur
		case "function_declaration", "method_declaration", "lambda_literal", "class_declaration", "source_file":
			return 0
		}
	}
	return 0
}

func startActivityContainerAssignsReceiver(file *scanner.File, container uint32, name string) bool {
	text := file.FlatNodeText(container)
	assign := strings.Index(text, "=")
	if assign < 0 {
		return false
	}
	before := text[:assign]
	for _, token := range strings.FieldsFunc(before, func(r rune) bool {
		return r != '_' && r != '$' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
	}) {
		if token == name {
			return true
		}
	}
	return false
}

func startActivityHasGuardBetween(file *scanner.File, launch uint32, intentVar string, parseStart uint32) bool {
	scope, ok := flatEnclosingCallable(file, launch)
	if !ok {
		return false
	}
	launchStart := file.FlatStartByte(launch)
	guarded := false
	for _, nodeType := range []string{"call_expression", "method_invocation", "assignment"} {
		file.FlatWalkNodes(scope, nodeType, func(node uint32) {
			if guarded || node == launch || !sameEnclosingCallable(file, node, scope) {
				return
			}
			start := file.FlatStartByte(node)
			if start <= parseStart || start >= launchStart {
				return
			}
			text := strings.TrimSpace(file.FlatNodeText(node))
			if strings.Contains(text, intentVar+".setPackage(") ||
				strings.Contains(text, intentVar+".setComponent(") ||
				strings.Contains(text, intentVar+".setClassName(") ||
				strings.Contains(text, intentVar+".component") {
				guarded = true
			}
		})
	}
	return guarded
}

var implicitPendingIntentMethods = map[string]bool{
	"getActivity":   true,
	"getBroadcast":  true,
	"getService":    true,
	"getActivities": true,
}

func implicitPendingIntentCall(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	name := javaAwareCallName(file, idx)
	if !implicitPendingIntentMethods[name] {
		return false
	}
	if !sourceImportsOrMentions(file, "android.app.PendingIntent") {
		return false
	}
	text := file.FlatNodeText(idx)
	compact := strings.Join(strings.Fields(text), "")
	if !strings.Contains(compact, "PendingIntent."+name+"(") {
		return false
	}
	if strings.Contains(text, "PendingIntentCompat") {
		return false
	}
	flagsExpr, ok := implicitPendingIntentFlagsExpr(file, idx)
	if !ok || flagsExpr == 0 {
		return false
	}
	// Only flag when the flags argument is an explicit flag expression that we
	// can see demonstrably lacks a mutability flag. Prefer false-negatives.
	return implicitPendingIntentFlagsAreInsecure(file, flagsExpr)
}

// implicitPendingIntentFlagsExpr returns the AST node for the PendingIntent
// flags argument (the 4th positional argument, or the named `flags` argument).
func implicitPendingIntentFlagsExpr(file *scanner.File, call uint32) (uint32, bool) {
	if file == nil || call == 0 {
		return 0, false
	}
	switch file.FlatType(call) {
	case "call_expression":
		args := flatCallKeyArguments(file, call)
		if args == 0 {
			return 0, false
		}
		if named := flatNamedValueArgument(file, args, "flags"); named != 0 {
			return flatValueArgumentExpression(file, named), true
		}
		var last uint32
		for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
			if file.FlatType(arg) != "value_argument" || flatHasValueArgumentLabel(file, arg) {
				continue
			}
			last = arg
		}
		if last == 0 {
			return 0, false
		}
		return flatValueArgumentExpression(file, last), true
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok || file.FlatNamedChildCount(args) == 0 {
			return 0, false
		}
		return file.FlatNamedChild(args, file.FlatNamedChildCount(args)-1), true
	default:
		return 0, false
	}
}

// implicitPendingIntentFlagsAreInsecure decides, from the flags-argument AST
// subtree, whether the call demonstrably omits a mutability flag. It is
// deliberately conservative: it only reports a problem when it can see an
// explicit flag expression (integer literal or qualified FLAG_* constants,
// optionally combined with bitwise/`or` operators) that contains no
// FLAG_IMMUTABLE / FLAG_MUTABLE reference. If the flags come from a helper
// call, a bare parameter/val, or any expression it cannot fully resolve, it
// returns false (no finding).
func implicitPendingIntentFlagsAreInsecure(file *scanner.File, expr uint32) bool {
	if file == nil || expr == 0 {
		return false
	}
	// A bare identifier (parameter or local val) carries no visible flags.
	switch file.FlatType(expr) {
	case "simple_identifier", "identifier":
		return false
	}
	sawFlagConstant := false
	hasMutability := false
	insecure := true
	file.FlatWalkAllNodes(expr, func(node uint32) {
		switch file.FlatType(node) {
		case "call_expression", "call_suffix", "method_invocation":
			// Flags produced by a helper call — cannot prove insecurity.
			insecure = false
		case "navigation_expression", "navigation_suffix",
			"simple_identifier", "identifier":
			text := file.FlatNodeText(node)
			if strings.Contains(text, "FLAG_IMMUTABLE") || strings.Contains(text, "FLAG_MUTABLE") {
				hasMutability = true
			}
			if strings.Contains(text, "FLAG_") {
				sawFlagConstant = true
			}
		case "integer_literal", "hex_literal", "decimal_literal", "long_literal":
			sawFlagConstant = true
		}
	})
	if !insecure || hasMutability {
		return false
	}
	// Require that we actually saw an explicit flag constant / literal; if the
	// expression is something opaque we did not classify, stay conservative.
	return sawFlagConstant
}
