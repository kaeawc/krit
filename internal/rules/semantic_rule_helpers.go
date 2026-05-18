package rules

import (
	"bytes"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/javafacts"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

func semanticTargetOwnerMatches(target string, owners ...string) bool {
	target = strings.TrimSpace(strings.ReplaceAll(target, "#", "."))
	if cut := strings.Index(target, "("); cut >= 0 {
		target = target[:cut]
	}
	for _, owner := range owners {
		owner = strings.TrimSpace(strings.ReplaceAll(owner, "#", "."))
		if target == owner || strings.HasPrefix(target, owner+".") {
			return true
		}
	}
	return false
}

func semanticResolvedTargetMatches(ctx *api.Context, call uint32, fqns ...string) bool {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || !target.Resolved {
		return false
	}
	got := strings.TrimSpace(strings.ReplaceAll(target.QualifiedName, "#", "."))
	for _, want := range fqns {
		want = strings.TrimSpace(strings.ReplaceAll(want, "#", "."))
		if got == want || strings.HasSuffix(got, "."+want) {
			return true
		}
	}
	return false
}

func semanticCallReceiverTypeMatches(ctx *api.Context, call uint32, types ...string) bool {
	if javaSemanticCallReceiverTypeMatches(ctx, call, types...) {
		return true
	}
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || !target.Receiver.Valid() {
		return false
	}
	typ, ok := semantics.ExpressionType(ctx, target.Receiver.Node)
	if !ok {
		return false
	}
	return semanticTypeNameMatches(typ.Name, types...) || semanticTypeNameMatches(typ.FQN, types...)
}

func semanticTypeNameMatches(got string, wants ...string) bool {
	got = strings.TrimSpace(strings.ReplaceAll(got, "$", "."))
	if got == "" {
		return false
	}
	for _, want := range wants {
		want = strings.TrimSpace(strings.ReplaceAll(want, "$", "."))
		if got == want || strings.HasSuffix(got, "."+want) {
			return true
		}
	}
	return false
}

func semanticCallTargetOrReceiverType(ctx *api.Context, call uint32, owners []string, receiverTypes []string) bool {
	if javaSemanticCallTargetOrReceiverType(ctx, call, owners, receiverTypes) {
		return true
	}
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok {
		return false
	}
	if target.Resolved {
		return semanticTargetOwnerMatches(target.QualifiedName, owners...)
	}
	return semanticCallReceiverTypeMatches(ctx, call, receiverTypes...)
}

func javaSemanticCallFact(ctx *api.Context, call uint32) (javafacts.CallFact, bool) {
	if ctx.File == nil || ctx.JavaSemanticFacts == nil || ctx.File.Language != scanner.LangJava {
		return javafacts.CallFact{}, false
	}
	return ctx.JavaSemanticFacts.CallAt(ctx.File.Path, ctx.File.FlatRow(call)+1, ctx.File.FlatCol(call)+1)
}

func javaSemanticCallReceiverTypeMatches(ctx *api.Context, call uint32, types ...string) bool {
	fact, ok := javaSemanticCallFact(ctx, call)
	return ok && javaProfileTypeMatches(ctx, fact.ReceiverType, types...)
}

func javaSemanticCallTargetOrReceiverType(ctx *api.Context, call uint32, owners []string, receiverTypes []string) bool {
	fact, ok := javaSemanticCallFact(ctx, call)
	if !ok {
		return false
	}
	return javaProfileTypeMatches(ctx, fact.MethodOwner, owners...) ||
		javaProfileTypeMatches(ctx, fact.ReceiverType, receiverTypes...)
}

func javaProfileTypeMatches(ctx *api.Context, got string, wants ...string) bool {
	profile := librarymodel.EnsureFacts(ctx.LibraryFacts).Java
	for _, want := range wants {
		if profile.IsSubtypeCandidate(want, got) || semanticTypeNameMatches(got, want) {
			return true
		}
	}
	return false
}

func javaProfileMethodReturn(ctx *api.Context, owner, receiver, method string, arity int) string {
	profile := librarymodel.EnsureFacts(ctx.LibraryFacts).Java
	if ret := profile.MethodReturn(owner, method, arity); ret != "" {
		return ret
	}
	return profile.MethodReturn(receiver, method, arity)
}

func obsoleteComposeModifierCall(ctx *api.Context, call uint32) (name string, replacement string, ok bool) {
	file := ctx.File
	name = flatCallExpressionName(file, call)
	replacements := map[string]string{
		"preferredWidth":  "width",
		"preferredHeight": "height",
		"preferredSize":   "size",
	}
	replacement, ok = replacements[name]
	if !ok {
		return "", "", false
	}
	target, targetOK := semantics.ResolveCallTarget(ctx, call)
	if targetOK && target.Resolved {
		if semanticTargetOwnerMatches(target.QualifiedName, "androidx.compose.ui.Modifier", "androidx.compose.foundation.layout") {
			return name, replacement, true
		}
		return "", "", false
	}
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 {
		return "", "", false
	}
	navSegments := flatNavigationChainIdentifiers(file, nav)
	if len(navSegments) >= 2 && navSegments[0] == "Modifier" && navSegments[len(navSegments)-1] == name {
		return name, replacement, true
	}
	receiver := file.FlatNamedChild(nav, 0)
	if receiver != 0 {
		segments := flatNavigationChainIdentifiers(file, receiver)
		if len(segments) > 0 && segments[0] == "Modifier" {
			return name, replacement, true
		}
		if semanticTypeNameMatches(semanticReferenceTypeName(ctx, receiver), "androidx.compose.ui.Modifier", "Modifier") {
			return name, replacement, true
		}
	}
	return "", "", false
}

func semanticReferenceTypeName(ctx *api.Context, node uint32) string {
	typ, ok := semantics.ExpressionType(ctx, node)
	if !ok {
		return ""
	}
	if typ.FQN != "" {
		return typ.FQN
	}
	return typ.Name
}

func classExtendsAnyFlat(file *scanner.File, class uint32, names ...string) bool {
	for child := file.FlatFirstChild(class); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "delegation_specifier":
			typeName := viewConstructorSupertypeNameFlat(file, child)
			if semanticTypeNameMatches(typeName, names...) {
				return true
			}
		case "superclass", "super_interfaces":
			if javaTypeContainerMatchesAny(file, child, names...) {
				return true
			}
		}
	}
	return false
}

func javaTypeContainerMatchesAny(file *scanner.File, idx uint32, names ...string) bool {
	if file == nil || idx == 0 {
		return false
	}
	if matchTypeBytes(file.FlatNodeBytes(idx), names) {
		return true
	}
	found := false
	file.FlatWalkAllNodes(idx, func(child uint32) {
		if found {
			return
		}
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "scoped_identifier", "generic_type":
			if matchTypeBytes(file.FlatNodeBytes(child), names) {
				found = true
			}
		}
	})
	return found
}

// matchTypeBytes is the byte-level equivalent of the
// "semanticTypeNameMatches(s, name) || strings.Contains(s, name)"
// fallback chain used by the Java supertype walker. The fast path is a
// pure bytes.Contains; the $-form normalization (only meaningful for
// nested-class JNI-style names) is paid for only when an unambiguous
// '$' is present, which keeps the common case allocation-free.
func matchTypeBytes(text []byte, names []string) bool {
	text = bytes.TrimSpace(text)
	if len(text) == 0 {
		return false
	}
	for _, name := range names {
		if bytes.Contains(text, []byte(name)) {
			return true
		}
	}
	// $→. normalization can rescue a match only when either side carries
	// '$'. Today no caller passes a $-form name, but guarding both keeps
	// the fast path strictly equivalent to the prior implementation.
	hasDollar := bytes.IndexByte(text, '$') >= 0
	if !hasDollar {
		for _, name := range names {
			if strings.IndexByte(name, '$') >= 0 {
				hasDollar = true
				break
			}
		}
	}
	if !hasDollar {
		return false
	}
	s := string(text)
	for _, name := range names {
		if semanticTypeNameMatches(s, name) {
			return true
		}
	}
	return false
}

func classHasMemberFunctionFlat(file *scanner.File, class uint32, name string) bool {
	body, _ := file.FlatFindChild(class, "class_body")
	if body == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(body, "function_declaration", func(fn uint32) {
		if found {
			return
		}
		if extractIdentifierFlat(file, fn) == name {
			found = true
		}
	})
	file.FlatWalkNodes(body, "method_declaration", func(fn uint32) {
		if found {
			return
		}
		if extractIdentifierFlat(file, fn) == name {
			found = true
		}
	})
	return found
}

func classHasNestedViewHolderFlat(file *scanner.File, class uint32) bool {
	body, _ := file.FlatFindChild(class, "class_body")
	if body == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(body, "class_declaration", func(child uint32) {
		if found || child == class {
			return
		}
		name := extractIdentifierFlat(file, child)
		if strings.Contains(name, "ViewHolder") || classExtendsAnyFlat(file, child, "RecyclerView.ViewHolder", "ViewHolder") {
			found = true
		}
	})
	return found
}

// recyclerAdapterSupertypeNames are the supertype names the RecyclerAdapter
// rules treat as Adapter-implementing. Shared between the local AST check and
// the resolver-fallback hierarchy walk to keep both paths in sync.
var recyclerAdapterSupertypeNames = []string{
	"androidx.recyclerview.widget.RecyclerView.Adapter",
	"android.support.v7.widget.RecyclerView.Adapter",
	"android.widget.BaseAdapter",
	"RecyclerView.Adapter",
	"BaseAdapter",
	"Adapter",
}

func isRecyclerAdapterClassFlat(ctx *api.Context, class uint32) bool {
	file := ctx.File
	if classExtendsAnyFlat(file, class, recyclerAdapterSupertypeNames...) {
		return true
	}
	if ctx.Resolver == nil || !classHasAdapterLikeSupertypeFlat(file, class) {
		return false
	}
	info := ctx.Resolver.ClassHierarchy(extractIdentifierFlat(file, class))
	if info == nil {
		return false
	}
	for _, st := range info.Supertypes {
		if semanticTypeNameMatches(st, recyclerAdapterSupertypeNames...) {
			return true
		}
	}
	return false
}

// classHasAdapterLikeSupertypeFlat returns true when the class has at least
// one direct supertype whose type identifier ends in "Adapter" or contains
// "RecyclerView" — a cheap signal that consulting the type oracle might
// produce a real Adapter match. The walk is restricted to type-identifier
// shapes so generic args, lambda bodies, and value arguments don't trigger.
func classHasAdapterLikeSupertypeFlat(file *scanner.File, class uint32) bool {
	if file == nil || class == 0 {
		return false
	}
	adapter := []byte("Adapter")
	recyclerView := []byte("RecyclerView")
	found := false
	for child := file.FlatFirstChild(class); child != 0 && !found; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "delegation_specifier", "superclass", "super_interfaces":
		default:
			continue
		}
		file.FlatWalkAllNodes(child, func(n uint32) {
			if found {
				return
			}
			switch file.FlatType(n) {
			case "type_identifier", "scoped_type_identifier", "scoped_identifier", "generic_type":
				text := file.FlatNodeBytes(n)
				if bytes.HasSuffix(text, adapter) || bytes.Contains(text, recyclerView) {
					found = true
				}
			}
		})
	}
	return found
}

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

func semanticsReceiverNode(ctx *api.Context, call uint32) uint32 {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || !target.Receiver.Valid() {
		return 0
	}
	return target.Receiver.Node
}

func astReceiverNodeFromCall(file *scanner.File, call uint32) uint32 {
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 {
		return 0
	}
	var receiver uint32
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
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

func receiverContainsCallName(file *scanner.File, call uint32, names ...string) bool {
	receiver := semanticsReceiverNode(&api.Context{File: file}, call)
	if receiver == 0 {
		receiver = astReceiverNodeFromCall(file, call)
	}
	if receiver == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(receiver, "call_expression", func(candidate uint32) {
		if found {
			return
		}
		callee := flatCallExpressionName(file, candidate)
		for _, name := range names {
			if callee == name {
				found = true
				return
			}
		}
	})
	return found
}

func callReceiverConstructedOrTyped(ctx *api.Context, call uint32, constructorName string, receiverTypes ...string) bool {
	if semanticCallTargetOrReceiverType(ctx, call, receiverTypes, receiverTypes) {
		return true
	}
	return receiverDeclaredFromCall(ctx.File, call, constructorName) || receiverContainsCallName(ctx.File, call, constructorName)
}

func callReceiverParameterHasType(ctx *api.Context, call uint32, typeNames ...string) bool {
	if ctx.File == nil || call == 0 {
		return false
	}
	receiver := semanticsReceiverNode(ctx, call)
	if receiver == 0 {
		receiver = astReceiverNodeFromCall(ctx.File, call)
	}
	name := semantics.ReferenceName(ctx.File, receiver)
	if name == "" {
		return false
	}
	fn, ok := flatEnclosingFunction(ctx.File, call)
	if !ok {
		return false
	}
	for _, typeName := range typeNames {
		if parameterHasTypeFlat(ctx.File, fn, name, typeName) {
			return true
		}
	}
	return false
}

func whileConditionHasCallName(file *scanner.File, stmt uint32, names ...string) bool {
	if file == nil || stmt == 0 || file.FlatType(stmt) != "while_statement" {
		return false
	}
	for child := file.FlatFirstChild(stmt); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "control_structure_body" || file.FlatType(child) == "statements" {
			return false
		}
		if subtreeHasCallName(file, child, names...) {
			return true
		}
	}
	return false
}

func subtreeHasCallName(file *scanner.File, root uint32, names ...string) bool {
	if file == nil || root == 0 || len(names) == 0 {
		return false
	}
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	found := false
	file.FlatWalkNodes(root, "call_expression", func(call uint32) {
		if found {
			return
		}
		if allowed[flatCallNameAny(file, call)] {
			found = true
		}
	})
	file.FlatWalkNodes(root, "method_invocation", func(call uint32) {
		if found {
			return
		}
		if allowed[databaseCallName(file, call)] {
			found = true
		}
	})
	return found
}

func subtreeHasReferenceName(file *scanner.File, root uint32, names ...string) bool {
	if file == nil || root == 0 || len(names) == 0 {
		return false
	}
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	found := false
	file.FlatWalkAllNodes(root, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "simple_identifier", "type_identifier", "identifier", "scoped_type_identifier":
			if allowed[file.FlatNodeText(n)] {
				found = true
			}
		}
	})
	return found
}

func constructorParameterTypeFlags(file *scanner.File, ctor uint32, typeNames ...string) map[string]bool {
	flags := make(map[string]bool, len(typeNames))
	if file == nil || ctor == 0 {
		return flags
	}
	allowed := make(map[string]bool, len(typeNames))
	for _, name := range typeNames {
		allowed[name] = true
	}
	for _, paramType := range []string{"parameter", "class_parameter"} {
		file.FlatWalkNodes(ctor, paramType, func(param uint32) {
			if nearestConstructorAncestorFlat(file, param) != ctor {
				return
			}
			file.FlatWalkNodes(param, "user_type", func(typ uint32) {
				name := apiNodeNameFlat(file, typ)
				for want := range allowed {
					if semanticTypeNameMatches(name, want) {
						flags[want] = true
					}
				}
			})
		})
	}
	return flags
}

func callHasBooleanArgument(file *scanner.File, call uint32, want bool) bool {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(args, "boolean_literal", func(lit uint32) {
		if found {
			return
		}
		found = (file.FlatNodeText(lit) == "true") == want
	})
	return found
}

func callArgHasBoolean(file *scanner.File, arg uint32, want bool) bool {
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(expr, "boolean_literal", func(lit uint32) {
		if found {
			return
		}
		found = (file.FlatNodeText(lit) == "true") == want
	})
	return found
}

func callHasReferenceArgument(file *scanner.File, call uint32, names ...string) bool {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) == "value_argument" && callArgHasReference(file, arg, names...) {
			return true
		}
	}
	return false
}

func callArgHasReference(file *scanner.File, arg uint32, names ...string) bool {
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return false
	}
	return subtreeHasReferenceName(file, expr, names...)
}

func postfixExpressionHasBangBang(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "postfix_expression" {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) && file.FlatNodeTextEquals(child, "!!") {
			return true
		}
	}
	return false
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
	if fact, ok := javaSemanticCallFact(ctx, call); ok {
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

func mainDispatcherReferenceFlat(ctx *api.Context, idx uint32) bool {
	file := ctx.File
	if file.FlatType(idx) != "navigation_expression" {
		return false
	}
	segments := flatNavigationChainIdentifiers(file, idx)
	if len(segments) != 2 || segments[0] != "Dispatchers" || segments[1] != "Main" {
		return false
	}
	if ctx.Resolver != nil {
		if typ, ok := semantics.ExpressionType(ctx, idx); ok && semanticTypeNameMatches(typ.FQN, "kotlinx.coroutines.MainCoroutineDispatcher", "MainCoroutineDispatcher") {
			return true
		}
		if fqn := ctx.Resolver.ResolveImport("Dispatchers", file); fqn != "" {
			return fqn == "kotlinx.coroutines.Dispatchers"
		}
		return false
	}
	return false
}

func receiverDeclaredFromCall(file *scanner.File, call uint32, initializerCallee string) bool {
	receiver := semanticsReceiverNode(&api.Context{File: file}, call)
	name := semantics.ReferenceName(file, receiver)
	if name == "" {
		return false
	}
	fn, ok := flatEnclosingFunction(file, call)
	if !ok {
		return false
	}
	found := false
	file.FlatWalkAllNodes(fn, func(n uint32) {
		if found || file.FlatStartByte(n) >= file.FlatStartByte(call) {
			return
		}
		if file.FlatType(n) != "property_declaration" && file.FlatType(n) != "variable_declaration" {
			return
		}
		if extractIdentifierFlat(file, n) == name && propertyInitializerCallCalleeName(file, n) == initializerCallee {
			found = true
		}
	})
	return found
}

func navigationReceiverHasTypedRoot(file *scanner.File, nav uint32, typeName string) bool {
	segments := flatNavigationChainIdentifiers(file, nav)
	if len(segments) == 0 {
		return false
	}
	root := segments[0]
	fn, ok := flatEnclosingFunction(file, nav)
	if !ok {
		return false
	}
	return parameterHasTypeFlat(file, fn, root, typeName)
}

func parameterHasTypeFlat(file *scanner.File, owner uint32, name string, typeName string) bool {
	if name == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(owner, "parameter", func(param uint32) {
		if found || extractIdentifierFlat(file, param) != name {
			return
		}
		file.FlatWalkNodes(param, "user_type", func(typ uint32) {
			if found {
				return
			}
			if semanticTypeNameMatches(apiNodeNameFlat(file, typ), typeName) {
				found = true
			}
		})
	})
	return found
}

func navigationChainContainsSegment(file *scanner.File, nav uint32, name string) bool {
	return navigationSegmentsContain(flatNavigationChainIdentifiers(file, nav), name)
}

func navigationSegmentsContain(segments []string, name string) bool {
	for _, segment := range segments {
		if segment == name {
			return true
		}
	}
	return false
}

func receiverChainHasQualifiedRoot(file *scanner.File, receiver uint32, roots ...string) bool {
	segments := flatNavigationChainIdentifiers(file, receiver)
	if len(segments) == 0 {
		if file.FlatType(receiver) == "call_expression" {
			nav, _ := flatCallExpressionParts(file, receiver)
			segments = flatNavigationChainIdentifiers(file, nav)
		}
	}
	if len(segments) > 0 {
		for _, root := range roots {
			if segments[0] == root {
				return true
			}
		}
	}
	found := false
	file.FlatWalkAllNodes(receiver, func(n uint32) {
		if found {
			return
		}
		if file.FlatType(n) != "navigation_expression" && file.FlatType(n) != "simple_identifier" {
			return
		}
		name := semantics.ReferenceName(file, n)
		for _, root := range roots {
			if name == root {
				found = true
				return
			}
		}
	})
	return found
}

func sameFileFunctionDeclaredInOwner(file *scanner.File, call uint32, owners ...string) bool {
	name := flatCallExpressionName(file, call)
	if name == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if found || extractIdentifierFlat(file, fn) != name {
			return
		}
		for p, ok := file.FlatParent(fn); ok; p, ok = file.FlatParent(p) {
			if file.FlatType(p) != "class_declaration" && file.FlatType(p) != "object_declaration" {
				continue
			}
			if semanticTypeNameMatches(extractIdentifierFlat(file, p), owners...) {
				found = true
			}
			return
		}
	})
	return found
}

func importShortNameFlat(file *scanner.File, idx uint32) string {
	text := string(file.FlatNodeBytes(idx))
	if lineEnd := strings.IndexByte(text, '\n'); lineEnd >= 0 {
		text = text[:lineEnd]
	}
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "import ") {
		return ""
	}
	imp := strings.TrimSpace(strings.TrimPrefix(text, "import "))
	aliasAt, lastDot := scanImportTail(imp)
	if aliasAt >= 0 {
		return strings.Trim(strings.TrimSpace(imp[aliasAt+4:]), "`")
	}
	last := imp[lastDot+1:]
	if last == "" || last == "*" {
		return ""
	}
	return strings.Trim(last, "`")
}

// scanImportTail walks the import path once, returning the byte offset of an
// " as " alias keyword (or -1) and the byte offset of the last '.' segment
// separator (or -1). Backtick-quoted spans are opaque, so a space or '.'
// inside `foo bar` or `a.b` does not split the path.
func scanImportTail(imp string) (aliasAt, lastDot int) {
	aliasAt, lastDot = -1, -1
	inBacktick := false
	for i := 0; i < len(imp); i++ {
		switch imp[i] {
		case '`':
			inBacktick = !inBacktick
		case ' ':
			if !inBacktick && aliasAt < 0 && strings.HasPrefix(imp[i:], " as ") {
				aliasAt = i
			}
		case '.':
			if !inBacktick {
				lastDot = i
			}
		}
	}
	return aliasAt, lastDot
}

var kotlinImplicitOperatorImportNames = map[string]struct{}{
	"unaryPlus":       {},
	"unaryMinus":      {},
	"not":             {},
	"inc":             {},
	"dec":             {},
	"plus":            {},
	"minus":           {},
	"times":           {},
	"div":             {},
	"mod":             {},
	"rangeTo":         {},
	"rangeUntil":      {},
	"contains":        {},
	"get":             {},
	"set":             {},
	"invoke":          {},
	"plusAssign":      {},
	"minusAssign":     {},
	"timesAssign":     {},
	"divAssign":       {},
	"modAssign":       {},
	"equals":          {},
	"compareTo":       {},
	"iterator":        {},
	"getValue":        {},
	"setValue":        {},
	"provideDelegate": {},
}

func unusedImportImplicitlyUsedByKotlin(shortName string) bool {
	if _, ok := kotlinImplicitOperatorImportNames[shortName]; ok {
		return true
	}
	if !strings.HasPrefix(shortName, "component") || len(shortName) == len("component") {
		return false
	}
	for _, r := range shortName[len("component"):] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func fileHasReferenceNameOutsideNode(file *scanner.File, name string, exclude uint32) bool {
	if file == nil || name == "" {
		return false
	}
	summary := fileReferenceSummaryForUnusedChecks(file)
	excludeStart := file.FlatStartByte(exclude)
	excludeEnd := file.FlatEndByte(exclude)
	for _, ref := range summary.ByName[name] {
		if ref.Start >= excludeStart && ref.End <= excludeEnd {
			continue
		}
		return true
	}
	return contentHasIdentifierOutsideRange(file.Content, name, excludeStart, excludeEnd)
}

func contentHasIdentifierOutsideRange(content []byte, name string, excludeStart, excludeEnd uint32) bool {
	if len(content) == 0 || name == "" {
		return false
	}
	start, end := int(excludeStart), int(excludeEnd)
	if start < 0 || end < start || end > len(content) {
		return false
	}
	return codeTextHasIdentifier(string(content[:start]), name) ||
		codeTextHasIdentifier(string(content[end:]), name)
}

func codeTextHasIdentifier(text, name string) bool {
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], "//") {
			if nl := strings.IndexByte(text[i:], '\n'); nl >= 0 {
				i += nl + 1
			} else {
				return false
			}
			continue
		}
		if strings.HasPrefix(text[i:], "/*") {
			if closeIdx := strings.Index(text[i+2:], "*/"); closeIdx >= 0 {
				i += closeIdx + 4
			} else {
				return false
			}
			continue
		}
		if strings.HasPrefix(text[i:], `"""`) {
			if closeIdx := strings.Index(text[i+3:], `"""`); closeIdx >= 0 {
				i += closeIdx + 6
			} else {
				return false
			}
			continue
		}
		if text[i] == '"' {
			i++
			for i < len(text) {
				if text[i] == '\\' {
					i += 2
					continue
				}
				if text[i] == '"' {
					i++
					break
				}
				i++
			}
			continue
		}
		if text[i] == '\'' {
			i++
			for i < len(text) {
				if text[i] == '\\' {
					i += 2
					continue
				}
				if text[i] == '\'' {
					i++
					break
				}
				i++
			}
			continue
		}
		if hasIdentifierAt(text, name, i) {
			return true
		}
		i++
	}
	return false
}

func hasIdentifierAt(text, name string, i int) bool {
	if i < 0 || i+len(name) > len(text) || text[i:i+len(name)] != name {
		return false
	}
	if i > 0 && isIdentifierByte(text[i-1]) {
		return false
	}
	if i+len(name) < len(text) && isIdentifierByte(text[i+len(name)]) {
		return false
	}
	return true
}

func isIdentifierByte(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '_'
}

// fileReferenceSummaryForUnusedChecks returns a per-file index of
// identifier references for unused-symbol analysis. Backed by the
// shared filefacts cache, which excludes references inside import_header
// and package_header nodes.
func fileReferenceSummaryForUnusedChecks(file *scanner.File) *filefacts.ReferenceFacts {
	return fileFactsCache().References(file, func(file *scanner.File, idx uint32) string {
		return unusedCheckReferenceName(file, idx)
	})
}

func unusedCheckReferenceName(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "call_expression":
		return flatCallExpressionName(file, idx)
	case "simple_identifier", "type_identifier", "navigation_expression", "user_type":
		return semantics.ReferenceName(file, idx)
	case "interpolated_identifier", "line_str_ref", "multi_line_str_ref":
		text := strings.TrimSpace(file.FlatNodeText(idx))
		text = strings.TrimPrefix(text, "$")
		text = strings.TrimPrefix(text, "{")
		text = strings.TrimSuffix(text, "}")
		return strings.Trim(text, "`")
	default:
		return ""
	}
}

func simpleCallExpressionNameFromText(text string) string {
	paren := strings.Index(text, "(")
	if paren < 0 {
		return ""
	}
	prefix := strings.TrimSpace(text[:paren])
	prefix = strings.TrimLeft(prefix, "!. \t\r\n")
	if dot := strings.LastIndex(prefix, "."); dot >= 0 {
		prefix = prefix[dot+1:]
	}
	prefix = strings.Trim(prefix, "`")
	if prefix == "" {
		return ""
	}
	for _, r := range prefix {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' {
			continue
		}
		return ""
	}
	return prefix
}
