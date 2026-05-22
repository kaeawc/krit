package rules

// Android exported-component security rules:
//   - ExportedContentProviderRule
//   - ExportedReceiverRule
//   - GrantAllUrisRule
//   - UnprotectedDynamicReceiverRule
//
// Helpers in this file are also called by
// BroadcastReceiverExportedFlagMissingRule (defined in android_security.go)
// — Go same-package resolution keeps that working.
//
// Extracted from android_security.go as part of the god-file split.

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ExportedContentProviderRule detects exported content providers without permission.
type ExportedContentProviderRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ExportedContentProviderRule) Confidence() float64 { return api.ConfidenceMedium }

func exportedPermissionEnforcedInClass(file *scanner.File, classIdx uint32) bool {
	body, _ := file.FlatFindChild(classIdx, "class_body")
	if body == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(body, "call_expression", func(call uint32) {
		if found {
			return
		}
		switch flatCallExpressionName(file, call) {
		case "enforceCallingPermission",
			"enforceCallingOrSelfPermission",
			"checkCallingPermission",
			"checkCallingOrSelfPermission",
			"enforcePermission",
			"checkPermission",
			"enforceUriPermission",
			"checkUriPermission":
			found = true
		}
	})
	return found
}

func exportedClassExtendsAndroid(file *scanner.File, classIdx uint32, simpleName, fqn string) bool {
	if !privacyClassDirectlyExtendsFlat(file, classIdx, simpleName) {
		return false
	}
	return missingPermissionHasImport(file, fqn)
}

func (r *ExportedContentProviderRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if !exportedClassExtendsAndroid(file, idx, "ContentProvider", "android.content.ContentProvider") {
		return
	}
	if exportedPermissionEnforcedInClass(file, idx) {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"ContentProvider subclass may be exported without permission. Ensure permissions are enforced.")
}

// ExportedReceiverRule detects exported receivers without permission.
type ExportedReceiverRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ExportedReceiverRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ExportedReceiverRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if !exportedClassExtendsAndroid(file, idx, "BroadcastReceiver", "android.content.BroadcastReceiver") {
		return
	}
	if exportedPermissionEnforcedInClass(file, idx) {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"BroadcastReceiver subclass may be exported without permission. Ensure permissions are enforced.")
}

// GrantAllUrisRule detects overly broad URI permissions.
type GrantAllUrisRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *GrantAllUrisRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *GrantAllUrisRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	name := grantURIPermissionCallName(file, idx)
	if name != "grantUriPermission" && name != "grantUriPermissions" {
		return
	}
	confidence := grantURIPermissionConfidence(ctx, idx)
	if confidence <= 0 {
		return
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Overly broad URI permission grant. Consider restricting to specific URIs.")
	f.Confidence = confidence
	ctx.Emit(f)
}

func grantURIPermissionCallName(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "call_expression":
		return flatCallExpressionName(file, idx)
	case "method_invocation":
		return wrongViewCastCallName(file, idx)
	default:
		return ""
	}
}

func grantURIPermissionConfidence(ctx *api.Context, idx uint32) float64 {
	file := ctx.File
	if file.FlatType(idx) == "method_invocation" {
		return grantURIPermissionJavaConfidence(file, idx)
	}
	navExpr, args := flatCallExpressionParts(file, idx)
	if navExpr != 0 && ctx.Resolver != nil {
		receiver := file.FlatNamedChild(navExpr, 0)
		if receiver != 0 {
			typ := ctx.Resolver.ResolveFlatNode(receiver, file)
			if typ == nil || typ.Kind == typeinfer.TypeUnknown {
				if file.FlatType(receiver) == "simple_identifier" {
					typ = ctx.Resolver.ResolveByNameFlat(file.FlatNodeText(receiver), receiver, file)
				}
			}
			if typ != nil && typ.Kind != typeinfer.TypeUnknown {
				if grantURITypeIsContext(ctx.Resolver, typ) {
					return 1.0
				}
				return 0
			}
		}
	}
	if navExpr != 0 {
		receiver := file.FlatNamedChild(navExpr, 0)
		if receiver != 0 && file.FlatType(receiver) == "simple_identifier" {
			recvName := file.FlatNodeText(receiver)
			if recvName == "this" || recvName == "context" || recvName == "ctx" {
				if missingPermissionHasImport(file, "android.content.Context") {
					return 0.85
				}
			}
		}
	} else if missingPermissionHasImport(file, "android.content.Context") {
		// Unqualified call in a file that imports Context — likely an Activity/Service.
		_ = args
		return 0.85
	}
	return 0.7
}

func grantURIPermissionJavaConfidence(file *scanner.File, idx uint32) float64 {
	receiver := wrongViewCastCallReceiverName(file, idx)
	switch receiver {
	case "context", "ctx", "this":
		if sourceImportsOrMentions(file, "android.content.Context") ||
			sourceImportsOrMentions(file, "android.app.Activity") ||
			sourceImportsOrMentions(file, "android.app.Service") {
			return 0.85
		}
	case "":
		if sourceImportsOrMentions(file, "android.content.Context") ||
			sourceImportsOrMentions(file, "android.app.Activity") ||
			sourceImportsOrMentions(file, "android.app.Service") {
			return 0.85
		}
	default:
		if strings.HasSuffix(receiver, ".Context") || strings.HasSuffix(receiver, ".Activity") || strings.HasSuffix(receiver, ".Service") {
			return 0.85
		}
	}
	return 0
}

func grantURITypeIsContext(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType) bool {
	if typ == nil {
		return false
	}
	seen := make(map[string]bool)
	var visit func(string) bool
	visit = func(name string) bool {
		if name == "" || seen[name] {
			return false
		}
		seen[name] = true
		if name == "Context" || name == "android.content.Context" {
			return true
		}
		if resolver == nil {
			return false
		}
		info := resolver.ClassHierarchy(name)
		if info == nil {
			return false
		}
		if info.Name == "Context" || info.FQN == "android.content.Context" {
			return true
		}
		for _, supertype := range info.Supertypes {
			if visit(supertype) {
				return true
			}
		}
		return false
	}
	return visit(typ.FQN) || visit(typ.Name)
}

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
		receiverText := strings.TrimSpace(file.FlatNodeText(receiver))
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
		receiver := wrongViewCastCallReceiverName(file, call)
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
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return false
		}
		positional := flatValueArgumentTexts(file, args)
		if broadcastReceiverUsesContextCompat(file, call) {
			if len(positional) < 4 {
				return true
			}
			return !broadcastReceiverFlagTextContainsExportedConstant(positional[len(positional)-1])
		}
		if len(positional) < 3 {
			return true
		}
		return !broadcastReceiverFlagTextContainsExportedConstant(positional[2])
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok {
			return false
		}
		positional := flatJavaArgumentTexts(file, args)
		if broadcastReceiverUsesContextCompat(file, call) {
			if len(positional) < 4 {
				return true
			}
			return !broadcastReceiverFlagTextContainsExportedConstant(positional[len(positional)-1])
		}
		if len(positional) < 3 {
			return true
		}
		return !broadcastReceiverFlagTextContainsExportedConstant(positional[2])
	default:
		return false
	}
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
			continue
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
