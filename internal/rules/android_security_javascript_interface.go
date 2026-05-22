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
	"os"
	"path/filepath"
	"strconv"
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
			"addJavascriptInterface called while minSdk is below 17. This exposes injected objects to reflection on older Android versions.")
		f.Confidence = confidence
		ctx.Emit(f)
	}
	if sdk.targetSdk >= 17 && addJavascriptInterfaceBridgeMissingAnnotation(file, ctx.Idx) {
		f := r.Finding(file, line, col,
			"Injected JavaScript interface has no @JavascriptInterface-annotated methods for targetSdk 17 or higher.")
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
	return 0, false
}

type addJavascriptInterfaceSDKContext struct {
	minSdk    int
	targetSdk int
}

func addJavascriptInterfaceSDKContextForFile(file *scanner.File) addJavascriptInterfaceSDKContext {
	if file == nil {
		return addJavascriptInterfaceSDKContext{}
	}
	return filefacts.FileFact(fileFactsCache(), file, slotAddJSInterfaceSDK, func() addJavascriptInterfaceSDKContext {
		sdk := addJavascriptInterfaceSDKContext{}
		for _, dir := range ancestorDirs(filepath.Dir(file.Path)) {
			for _, name := range []string{"build.gradle.kts", "build.gradle"} {
				buildPath := filepath.Join(dir, name)
				data, err := os.ReadFile(buildPath)
				if err != nil {
					continue
				}
				cfg, err := androidproject.ParseBuildGradleContent(string(data))
				if err != nil {
					continue
				}
				if cfg.MinSdkVersion > 0 {
					sdk.minSdk = cfg.MinSdkVersion
				}
				if cfg.TargetSdkVersion > 0 {
					sdk.targetSdk = cfg.TargetSdkVersion
				}
				if sdk.minSdk > 0 || sdk.targetSdk > 0 {
					return sdk
				}
			}
			for _, rel := range []string{"src/main/AndroidManifest.xml", "AndroidManifest.xml"} {
				manifestPath := filepath.Join(dir, rel)
				manifest, err := androidproject.ParseManifest(manifestPath)
				if err != nil {
					continue
				}
				if manifest.UsesSdk.MinSdkVersion != "" {
					sdk.minSdk, _ = strconv.Atoi(manifest.UsesSdk.MinSdkVersion)
				}
				if manifest.UsesSdk.TargetSdkVersion != "" {
					sdk.targetSdk, _ = strconv.Atoi(manifest.UsesSdk.TargetSdkVersion)
				}
				if sdk.minSdk > 0 || sdk.targetSdk > 0 {
					return sdk
				}
			}
		}
		return sdk
	})
}

func ancestorDirs(dir string) []string {
	if dir == "" || dir == "." {
		return nil
	}
	dir = filepath.Clean(dir)
	var dirs []string
	for {
		dirs = append(dirs, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dirs
}

func addJavascriptInterfaceBridgeMissingAnnotation(file *scanner.File, call uint32) bool {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return false
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
		return false
	}
	classDecl := addJavascriptInterfaceSameFileClass(file, className)
	return classDecl != 0 && !addJavascriptInterfaceClassHasAnnotatedMethod(file, classDecl)
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
