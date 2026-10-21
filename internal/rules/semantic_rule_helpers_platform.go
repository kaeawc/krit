package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

func logCallIsAndroidLog(ctx *api.Context, call uint32) bool {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || target.CalleeName == "" || !logLevelNames[target.CalleeName] {
		return false
	}
	if target.Resolved {
		return semanticTargetOwnerMatches(target.QualifiedName, "android.util.Log")
	}
	receiver := flatReceiverNameFromCall(ctx.File, call)
	switch receiver {
	case "Log":
		// Require an import of `android.util.Log` so Timber.Log,
		// slf4j Logger aliases, kotlin-logging KLogger, and same-file
		// `class Log` lookalikes are not misclassified as
		// android.util.Log. The resolver already shadows imports with
		// same-file declarations of the simple name; the filefacts
		// fallback still rejects when the file imports anything other
		// than android.util.Log under the simple name `Log`.
		if ctx.Resolver != nil {
			return ctx.Resolver.ResolveImport("Log", ctx.File) == "android.util.Log"
		}
		return fileImportsFQN(ctx.File, "android.util.Log") &&
			!fileDeclaresSimpleTypeNameLocal(ctx.File, "Log")
	case "android.util.Log":
		return true
	default:
		return false
	}
}

// fileDeclaresSimpleTypeNameLocal reports whether `file` declares a
// top-level class / interface / object / type alias whose simple name is
// `name`. Used to reject same-file lookalikes (e.g. a local `class Log`)
// when the resolver is unavailable.
func fileDeclaresSimpleTypeNameLocal(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	for _, kind := range []string{"class_declaration", "object_declaration", "type_alias", "interface_declaration"} {
		file.FlatWalkNodes(0, kind, func(decl uint32) {
			if found {
				return
			}
			if semantics.DeclarationName(file, decl) == name {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
}

// hasAndroidLogGuardFlat reports whether `call` is wrapped in a debug-only
// guard recognized by AOSP Android lint: either
//   - `if (Log.isLoggable(...))`, or
//   - `if (BuildConfig.DEBUG)`
//
// The walk stops at the nearest scope boundary (function/class/lambda) so an
// `isLoggable` guard inside a sibling block does not satisfy this call.
func hasAndroidLogGuardFlat(ctx *api.Context, call uint32) bool {
	file := ctx.File
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "if_expression":
			condition, _ := ifConditionAndThenBodyFlat(file, p)
			if condition != 0 && conditionIsBuildConfigDebugFlat(file, condition) {
				return true
			}
			found := false
			file.FlatWalkNodes(p, "call_expression", func(c uint32) {
				if found || c == call || flatCallExpressionName(file, c) != "isLoggable" {
					return
				}
				if semanticResolvedTargetMatches(ctx, c, "android.util.Log.isLoggable") {
					found = true
					return
				}
				if flatReceiverNameFromCall(file, c) != "Log" {
					return
				}
				if ctx.Resolver != nil {
					if ctx.Resolver.ResolveImport("Log", file) == "android.util.Log" {
						found = true
					}
					return
				}
				// Resolver missing: accept the guard only when the
				// file actually imports android.util.Log and does
				// not declare a same-file `Log` lookalike.
				if fileImportsFQN(file, "android.util.Log") &&
					!fileDeclaresSimpleTypeNameLocal(file, "Log") {
					found = true
				}
			})
			if found {
				return true
			}
		case "function_declaration", "method_declaration",
			"class_declaration", "object_declaration",
			"lambda_literal", "anonymous_function",
			"source_file":
			return false
		}
	}
	return false
}

// conditionIsBuildConfigDebugFlat reports whether `cond` is a non-negated
// BuildConfig.DEBUG reference (optionally parenthesized). The release-
// engineering BuildConfigDebugInverted rule handles the negated form
// separately; here we only treat the positive guard as a no-fire signal.
func conditionIsBuildConfigDebugFlat(file *scanner.File, cond uint32) bool {
	if file == nil || cond == 0 {
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(cond))
	for strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
		inner := strings.TrimSpace(text[1 : len(text)-1])
		if inner == text {
			break
		}
		text = inner
	}
	if strings.HasPrefix(text, "!") {
		return false
	}
	return text == "BuildConfig.DEBUG" || strings.HasSuffix(text, ".BuildConfig.DEBUG")
}

func wakeLockAcquireCall(ctx *api.Context, call uint32) bool {
	if flatCallExpressionName(ctx.File, call) != "acquire" {
		return false
	}
	if semanticCallTargetOrReceiverType(ctx, call,
		[]string{"android.os.PowerManager.WakeLock"},
		[]string{"android.os.PowerManager.WakeLock", "WakeLock"},
	) {
		return true
	}
	return receiverDeclaredFromCall(ctx.File, call, "newWakeLock")
}

func wakeLockReleaseCallOnSameReceiver(ctx *api.Context, acquire, release uint32) bool {
	if flatCallExpressionName(ctx.File, release) != "release" {
		return false
	}
	if !semanticCallTargetOrReceiverType(ctx, release,
		[]string{"android.os.PowerManager.WakeLock"},
		[]string{"android.os.PowerManager.WakeLock", "WakeLock"},
	) {
		if !receiverDeclaredFromCall(ctx.File, release, "newWakeLock") {
			return false
		}
	}
	ar := semantics.ReferenceName(ctx.File, semanticsReceiverNode(ctx, acquire))
	rr := semantics.ReferenceName(ctx.File, semanticsReceiverNode(ctx, release))
	return ar != "" && ar == rr
}

func setJavaScriptEnabledCall(ctx *api.Context, call uint32) bool {
	if flatCallExpressionName(ctx.File, call) != "setJavaScriptEnabled" {
		return false
	}
	if semanticCallTargetOrReceiverType(ctx, call,
		[]string{"android.webkit.WebSettings"},
		[]string{"android.webkit.WebSettings", "WebSettings"},
	) {
		return true
	}
	receiver := semanticsReceiverNode(ctx, call)
	return receiver != 0 && navigationReceiverHasTypedRoot(ctx.File, receiver, "WebView") && navigationChainContainsSegment(ctx.File, receiver, "settings")
}

func setJavaScriptEnabledJavaCall(ctx *api.Context, call uint32) bool {
	file := ctx.File
	if javaMethodInvocationName(file, call) != "setJavaScriptEnabled" {
		return false
	}
	args := javaArgumentExpressions(file, call)
	if len(args) != 1 || !isJavaBooleanTrue(file, args[0]) {
		return false
	}
	if fact, ok := javaSemanticCallFact(ctx, call); ok && fact.ReceiverType != "" {
		return javaProfileTypeMatches(ctx, fact.ReceiverType, "android.webkit.WebSettings")
	}
	if !sourceImportsOrMentions(file, "android.webkit.WebSettings") && !sourceImportsOrMentions(file, "android.webkit.WebView") {
		return false
	}
	receiver := javaMethodReceiverText(file, call)
	if strings.Contains(receiver, "getSettings") {
		return true
	}
	name := wrongViewCastCallReceiverName(file, call)
	return name == "settings" || name == "webSettings" || strings.HasSuffix(name, ".settings") || strings.HasSuffix(name, ".webSettings")
}

func webSettingsAssignmentTarget(ctx *api.Context, assignment uint32) bool {
	file := ctx.File
	target, _ := file.FlatFindChild(assignment, "directly_assignable_expression")
	if target == 0 || finalSimpleIdentifier(file, target) != "javaScriptEnabled" {
		return false
	}
	if directlyAssignableWebSettingsTarget(file, assignment, target) {
		return true
	}
	receiver := assignmentReceiverBeforeFinal(file, target)
	if receiver == 0 {
		return false
	}
	if semanticTypeNameMatches(semanticReferenceTypeName(ctx, receiver), "android.webkit.WebSettings", "WebSettings") {
		return true
	}
	if file.FlatType(receiver) == "navigation_expression" {
		return navigationReceiverHasTypedRoot(file, receiver, "WebView") && navigationChainContainsSegment(file, receiver, "settings")
	}
	foundWebSettingsReceiver := false
	file.FlatWalkNodes(target, "navigation_expression", func(nav uint32) {
		if foundWebSettingsReceiver {
			return
		}
		if semanticTypeNameMatches(semanticReferenceTypeName(ctx, nav), "android.webkit.WebSettings", "WebSettings") ||
			(navigationReceiverHasTypedRoot(file, nav, "WebView") && navigationChainContainsSegment(file, nav, "settings")) {
			foundWebSettingsReceiver = true
		}
	})
	if foundWebSettingsReceiver {
		return true
	}
	name := semantics.ReferenceName(file, receiver)
	fn, ok := flatEnclosingFunction(file, assignment)
	return ok && parameterHasTypeFlat(file, fn, name, "WebSettings")
}

func directlyAssignableWebSettingsTarget(file *scanner.File, assignment uint32, target uint32) bool {
	segments := directlyAssignableSegments(file, target)
	if len(segments) < 2 || segments[len(segments)-1] != "javaScriptEnabled" {
		return false
	}
	fn, ok := flatEnclosingFunction(file, assignment)
	if !ok {
		return false
	}
	if len(segments) == 2 {
		return parameterHasTypeFlat(file, fn, segments[0], "WebSettings")
	}
	return navigationSegmentsContain(segments[:len(segments)-1], "settings") && parameterHasTypeFlat(file, fn, segments[0], "WebView")
}

func directlyAssignableSegments(file *scanner.File, target uint32) []string {
	if file.FlatType(target) != "directly_assignable_expression" {
		return nil
	}
	var out []string
	for child := file.FlatFirstChild(target); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			out = append(out, file.FlatNodeText(child))
		case "navigation_suffix":
			if ident, ok := file.FlatFindChild(child, "simple_identifier"); ok {
				out = append(out, file.FlatNodeText(ident))
			}
		case "navigation_expression":
			out = append(out, flatNavigationChainIdentifiers(file, child)...)
		}
	}
	return out
}

func assignmentReceiverBeforeFinal(file *scanner.File, idx uint32) uint32 {
	switch file.FlatType(idx) {
	case "directly_assignable_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				return assignmentReceiverBeforeFinal(file, child)
			}
		}
	case "navigation_expression":
		var receiver uint32
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			if file.FlatType(child) == "navigation_suffix" {
				break
			}
			receiver = child
		}
		return receiver
	}
	return 0
}

func apiGuardedByVersionCheckFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if declarationHasAPIGuardAnnotationFlat(file, p) {
			return true
		}
		switch file.FlatType(p) {
		case "annotation":
			name := annotationFinalName(file, p)
			if name == "RequiresApi" || name == "TargetApi" {
				return true
			}
		case "if_expression", "when_expression":
			found := false
			file.FlatWalkAllNodes(p, func(n uint32) {
				if found || n == idx {
					return
				}
				if file.FlatType(n) == "navigation_expression" {
					segments := flatNavigationChainIdentifiers(file, n)
					if len(segments) >= 3 && segments[0] == "Build" && segments[1] == "VERSION" && segments[2] == "SDK_INT" {
						found = true
					}
				}
			})
			if found {
				return true
			}
		case "source_file":
			return false
		}
	}
	return false
}

func declarationHasAPIGuardAnnotationFlat(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "function_declaration", "class_declaration", "object_declaration", "property_declaration", "variable_declaration":
	default:
		return false
	}
	mods, _ := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(mods, "annotation", func(ann uint32) {
		if found {
			return
		}
		name := annotationFinalName(file, ann)
		if name == "RequiresApi" || name == "TargetApi" {
			found = true
		}
	})
	return found
}

func nodeHasAncestorTypeFlat(file *scanner.File, idx uint32, types ...string) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		pt := file.FlatType(p)
		for _, typ := range types {
			if pt == typ {
				return true
			}
		}
	}
	return false
}

func apiNodeNameFlat(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "call_expression":
		if name := flatCallExpressionName(file, idx); name != "" {
			return name
		}
		return simpleCallExpressionNameFromText(file.FlatNodeText(idx))
	case "simple_identifier", "type_identifier":
		return file.FlatNodeText(idx)
	case "user_type", "navigation_expression":
		if name := flatNavigationExpressionLastIdentifier(file, idx); name != "" {
			return name
		}
		return strings.TrimSpace(file.FlatNodeText(idx))
	}
	return semantics.ReferenceName(file, idx)
}

func newAPINestedAccessHandledByOuterNode(file *scanner.File, idx uint32) bool {
	name := apiNodeNameFlat(file, idx)
	if name == "" {
		return false
	}
	switch file.FlatType(idx) {
	case "simple_identifier", "type_identifier":
		for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
			switch file.FlatType(parent) {
			case "call_expression", "navigation_expression", "user_type":
				if apiNodeNameFlat(file, parent) == name {
					return true
				}
			case "source_file":
				return false
			}
		}
	case "navigation_expression":
		for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
			switch file.FlatType(parent) {
			case "call_expression", "user_type":
				if apiNodeNameFlat(file, parent) == name {
					return true
				}
			case "source_file":
				return false
			}
		}
	case "user_type":
		for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
			switch file.FlatType(parent) {
			case "user_type":
				if apiNodeNameFlat(file, parent) == name {
					return true
				}
			case "call_expression", "navigation_expression", "source_file":
				return false
			}
		}
	}
	return false
}

func inlinedAPIReferenceNameFlat(file *scanner.File, idx uint32) string {
	segments := flatNavigationChainIdentifiers(file, idx)
	if len(segments) > 0 {
		return strings.Join(segments, ".")
	}
	return apiNodeNameFlat(file, idx)
}

func sharedPrefsEditorCall(ctx *api.Context, call uint32) bool {
	name := flatCallExpressionName(ctx.File, call)
	if !isSharedPrefsPutMethod(name) && !isSharedPrefsGetMethod(name) {
		return false
	}
	return semanticCallTargetOrReceiverType(ctx, call,
		[]string{"android.content.SharedPreferences.Editor", "android.content.SharedPreferences"},
		[]string{"android.content.SharedPreferences.Editor", "android.content.SharedPreferences", "Editor", "SharedPreferences"},
	) || sameFileFunctionDeclaredInOwner(ctx.File, call, "Editor", "SharedPreferences")
}

func encryptedStorageReceiver(ctx *api.Context, call uint32) bool {
	receiver := semanticsReceiverNode(ctx, call)
	if receiver == 0 {
		receiver = astReceiverNodeFromCall(ctx.File, call)
		if receiver == 0 {
			return false
		}
	}
	typ, ok := semantics.ExpressionType(ctx, receiver)
	if !ok {
		return receiverChainHasQualifiedRoot(ctx.File, receiver, "EncryptedSharedPreferences", "EncryptedFile")
	}
	return semanticTypeNameMatches(typ.Name, "EncryptedSharedPreferences", "EncryptedFile") ||
		semanticTypeNameMatches(typ.FQN, "androidx.security.crypto.EncryptedSharedPreferences", "androidx.security.crypto.EncryptedFile")
}

func plainFileWriteCall(ctx *api.Context, call uint32) bool {
	name := flatCallExpressionName(ctx.File, call)
	if name != "writeText" && name != "writeBytes" {
		return false
	}
	return semanticCallTargetOrReceiverType(ctx, call,
		[]string{"java.io.File", "kotlin.io.FilesKt"},
		[]string{"java.io.File", "File"},
	) || receiverDeclaredFromCall(ctx.File, call, "File") || receiverContainsCallName(ctx.File, call, "File")
}

func sensitiveFileReceiverLiteral(ctx *api.Context, call uint32) string {
	receiver := semanticsReceiverNode(ctx, call)
	if receiver == 0 {
		receiver = astReceiverNodeFromCall(ctx.File, call)
	}
	if receiver == 0 {
		return ""
	}
	if ctx.File.FlatType(receiver) != "call_expression" || flatCallExpressionName(ctx.File, receiver) != "File" {
		found := uint32(0)
		ctx.File.FlatWalkNodes(receiver, "call_expression", func(candidate uint32) {
			if found == 0 && flatCallExpressionName(ctx.File, candidate) == "File" {
				found = candidate
			}
		})
		receiver = found
		if receiver == 0 {
			return ""
		}
	}
	_, args := flatCallExpressionParts(ctx.File, receiver)
	if args == 0 {
		return ""
	}
	for arg := ctx.File.FlatFirstChild(args); arg != 0; arg = ctx.File.FlatNextSib(arg) {
		if ctx.File.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(ctx.File, arg)
		if expr != 0 && ctx.File.FlatType(expr) == "string_literal" {
			return stringLiteralContent(ctx.File, expr)
		}
	}
	return ""
}
