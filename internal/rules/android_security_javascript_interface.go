package rules

// Security rule: AddJavascriptInterface. Detects
// WebView.addJavascriptInterface() calls vulnerable to remote code
// execution on older Android versions or missing the
// @JavascriptInterface annotation on injected objects targeting
// Android 17+.
//
// Extracted from android_security.go as part of the god-file split.
// All helpers in this file are prefixed addJavascriptInterface* and
// are not referenced by any other rule.

import (
	"strings"

	androidproject "github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// AddJavascriptInterfaceRule detects WebView.addJavascriptInterface() calls.
type AddJavascriptInterfaceRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *AddJavascriptInterfaceRule) check(ctx *api.Context) {
	file := ctx.File
	if file.FlatType(ctx.Idx) != "call_expression" && file.FlatType(ctx.Idx) != "method_invocation" {
		return
	}
	if javaAwareCallName(file, ctx.Idx) != "addJavascriptInterface" {
		return
	}
	confidence, ok := addJavascriptInterfaceCallConfidence(ctx, ctx.Idx)
	if !ok {
		return
	}
	line := file.FlatRow(ctx.Idx) + 1
	col := file.FlatCol(ctx.Idx) + 1
	sdk := addJavascriptInterfaceSDKContextForFile(file)
	if sdk.minSdk < 17 {
		f := r.Finding(file, line, col,
			"`WebView.addJavascriptInterface` should not be called with minSdkVersion < 17 for security reasons: JavaScript can use reflection to manipulate application")
		f.Confidence = confidence
		ctx.Emit(f)
	}
	if bridge := addJavascriptInterfaceBridgeMissingAnnotation(file, ctx.Idx); sdk.targetSdk >= 17 && bridge != "" {
		f := r.Finding(file, line, col,
			"None of the methods in the added interface ("+bridge+") have been annotated with `@android.webkit.JavascriptInterface`; they will not be visible in API 17")
		f.Confidence = confidence
		ctx.Emit(f)
	}
}

func addJavascriptInterfaceCallConfidence(ctx *api.Context, call uint32) (float64, bool) {
	file := ctx.File
	if file.FlatType(call) == "method_invocation" {
		return addJavascriptInterfaceJavaConfidence(file, call)
	}
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return 0, false
	}
	return addJavascriptInterfaceReceiverConfidence(ctx, file.FlatNamedChild(navExpr, 0))
}

func addJavascriptInterfaceJavaConfidence(file *scanner.File, call uint32) (float64, bool) {
	if !sourceImportsOrMentions(file, "android.webkit.WebView") {
		return 0, false
	}
	receiver := javaMethodReceiverText(file, call)
	if receiver == "" {
		return 0, false
	}
	if strings.Contains(receiver, "getSettings") {
		return 0, false
	}
	name := wrongViewCastCallReceiverName(file, call)
	if name == "" {
		name = receiver
	}
	if name == "webView" || name == "wv" || strings.HasSuffix(name, ".webView") || strings.HasSuffix(name, ".wv") {
		return 0.85, true
	}
	if addJavascriptInterfaceJavaDeclaredWebView(file, name) {
		return 1.0, true
	}
	return 0, false
}

type addJavascriptInterfaceSDKContext struct {
	minSdk    int
	targetSdk int
}

// addJavascriptInterfaceSDKContextForFile resolves the file's SDK levels with
// androidproject.ResolveSDKLevels, the helper the FIR pass also sends to
// krit-fir checkers (firchecks.FileFacts.SDKLevels), so both sides agree.
func addJavascriptInterfaceSDKContextForFile(file *scanner.File) addJavascriptInterfaceSDKContext {
	if file == nil {
		return addJavascriptInterfaceSDKContext{}
	}
	return filefacts.FileFact(fileFactsCache(), file, slotAddJSInterfaceSDK, func() addJavascriptInterfaceSDKContext {
		levels := androidproject.ResolveSDKLevels(file.Path)
		return addJavascriptInterfaceSDKContext{minSdk: levels.MinSdk, targetSdk: levels.TargetSdk}
	})
}

// addJavascriptInterfaceBridgeMissingAnnotation returns the name of the injected class when none of its methods carry
// @JavascriptInterface, or "" otherwise.
func addJavascriptInterfaceBridgeMissingAnnotation(file *scanner.File, call uint32) string {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return ""
	}
	arg := flatPositionalValueArgument(file, args, 0)
	if arg == 0 {
		arg = flatNamedValueArgument(file, args, "object")
	}
	if arg == 0 {
		arg = flatNamedValueArgument(file, args, "obj")
	}
	expr := flatValueArgumentExpression(file, arg)
	className := addJavascriptInterfaceConstructedClassName(file, expr)
	if className == "" {
		return ""
	}
	classDecl := addJavascriptInterfaceSameFileClass(file, className)
	if classDecl != 0 && !addJavascriptInterfaceClassHasAnnotatedMethod(file, classDecl) {
		return className
	}
	return ""
}

func addJavascriptInterfaceConstructedClassName(file *scanner.File, expr uint32) string {
	if file == nil || expr == 0 {
		return ""
	}
	expr = flatUnwrapParenExpr(file, expr)
	if file.FlatType(expr) == "call_expression" {
		if name := flatCallExpressionName(file, expr); name != "" {
			return name
		}
	}
	var name string
	file.FlatWalkNodes(expr, "type_identifier", func(idx uint32) {
		if name == "" {
			name = file.FlatNodeText(idx)
		}
	})
	return name
}

func addJavascriptInterfaceSameFileClass(file *scanner.File, name string) uint32 {
	var classDecl uint32
	file.FlatWalkNodes(0, "class_declaration", func(candidate uint32) {
		if classDecl == 0 && extractIdentifierFlat(file, candidate) == name {
			classDecl = candidate
		}
	})
	return classDecl
}

func addJavascriptInterfaceClassHasAnnotatedMethod(file *scanner.File, classDecl uint32) bool {
	found := false
	file.FlatWalkNodes(classDecl, "function_declaration", func(fn uint32) {
		if found {
			return
		}
		owner, ok := flatEnclosingAncestor(file, fn, "class_declaration")
		if ok && owner == classDecl && hasAnnotationNamed(file, fn, "JavascriptInterface") {
			found = true
		}
	})
	file.FlatWalkNodes(classDecl, "method_declaration", func(method uint32) {
		if found {
			return
		}
		owner, ok := flatEnclosingAncestor(file, method, "class_declaration")
		if ok && owner == classDecl && strings.Contains(file.FlatNodeText(method), "@JavascriptInterface") {
			found = true
		}
	})
	return found
}

func addJavascriptInterfaceReceiverConfidence(ctx *api.Context, receiverExpr uint32) (float64, bool) {
	file := ctx.File
	receiver := flatUnwrapParenExpr(file, receiverExpr)
	if ctx.Resolver != nil {
		typ := ctx.Resolver.ResolveFlatNode(receiver, file)
		if (typ == nil || typ.Kind == typeinfer.TypeUnknown) && file.FlatType(receiver) == "simple_identifier" {
			typ = ctx.Resolver.ResolveByNameFlat(file.FlatNodeText(receiver), receiver, file)
		}
		if typ != nil && typ.Kind != typeinfer.TypeUnknown && (typ.Name != "" || typ.FQN != "") {
			if addJavascriptInterfaceTypeIsWebView(ctx.Resolver, typ) {
				return 1.0, true
			}
			return 0, false
		}
	}
	name := addJavascriptInterfaceReceiverName(file, receiver)
	if name == "" {
		return 0, false
	}
	if name == "webView" || name == "wv" {
		return 0.85, true
	}
	return 0, false
}

func addJavascriptInterfaceReceiverName(file *scanner.File, receiver uint32) string {
	switch file.FlatType(receiver) {
	case "simple_identifier":
		return file.FlatNodeText(receiver)
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifier(file, receiver)
	}
	return ""
}

func addJavascriptInterfaceTypeIsWebView(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType) bool {
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
		if name == "WebView" || name == "android.webkit.WebView" {
			return true
		}
		if resolver == nil {
			return false
		}
		info := resolver.ClassHierarchy(name)
		if info == nil {
			return false
		}
		if info.Name == "WebView" || info.FQN == "android.webkit.WebView" {
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

// addJavascriptInterfaceJavaDeclaredWebView reports whether a Java receiver identifier is declared in
// this file (field, local or parameter) with the type WebView.
func addJavascriptInterfaceJavaDeclaredWebView(file *scanner.File, receiver string) bool {
	if receiver == "" || strings.ContainsAny(receiver, ".()") {
		return false
	}
	found := false
	isWebViewType := func(n uint32) bool {
		switch file.FlatType(n) {
		case "type_identifier", "scoped_type_identifier":
			text := file.FlatNodeText(n)
			return text == "WebView" || text == "android.webkit.WebView"
		}
		return false
	}
	file.FlatWalkNodes(0, "variable_declarator", func(decl uint32) {
		if found || file.FlatNamedChildCount(decl) == 0 || file.FlatNodeText(file.FlatNamedChild(decl, 0)) != receiver {
			return
		}
		if parent, ok := file.FlatParent(decl); ok {
			for c := file.FlatFirstChild(parent); c != 0; c = file.FlatNextSib(c) {
				if isWebViewType(c) {
					found = true
				}
			}
		}
	})
	file.FlatWalkNodes(0, "formal_parameter", func(param uint32) {
		if found {
			return
		}
		hasName, hasType := false, false
		for c := file.FlatFirstChild(param); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "identifier" && file.FlatNodeText(c) == receiver {
				hasName = true
			}
			if isWebViewType(c) {
				hasType = true
			}
		}
		found = hasName && hasType
	})
	return found
}
