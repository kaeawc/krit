package rules

// Android Lint rules for Security, Performance, Accessibility, I18N, and RTL categories.
// Ported from AOSP Android Lint.
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	androidproject "github.com/kaeawc/krit/internal/android"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// Additional category constants not in android.go
const (
	ALCRTL AndroidLintCategory = "rtl"
)

// =============================================================================
// Security Rules
// =============================================================================

// AddJavascriptInterfaceRule detects WebView.addJavascriptInterface() calls.
type AddJavascriptInterfaceRule struct {
	FlatDispatchBase
	AndroidRule
}

var addJavascriptInterfaceSDKCache sync.Map // map[string]addJavascriptInterfaceSDKContext

func (r *AddJavascriptInterfaceRule) check(ctx *v2.Context) {
	if ctx == nil || ctx.File == nil || ctx.Idx == 0 {
		return
	}
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
	sdk := addJavascriptInterfaceSDKContextForFile(file.Path)
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

func addJavascriptInterfaceCallConfidence(ctx *v2.Context, call uint32) (float64, bool) {
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

func addJavascriptInterfaceSDKContextForFile(path string) addJavascriptInterfaceSDKContext {
	if cached, ok := addJavascriptInterfaceSDKCache.Load(path); ok {
		return cached.(addJavascriptInterfaceSDKContext)
	}
	sdk := addJavascriptInterfaceSDKContext{}
	for _, dir := range ancestorDirs(filepath.Dir(path)) {
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
				addJavascriptInterfaceSDKCache.Store(path, sdk)
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
				addJavascriptInterfaceSDKCache.Store(path, sdk)
				return sdk
			}
		}
	}
	addJavascriptInterfaceSDKCache.Store(path, sdk)
	return sdk
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

func addJavascriptInterfaceReceiverConfidence(ctx *v2.Context, receiverExpr uint32) (float64, bool) {
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

// GetInstanceRule detects Cipher.getInstance with insecure algorithms (ECB, DES).
type GetInstanceRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. AST-based
// detection resolves the call shape structurally (call_expression →
// navigation_expression(Cipher.getInstance) → string_literal arg) and
// confirms the receiver is javax.crypto.Cipher via import presence or
// the absence of a same-file user-defined Cipher class. Algorithm
// inspection uses the literal's parsed content, not regex slicing.
func (r *GetInstanceRule) Confidence() float64 { return 0.85 }

var getInstanceInsecureAlgoTokens = []string{"ECB", "DES", "RC2", "RC4"}

func (r *GetInstanceRule) check(ctx *v2.Context) {
	file, idx := ctx.File, ctx.Idx
	navExpr, args := flatCallExpressionParts(file, idx)
	if navExpr == 0 || args == 0 {
		return
	}
	if flatNavigationExpressionLastIdentifier(file, navExpr) != "getInstance" {
		return
	}
	if !getInstanceReceiverIsJavaxCipher(file, navExpr) {
		return
	}
	algo, ok := getInstanceFirstStringArg(file, args)
	if !ok {
		return
	}
	upper := strings.ToUpper(algo)
	hit := false
	for _, tok := range getInstanceInsecureAlgoTokens {
		if strings.Contains(upper, tok) {
			hit = true
			break
		}
	}
	if !hit {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Cipher.getInstance uses insecure algorithm. Avoid ECB mode and DES/RC2/RC4.")
}

// getInstanceReceiverIsJavaxCipher returns true when the navigation
// expression's receiver is javax.crypto.Cipher — either explicitly
// spelled `javax.crypto.Cipher.getInstance(...)` or a bare `Cipher`
// reference backed by an import of `javax.crypto.Cipher` with no
// conflicting user-defined Cipher class in the same file.
func getInstanceReceiverIsJavaxCipher(file *scanner.File, navExpr uint32) bool {
	if file == nil || navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return false
	}
	receiver := file.FlatNamedChild(navExpr, 0)
	text := strings.TrimSpace(file.FlatNodeText(receiver))
	if text == "javax.crypto.Cipher" {
		return true
	}
	if text != "Cipher" {
		return false
	}
	if getInstanceFileDeclaresCipherType(file) {
		return false
	}
	return getInstanceFileImportsJavaxCipher(file)
}

func getInstanceFileImportsJavaxCipher(file *scanner.File) bool {
	found := false
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		if found {
			return
		}
		text := strings.TrimSpace(file.FlatNodeText(node))
		text = strings.TrimPrefix(text, "import ")
		text = strings.TrimSuffix(text, ";")
		text = strings.TrimSpace(text)
		if text == "javax.crypto.Cipher" || text == "javax.crypto.*" {
			found = true
		}
	})
	return found
}

func getInstanceFileDeclaresCipherType(file *scanner.File) bool {
	found := false
	for _, nodeType := range []string{"class_declaration", "object_declaration", "type_alias"} {
		file.FlatWalkNodes(0, nodeType, func(node uint32) {
			if found {
				return
			}
			if extractIdentifierFlat(file, node) == "Cipher" {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
}

func getInstanceFirstStringArg(file *scanner.File, args uint32) (string, bool) {
	if file == nil || args == 0 {
		return "", false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if !file.FlatIsNamed(arg) {
			continue
		}
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		if expr == 0 {
			return "", false
		}
		switch file.FlatType(expr) {
		case "string_literal", "line_string_literal", "multi_line_string_literal":
			if flatContainsStringInterpolation(file, expr) {
				return "", false
			}
			text := file.FlatNodeText(expr)
			if strings.HasPrefix(text, `"""`) && strings.HasSuffix(text, `"""`) {
				return strings.TrimSuffix(strings.TrimPrefix(text, `"""`), `"""`), true
			}
			value, err := strconv.Unquote(text)
			if err != nil {
				return "", false
			}
			return value, true
		}
		return "", false
	}
	return "", false
}

// EasterEggRule detects comments containing easter egg references.
type EasterEggRule struct{ AndroidRule }

var easterEggRe = regexp.MustCompile(`(?i)\b(easter\s*egg|cheat\s*code|secret\s*(?:mode|menu|feature))\b`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *EasterEggRule) Confidence() float64 { return 0.75 }

func (r *EasterEggRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		// Only check comments
		if scanner.IsCommentLine(line) {
			if easterEggRe.MatchString(line) {
				ctx.Emit(r.Finding(file, i+1, 1,
					"Code contains easter egg / hidden feature reference. Review for security implications."))
			}
		}
	}
}

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
func (r *ExportedContentProviderRule) Confidence() float64 { return 0.75 }

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

func (r *ExportedContentProviderRule) check(ctx *v2.Context) {
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
func (r *ExportedReceiverRule) Confidence() float64 { return 0.75 }

func (r *ExportedReceiverRule) check(ctx *v2.Context) {
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
func (r *GrantAllUrisRule) Confidence() float64 { return 0.75 }

func (r *GrantAllUrisRule) check(ctx *v2.Context) {
	file, idx := ctx.File, ctx.Idx
	name := grantUriPermissionCallName(file, idx)
	if name != "grantUriPermission" && name != "grantUriPermissions" {
		return
	}
	confidence := grantUriPermissionConfidence(ctx, idx)
	if confidence <= 0 {
		return
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Overly broad URI permission grant. Consider restricting to specific URIs.")
	f.Confidence = confidence
	ctx.Emit(f)
}

func grantUriPermissionCallName(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "call_expression":
		return flatCallExpressionName(file, idx)
	case "method_invocation":
		return wrongViewCastCallName(file, idx)
	default:
		return ""
	}
}

func grantUriPermissionConfidence(ctx *v2.Context, idx uint32) float64 {
	file := ctx.File
	if file.FlatType(idx) == "method_invocation" {
		return grantUriPermissionJavaConfidence(file, idx)
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
				if grantUriTypeIsContext(ctx.Resolver, typ) {
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

func grantUriPermissionJavaConfidence(file *scanner.File, idx uint32) float64 {
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

func grantUriTypeIsContext(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType) bool {
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

// SecureRandomRule detects insecure random usage: java.util.Random where
// SecureRandom should be used, and deterministic SecureRandom.setSeed(long)
// calls that Android lint reports as SecureRandom issues.
type SecureRandomRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *SecureRandomRule) Confidence() float64 { return 0.85 }

func (r *SecureRandomRule) check(ctx *v2.Context) {
	if ctx == nil || ctx.File == nil || ctx.Idx == 0 {
		return
	}
	file := ctx.File
	switch file.FlatType(ctx.Idx) {
	case "call_expression":
		r.checkKotlinCall(ctx, file, ctx.Idx)
	case "method_invocation":
		r.checkJavaMethodInvocation(ctx, file, ctx.Idx)
	}
}

func (r *SecureRandomRule) checkKotlinCall(ctx *v2.Context, file *scanner.File, call uint32) {
	if r.isInsecureKotlinRandomCall(file, call) {
		ctx.Emit(r.Finding(file, file.FlatRow(call)+1, file.FlatCol(call)+1,
			"Using java.util.Random. Use java.security.SecureRandom for security-sensitive operations."))
		return
	}

	if !secureRandomIsKotlinSetSeedCall(file, call) {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(call)+1, file.FlatCol(call)+1,
		"Calling SecureRandom.setSeed() with a fixed or time-based seed makes output predictable. Use the default SecureRandom seeding."))
}

func (r *SecureRandomRule) isInsecureKotlinRandomCall(file *scanner.File, call uint32) bool {
	navExpr, _ := flatCallExpressionParts(file, call)
	insecure := false
	if navExpr != 0 {
		ids := flatNavigationIdentifierParts(file, navExpr)
		if len(ids) == 3 && ids[0] == "java" && ids[1] == "util" && ids[2] == "Random" {
			insecure = true
		}
	} else if flatCallExpressionName(file, call) == "Random" && secureRandomImportsJavaUtilRandom(file) {
		insecure = true
	}
	return insecure
}

func secureRandomIsKotlinSetSeedCall(file *scanner.File, call uint32) bool {
	navExpr, args := flatCallExpressionParts(file, call)
	if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "setSeed" {
		return false
	}
	arg := singleKotlinValueArgument(file, args)
	if arg == 0 || !secureRandomIsDeterministicSeedArgument(file, arg) {
		return false
	}
	receiver := flatNavigationReceiver(file, navExpr)
	return secureRandomKotlinReceiverIsSecureRandom(file, receiver)
}

func (r *SecureRandomRule) checkJavaMethodInvocation(ctx *v2.Context, file *scanner.File, call uint32) {
	if !secureRandomIsJavaSetSeedCall(file, call) {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(call)+1, file.FlatCol(call)+1,
		"Calling SecureRandom.setSeed() with a fixed or time-based seed makes output predictable. Use the default SecureRandom seeding."))
}

func secureRandomImportsJavaUtilRandom(file *scanner.File) bool {
	javaUtil := false
	kotlinRandom := false
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		switch missingPermissionIdentifierPath(file, node) {
		case "java.util.Random":
			javaUtil = true
		case "kotlin.random.Random":
			kotlinRandom = true
		}
	})
	return javaUtil && !kotlinRandom
}

func singleKotlinValueArgument(file *scanner.File, args uint32) uint32 {
	if file == nil || args == 0 {
		return 0
	}
	var out uint32
	count := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		if expr == 0 {
			continue
		}
		out = expr
		count++
	}
	if count != 1 {
		return 0
	}
	return out
}

func secureRandomIsDeterministicSeedArgument(file *scanner.File, arg uint32) bool {
	arg = flatUnwrapParenExpr(file, arg)
	switch file.FlatType(arg) {
	case "integer_literal", "long_literal", "decimal_integer_literal", "hex_integer_literal", "octal_integer_literal", "binary_integer_literal":
		return true
	case "call_expression":
		navExpr, args := flatCallExpressionParts(file, arg)
		return args != 0 && file.FlatNamedChildCount(args) == 0 && secureRandomIsSystemTimeNavigation(file, navExpr)
	case "method_invocation":
		return secureRandomIsJavaSystemTimeCall(file, arg)
	default:
		return false
	}
}

func secureRandomIsSystemTimeNavigation(file *scanner.File, navExpr uint32) bool {
	ids := flatNavigationIdentifierParts(file, navExpr)
	return len(ids) == 2 && ids[0] == "System" && (ids[1] == "currentTimeMillis" || ids[1] == "nanoTime")
}

func secureRandomKotlinReceiverIsSecureRandom(file *scanner.File, receiver uint32) bool {
	if file == nil || receiver == 0 {
		return false
	}
	switch file.FlatType(receiver) {
	case "call_expression":
		return secureRandomKotlinConstructorCall(file, receiver)
	case "simple_identifier":
		name := file.FlatNodeString(receiver, nil)
		return secureRandomKotlinIdentifierIsSecureRandom(file, name)
	case "navigation_expression":
		return secureRandomKotlinQualifiedConstructor(file, receiver)
	default:
		return false
	}
}

func secureRandomKotlinConstructorCall(file *scanner.File, call uint32) bool {
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr != 0 {
		return secureRandomKotlinQualifiedConstructor(file, navExpr)
	}
	return flatCallExpressionName(file, call) == "SecureRandom" && secureRandomImportsJavaSecuritySecureRandom(file)
}

func secureRandomKotlinQualifiedConstructor(file *scanner.File, navExpr uint32) bool {
	ids := flatNavigationIdentifierParts(file, navExpr)
	return len(ids) == 3 && ids[0] == "java" && ids[1] == "security" && ids[2] == "SecureRandom"
}

func secureRandomKotlinIdentifierIsSecureRandom(file *scanner.File, name string) bool {
	if name == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "property_declaration", func(prop uint32) {
		if found {
			return
		}
		decl, ok := file.FlatFindChild(prop, "variable_declaration")
		if !ok || !flatDeclarationContainsIdentifier(file, decl, name) {
			return
		}
		if secureRandomKotlinPropertyDeclaresSecureRandom(file, prop) {
			found = true
		}
	})
	return found
}

func secureRandomKotlinPropertyDeclaresSecureRandom(file *scanner.File, prop uint32) bool {
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type":
			if secureRandomTypeNodeNamesSecureRandom(file, child) {
				return true
			}
		case "call_expression":
			if secureRandomKotlinConstructorCall(file, child) {
				return true
			}
		}
	}
	return false
}

func flatDeclarationContainsIdentifier(file *scanner.File, node uint32, name string) bool {
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" && file.FlatNodeString(child, nil) == name {
			return true
		}
	}
	return false
}

func secureRandomImportsJavaSecuritySecureRandom(file *scanner.File) bool {
	importedSecureRandom := false
	importedKotlinRandom := false
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		switch missingPermissionIdentifierPath(file, node) {
		case "java.security.SecureRandom":
			importedSecureRandom = true
		case "kotlin.random.Random":
			importedKotlinRandom = true
		}
	})
	return importedSecureRandom && !importedKotlinRandom
}

func secureRandomTypeNodeNamesSecureRandom(file *scanner.File, node uint32) bool {
	text := strings.TrimSpace(file.FlatNodeText(node))
	return text == "SecureRandom" || text == "java.security.SecureRandom"
}

func secureRandomIsJavaSetSeedCall(file *scanner.File, call uint32) bool {
	if secureRandomJavaMethodName(file, call) != "setSeed" {
		return false
	}
	arg := singleJavaArgument(file, call)
	if arg == 0 || !secureRandomIsDeterministicSeedArgument(file, arg) {
		return false
	}
	receiver := secureRandomJavaMethodReceiver(file, call)
	if receiver == 0 {
		return false
	}
	if file.FlatType(receiver) == "object_creation_expression" {
		return secureRandomJavaObjectCreationIsSecureRandom(file, receiver)
	}
	if file.FlatType(receiver) == "identifier" {
		return secureRandomJavaIdentifierIsSecureRandom(file, file.FlatNodeString(receiver, nil))
	}
	return false
}

func secureRandomJavaMethodName(file *scanner.File, call uint32) string {
	last := ""
	for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "identifier" {
			last = file.FlatNodeString(child, nil)
		}
	}
	return last
}

func secureRandomJavaMethodReceiver(file *scanner.File, call uint32) uint32 {
	for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "identifier", "object_creation_expression", "method_invocation", "field_access":
			return child
		case "argument_list":
			return 0
		}
	}
	return 0
}

func singleJavaArgument(file *scanner.File, call uint32) uint32 {
	args, ok := file.FlatFindChild(call, "argument_list")
	if !ok {
		return 0
	}
	var out uint32
	count := 0
	for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		out = child
		count++
	}
	if count != 1 {
		return 0
	}
	return out
}

func secureRandomIsJavaSystemTimeCall(file *scanner.File, call uint32) bool {
	if name := secureRandomJavaMethodName(file, call); name != "currentTimeMillis" && name != "nanoTime" {
		return false
	}
	if args, ok := file.FlatFindChild(call, "argument_list"); !ok || file.FlatNamedChildCount(args) != 0 {
		return false
	}
	receiver := secureRandomJavaMethodReceiver(file, call)
	return receiver != 0 && file.FlatType(receiver) == "identifier" && file.FlatNodeString(receiver, nil) == "System"
}

func secureRandomJavaIdentifierIsSecureRandom(file *scanner.File, name string) bool {
	if name == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "local_variable_declaration", func(decl uint32) {
		if found {
			return
		}
		if !secureRandomJavaLocalDeclarationNames(file, decl, name) {
			return
		}
		if secureRandomJavaLocalDeclarationIsSecureRandom(file, decl) {
			found = true
		}
	})
	return found
}

func secureRandomJavaLocalDeclarationNames(file *scanner.File, decl uint32, name string) bool {
	var found bool
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "variable_declarator" {
			continue
		}
		if ident, ok := file.FlatFindChild(child, "identifier"); ok && file.FlatNodeString(ident, nil) == name {
			found = true
		}
	}
	return found
}

func secureRandomJavaLocalDeclarationIsSecureRandom(file *scanner.File, decl uint32) bool {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier":
			if secureRandomJavaTypeNodeNamesSecureRandom(file, child) {
				return true
			}
		case "variable_declarator":
			for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatType(gc) == "object_creation_expression" && secureRandomJavaObjectCreationIsSecureRandom(file, gc) {
					return true
				}
			}
		}
	}
	return false
}

func secureRandomJavaObjectCreationIsSecureRandom(file *scanner.File, node uint32) bool {
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "scoped_identifier":
			if secureRandomJavaTypeNodeNamesSecureRandom(file, child) {
				return true
			}
		}
	}
	return false
}

func secureRandomJavaTypeNodeNamesSecureRandom(file *scanner.File, node uint32) bool {
	text := strings.TrimSpace(file.FlatNodeText(node))
	if text == "java.security.SecureRandom" {
		return true
	}
	return text == "SecureRandom" && secureRandomJavaImportsSecureRandom(file)
}

func secureRandomJavaImportsSecureRandom(file *scanner.File) bool {
	found := false
	file.FlatWalkNodes(0, "import_declaration", func(node uint32) {
		if strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(node)), "import "), ";") == "java.security.SecureRandom" {
			found = true
		}
	})
	return found
}

// TrustedServerRule detects trust-all certificate patterns.
type TrustedServerRule struct{ AndroidRule }

// Confidence bumps this rule to tier-1 (high). The AST dispatch inspects
// class/object declarations for X509TrustManager supertypes with empty
// override bodies, and matches a short allow-list of known trust-all
// hostname-verifier identifiers. Both paths avoid the comment/string
// false positives that the previous line scan was prone to.
func (r *TrustedServerRule) Confidence() float64 { return 0.95 }

var trustedServerInsecureIdentifiers = map[string]bool{
	"TrustAllCertificates":        true,
	"AllowAllHostnameVerifier":    true,
	"ALLOW_ALL_HOSTNAME_VERIFIER": true,
}

func (r *TrustedServerRule) check(ctx *v2.Context) {
	if ctx == nil || ctx.File == nil || ctx.Idx == 0 {
		return
	}
	file := ctx.File
	node := ctx.Idx
	switch file.FlatType(node) {
	case "simple_identifier", "type_identifier":
		name := file.FlatNodeText(node)
		if !trustedServerInsecureIdentifiers[name] {
			return
		}
		// Skip the declaring site: `class TrustAllCertificates` or
		// `val AllowAllHostnameVerifier = ...` are declarations, not
		// usages of a known insecure API.
		if parent, ok := file.FlatParent(node); ok {
			switch file.FlatType(parent) {
			case "class_declaration", "object_declaration", "interface_declaration",
				"variable_declaration":
				return
			}
		}
		ctx.Emit(r.Finding(file, file.FlatRow(node)+1, file.FlatCol(node)+1,
			"Trusting all certificates or hostnames is insecure. Use proper certificate validation."))
	case "class_declaration", "object_literal", "object_declaration":
		if !trustedServerDeclaresX509(file, node) {
			return
		}
		if !trustedServerHasEmptyTrustCheck(file, node) {
			return
		}
		ctx.Emit(r.Finding(file, file.FlatRow(node)+1, file.FlatCol(node)+1,
			"Trust manager overrides checkClientTrusted/checkServerTrusted with an empty body. Perform real certificate validation."))
	}
}

func trustedServerDeclaresX509(file *scanner.File, decl uint32) bool {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		found := false
		file.FlatWalkAllNodes(child, func(n uint32) {
			if found {
				return
			}
			switch file.FlatType(n) {
			case "type_identifier", "simple_identifier":
				if file.FlatNodeText(n) == "X509TrustManager" {
					found = true
				}
			}
		})
		if found {
			return true
		}
	}
	return false
}

func trustedServerHasEmptyTrustCheck(file *scanner.File, decl uint32) bool {
	var body uint32
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "class_body" {
			body = child
			break
		}
	}
	if body == 0 {
		return false
	}
	empty := false
	for fn := file.FlatFirstChild(body); fn != 0; fn = file.FlatNextSib(fn) {
		if file.FlatType(fn) != "function_declaration" {
			continue
		}
		name := ""
		var fnBody uint32
		for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "simple_identifier":
				if name == "" {
					name = file.FlatNodeText(child)
				}
			case "function_body":
				fnBody = child
			}
		}
		if name != "checkClientTrusted" && name != "checkServerTrusted" {
			continue
		}
		if fnBody != 0 && trustedServerFunctionBodyIsEmpty(file, fnBody) {
			empty = true
			break
		}
	}
	return empty
}

func trustedServerFunctionBodyIsEmpty(file *scanner.File, body uint32) bool {
	// function_body is either `{ stmts... }` or `= expr`. An empty block
	// has only `{` and `}` named-false children plus at most a statements
	// wrapper with no children.
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "statements":
			if file.FlatFirstChild(child) != 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// WorldReadableFilesRule detects MODE_WORLD_READABLE usage.
type WorldReadableFilesRule struct{ AndroidRule }

// Confidence bumps this rule to tier-1 (high). AST dispatch on
// simple_identifier nodes means matches inside comments or string
// literals can no longer occur — those live under line_comment /
// string_content, which tree-sitter does not treat as identifiers.
func (r *WorldReadableFilesRule) Confidence() float64 { return 0.95 }

func (r *WorldReadableFilesRule) check(ctx *v2.Context) {
	if worldReadableIdentifierMatch(ctx, "MODE_WORLD_READABLE") {
		file := ctx.File
		ctx.Emit(r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
			"MODE_WORLD_READABLE is insecure. Use more restrictive file permissions."))
	}
}

// WorldWriteableFilesRule detects MODE_WORLD_WRITEABLE usage.
type WorldWriteableFilesRule struct{ AndroidRule }

// Confidence bumps this rule to tier-1 (high). AST dispatch on
// simple_identifier nodes means matches inside comments or string
// literals can no longer occur — those live under line_comment /
// string_content, which tree-sitter does not treat as identifiers.
func (r *WorldWriteableFilesRule) Confidence() float64 { return 0.95 }

func (r *WorldWriteableFilesRule) check(ctx *v2.Context) {
	if worldReadableIdentifierMatch(ctx, "MODE_WORLD_WRITEABLE") ||
		worldReadableIdentifierMatch(ctx, "MODE_WORLD_WRITABLE") {
		file := ctx.File
		ctx.Emit(r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
			"MODE_WORLD_WRITEABLE is insecure. Use more restrictive file permissions."))
	}
}

func worldReadableIdentifierMatch(ctx *v2.Context, want string) bool {
	if ctx == nil || ctx.File == nil || ctx.Idx == 0 {
		return false
	}
	file := ctx.File
	switch file.FlatType(ctx.Idx) {
	case "simple_identifier", "identifier":
	default:
		return false
	}
	if file.FlatNodeText(ctx.Idx) != want {
		return false
	}
	// Skip declaration sites (unlikely but harmless): `val MODE_WORLD_READABLE = ...`.
	if parent, ok := file.FlatParent(ctx.Idx); ok {
		switch file.FlatType(parent) {
		case "variable_declaration", "variable_declarator":
			return false
		}
	}
	return true
}

// =============================================================================
// Performance Rules
// =============================================================================

// DrawAllocationRule detects object allocations inside onDraw/draw methods.
// Uses AST dispatch on function_declaration to avoid brace-counting errors
// from string literals, comments, and multi-line signatures that broke the
// prior line-scan implementation.
type DrawAllocationRule struct {
	FlatDispatchBase
	AndroidRule
}

// drawAllocationAllocTypes is the fixed allow-list of graphics types whose
// construction inside onDraw/draw triggers a finding. Matched by unqualified
// type name.
var drawAllocationAllocTypes = map[string]bool{
	"Paint":                 true,
	"Rect":                  true,
	"RectF":                 true,
	"Path":                  true,
	"Matrix":                true,
	"LinearGradient":        true,
	"RadialGradient":        true,
	"SweepGradient":         true,
	"Bitmap":                true,
	"PorterDuffXfermode":    true,
	"Shader":                true,
	"ColorFilter":           true,
	"PorterDuffColorFilter": true,
	"BitmapShader":          true,
	"ComposeShader":         true,
	"Region":                true,
}

// Confidence reports a tier-2 (medium) base confidence. AST-based
// detection scopes allocations to the onDraw/draw function body,
// eliminating the prior regex/brace-scan false positives from string
// literals, comments, and multi-line signatures. Unqualified type-name
// matching against the fixed allow-list keeps this pattern-based
// without KAA type resolution. Classified per roadmap/17.
func (r *DrawAllocationRule) Confidence() float64 { return 0.85 }

func (r *DrawAllocationRule) check(ctx *v2.Context) {
	file := ctx.File
	fn := ctx.Idx
	if file == nil || fn == 0 || file.FlatType(fn) != "function_declaration" {
		return
	}
	name := flatFunctionName(file, fn)
	if name != "onDraw" && name != "draw" {
		return
	}
	if !file.FlatHasModifier(fn, "override") {
		return
	}
	body, _ := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return
	}
	file.FlatWalkNodes(body, "call_expression", func(call uint32) {
		if !drawAllocationIsAllocCall(file, call) {
			return
		}
		ctx.EmitAt(file.FlatRow(call)+1, file.FlatCol(call)+1,
			"Allocation in drawing code. Move allocations out of onDraw() for better performance.")
	})
}

// drawAllocationIsAllocCall reports whether a call_expression is an
// unqualified constructor-style call whose callee name appears in the
// graphics allow-list. Qualified calls (receiver.Foo()) and calls whose
// callee is not a simple identifier are ignored.
func drawAllocationIsAllocCall(file *scanner.File, call uint32) bool {
	if file == nil || file.FlatType(call) != "call_expression" {
		return false
	}
	// Require the callee to be a bare simple_identifier, not a
	// navigation_expression. `receiver.Paint()` is a method call, not a
	// constructor of the Paint type.
	var calleeName string
	for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "simple_identifier":
			calleeName = file.FlatNodeText(child)
		case "navigation_expression":
			return false
		case "call_suffix":
			// no-op
		}
		if calleeName != "" {
			break
		}
	}
	return drawAllocationAllocTypes[calleeName]
}

// FieldGetterRule detects using getter instead of direct field access in loops.
type FieldGetterRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *FieldGetterRule) NodeTypes() []string {
	return []string{"call_expression"}
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *FieldGetterRule) Confidence() float64 { return 0.75 }

func (r *FieldGetterRule) check(ctx *v2.Context) {
	file := ctx.File
	if file == nil || ctx.Idx == 0 {
		return
	}

	callIdx := ctx.Idx
	if file.FlatType(callIdx) != "call_expression" {
		return
	}

	// Check if this call is inside a for or while loop
	if !isCallInLoop(file, callIdx) {
		return
	}

	navExpr, args := flatCallExpressionParts(file, callIdx)
	if navExpr == 0 {
		return
	}

	methodName := flatNavigationExpressionLastIdentifier(file, navExpr)
	if methodName == "" || !isFieldGetterName(methodName) || nonFieldGetters[methodName] {
		return
	}

	// Check that there are no arguments
	if !hasZeroArguments(file, args) {
		return
	}

	ctx.EmitAt(file.FlatRow(callIdx)+1, file.FlatCol(callIdx)+1,
		"Getter call inside loop. Use direct field access for better performance.")
}

// isCallInLoop checks if a call_expression node is within a for_statement or while_statement
func isCallInLoop(file *scanner.File, callIdx uint32) bool {
	if file == nil || callIdx == 0 {
		return false
	}
	// Walk up the tree to find if we're inside a loop
	for current, ok := file.FlatParent(callIdx); ok; current, ok = file.FlatParent(current) {
		parentType := file.FlatType(current)
		if parentType == "for_statement" || parentType == "while_statement" || parentType == "do_while_statement" {
			return true
		}
	}
	return false
}

// Non-field-getter methods that start with "get" but should be filtered out.
var nonFieldGetters = map[string]bool{
	"getOrDefault": true,
	"getOrNull":    true,
	"getOrElse":    true,
	"getOrPut":     true,
	"getValue":     true,
	"getKey":       true,
}

// isFieldGetterName checks if a method name matches get[A-Z] pattern
func isFieldGetterName(methodName string) bool {
	if len(methodName) < 4 {
		return false
	}
	if !strings.HasPrefix(methodName, "get") {
		return false
	}
	secondChar := methodName[3]
	// Must be followed by uppercase letter
	return secondChar >= 'A' && secondChar <= 'Z'
}

// hasZeroArguments checks if the value_arguments is empty
func hasZeroArguments(file *scanner.File, args uint32) bool {
	if args == 0 {
		return true
	}
	if file == nil {
		return false
	}
	// Count named children of value_arguments
	namedCount := 0
	for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			namedCount++
		}
	}
	return namedCount == 0
}

// FloatMathRule detects deprecated FloatMath usage via AST dispatch.
type FloatMathRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence for structural match.
// With type resolver verifying FQN: 1.0. Classified per roadmap/17.
func (r *FloatMathRule) Confidence() float64 { return 0.75 }

func (r *FloatMathRule) NodeTypes() []string { return []string{"navigation_expression"} }

func (r *FloatMathRule) check(ctx *v2.Context) {
	if !floatMathReceiverIsFloatMath(ctx.File, ctx.Idx) {
		return
	}
	ctx.Emit(r.Finding(ctx.File, ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
		"FloatMath is deprecated. Use kotlin.math or java.lang.Math instead."))
}

// HandlerLeakRule detects non-static inner Handler classes via AST dispatch.
type HandlerLeakRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence for structural match.
// With type resolver verifying Handler inheritance: 0.90+. Classified per roadmap/17.
func (r *HandlerLeakRule) Confidence() float64 { return 0.75 }

func (r *HandlerLeakRule) NodeTypes() []string {
	return []string{"class_declaration", "object_literal", "object_creation_expression"}
}

func (r *HandlerLeakRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	nodeType := file.FlatType(idx)

	if nodeType == "class_declaration" {
		if !handlerClassMayCaptureOuterInstance(file, idx) {
			return
		}
		if handlerClassExtendsAndroidHandler(ctx, file, idx) {
			if handlerClassHasLooperSuperConstructor(file, idx) {
				return
			}
			ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				"This Handler class should be static or leaks might occur. Use a WeakReference to the outer class."))
		}
		return
	}

	if nodeType == "object_literal" {
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			if handlerSupertypeIsHandler(ctx, file, child) {
				ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Anonymous Handler may leak the enclosing class. Use a static inner class with a WeakReference."))
				return
			}
		}
		return
	}

	if nodeType == "object_creation_expression" {
		if !handlerJavaObjectCreationIsAnonymousHandler(ctx, file, idx) {
			return
		}
		ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			"Anonymous Handler may leak the enclosing class. Use a static inner class with a WeakReference."))
	}
}

func handlerClassMayCaptureOuterInstance(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "class_declaration" {
		return false
	}
	if file.Language == scanner.LangJava {
		if file.FlatHasModifier(idx, "static") {
			return false
		}
		parent, _ := file.FlatParent(idx)
		return parent != 0 && file.FlatType(parent) == "class_body"
	}
	return file.FlatHasModifier(idx, "inner")
}

func handlerClassExtendsAndroidHandler(ctx *v2.Context, file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "delegation_specifier":
			if handlerSupertypeIsHandler(ctx, file, child) {
				return true
			}
		case "superclass":
			if handlerJavaSuperclassIsHandler(ctx, file, child) {
				return true
			}
		}
	}
	return false
}

// handlerSupertypeIsHandler extracts a Kotlin supertype from delegation_specifier and checks
// whether resolver/import evidence points at android.os.Handler. Without resolver evidence,
// it preserves the existing simple-name fallback.
func handlerSupertypeIsHandler(ctx *v2.Context, file *scanner.File, delegIdx uint32) bool {
	if delegIdx == 0 || file.FlatType(delegIdx) != "delegation_specifier" {
		return false
	}
	// Find user_type or constructor_invocation->user_type
	ut, _ := file.FlatFindChild(delegIdx, "user_type")
	if ut == 0 {
		if ci, ok := file.FlatFindChild(delegIdx, "constructor_invocation"); ok {
			ut, _ = file.FlatFindChild(ci, "user_type")
		}
	}
	if ut == 0 {
		return false
	}
	// Extract the last type_identifier (simple name of the type)
	var lastIdent string
	for i := 0; i < file.FlatChildCount(ut); i++ {
		child := file.FlatChild(ut, i)
		if file.FlatType(child) == "type_identifier" {
			lastIdent = file.FlatNodeText(child)
		}
	}
	return handlerTypeIsAndroidHandler(ctx, file, lastIdent)
}

func handlerJavaSuperclassIsHandler(ctx *v2.Context, file *scanner.File, superclass uint32) bool {
	for child := file.FlatFirstChild(superclass); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "scoped_identifier":
			return handlerTypeIsAndroidHandler(ctx, file, file.FlatNodeText(child))
		}
	}
	return false
}

func handlerJavaObjectCreationIsAnonymousHandler(ctx *v2.Context, file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "object_creation_expression" {
		return false
	}
	if body, _ := file.FlatFindChild(idx, "class_body"); body == 0 {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "scoped_identifier":
			return handlerTypeIsAndroidHandler(ctx, file, file.FlatNodeText(child))
		}
	}
	return false
}

func handlerTypeIsAndroidHandler(ctx *v2.Context, file *scanner.File, typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return false
	}
	if typeName == "android.os.Handler" || strings.HasSuffix(typeName, ".android.os.Handler") {
		return true
	}
	if imported, ok := handlerFileImportForSimple(file, handlerSimpleTypeName(typeName)); ok {
		return imported == "android.os.Handler" || imported == "android.os.*"
	}
	resolver := typeinfer.TypeResolver(nil)
	if ctx != nil {
		resolver = ctx.Resolver
	}
	if resolver != nil {
		simple := handlerSimpleTypeName(typeName)
		if imported := resolver.ResolveImport(simple, file); imported != "" {
			return imported == "android.os.Handler"
		}
		if info := resolver.ClassHierarchy(typeName); info != nil {
			return info.FQN == "android.os.Handler" || handlerSupertypesContain(info.Supertypes, "android.os.Handler")
		}
		if simple != typeName {
			return false
		}
		return false
	}
	return typeName == "Handler" || strings.HasSuffix(typeName, ".Handler")
}

func handlerFileImportForSimple(file *scanner.File, simple string) (string, bool) {
	if file == nil || simple == "" {
		return "", false
	}
	var out string
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if out != "" {
			return
		}
		switch file.FlatType(idx) {
		case "import_header", "import_declaration":
			text := strings.TrimSpace(file.FlatNodeText(idx))
			text = strings.TrimPrefix(text, "import")
			text = strings.TrimSpace(strings.TrimSuffix(text, ";"))
			text = strings.TrimSuffix(text, ".*")
			if handlerSimpleTypeName(text) == simple {
				out = text
				return
			}
			if text == "android.os" {
				out = "android.os.*"
			}
		}
	})
	return out, out != ""
}

func handlerSimpleTypeName(typeName string) string {
	if i := strings.LastIndex(typeName, "."); i >= 0 {
		return typeName[i+1:]
	}
	return typeName
}

func handlerSupertypesContain(supertypes []string, want string) bool {
	for _, st := range supertypes {
		if st == want {
			return true
		}
	}
	return false
}

func handlerClassHasLooperSuperConstructor(file *scanner.File, classIdx uint32) bool {
	if file == nil || classIdx == 0 {
		return false
	}
	if file.Language == scanner.LangJava {
		body, _ := file.FlatFindChild(classIdx, "class_body")
		for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "constructor_declaration" && handlerJavaConstructorPassesLooperToSuper(file, child) {
				return true
			}
		}
		return false
	}
	return handlerKotlinPrimaryConstructorPassesLooperToHandler(file, classIdx)
}

func handlerJavaConstructorPassesLooperToSuper(file *scanner.File, ctor uint32) bool {
	params, _ := file.FlatFindChild(ctor, "formal_parameters")
	looperNames := map[string]bool{}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "formal_parameter" {
			continue
		}
		var hasLooper bool
		var name string
		for part := file.FlatFirstChild(child); part != 0; part = file.FlatNextSib(part) {
			switch file.FlatType(part) {
			case "type_identifier", "scoped_type_identifier", "scoped_identifier":
				if handlerTypeNameIsLooper(file.FlatNodeText(part)) {
					hasLooper = true
				}
			case "identifier":
				name = file.FlatNodeText(part)
			}
		}
		if hasLooper && name != "" {
			looperNames[name] = true
		}
	}
	if len(looperNames) == 0 {
		return false
	}
	body, _ := file.FlatFindChild(ctor, "constructor_body")
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "explicit_constructor_invocation" || !strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(child)), "super") {
			continue
		}
		if handlerArgumentListUsesAnyName(file, child, looperNames) {
			return true
		}
	}
	return false
}

func handlerKotlinPrimaryConstructorPassesLooperToHandler(file *scanner.File, classIdx uint32) bool {
	params, _ := file.FlatFindChild(classIdx, "primary_constructor")
	looperNames := map[string]bool{}
	file.FlatWalkAllNodes(params, func(idx uint32) {
		if file.FlatType(idx) != "class_parameter" {
			return
		}
		nameNode, _ := file.FlatFindChild(idx, "simple_identifier")
		if nameNode == 0 {
			return
		}
		if handlerKotlinNodeContainsLooperType(file, idx) {
			looperNames[file.FlatNodeText(nameNode)] = true
		}
	})
	if len(looperNames) == 0 {
		return false
	}
	for child := file.FlatFirstChild(classIdx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		if handlerKotlinDelegationInvokesHandlerWithLooper(file, child, looperNames) {
			return true
		}
	}
	return false
}

func handlerKotlinNodeContainsLooperType(file *scanner.File, idx uint32) bool {
	found := false
	file.FlatWalkAllNodes(idx, func(child uint32) {
		if file.FlatType(child) == "type_identifier" && handlerTypeNameIsLooper(file.FlatNodeText(child)) {
			found = true
		}
	})
	return found
}

func handlerKotlinDelegationInvokesHandlerWithLooper(file *scanner.File, delegIdx uint32, looperNames map[string]bool) bool {
	ctor, _ := file.FlatFindChild(delegIdx, "constructor_invocation")
	if ctor == 0 {
		return false
	}
	userType, _ := file.FlatFindChild(ctor, "user_type")
	typeNode := flatLastChildOfType(file, userType, "type_identifier")
	if userType == 0 || typeNode == 0 || file.FlatNodeText(typeNode) != "Handler" {
		return false
	}
	args, _ := file.FlatFindChild(ctor, "value_arguments")
	return handlerArgumentListUsesAnyName(file, args, looperNames)
}

func handlerArgumentListUsesAnyName(file *scanner.File, idx uint32, names map[string]bool) bool {
	used := false
	file.FlatWalkAllNodes(idx, func(child uint32) {
		if used {
			return
		}
		switch file.FlatType(child) {
		case "identifier", "simple_identifier":
			used = names[file.FlatNodeText(child)]
		}
	})
	return used
}

func handlerTypeNameIsLooper(typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	return typeName == "Looper" || typeName == "android.os.Looper" || strings.HasSuffix(typeName, ".Looper")
}

// floatMathReceiverIsFloatMath checks if navigation_expression starts with "FloatMath" receiver.
func floatMathReceiverIsFloatMath(file *scanner.File, navExprIdx uint32) bool {
	if navExprIdx == 0 || file.FlatType(navExprIdx) != "navigation_expression" {
		return false
	}
	// Get first named child (the receiver)
	if file.FlatNamedChildCount(navExprIdx) == 0 {
		return false
	}
	first := file.FlatNamedChild(navExprIdx, 0)
	if first == 0 {
		return false
	}
	// Check if it's a simple_identifier with text "FloatMath"
	if file.FlatType(first) == "simple_identifier" {
		return file.FlatNodeText(first) == "FloatMath"
	}
	return false
}

// RecycleRule detects missing recycle()/close() calls for resources.
type RecycleRule struct {
	FlatDispatchBase
	AndroidRule
}

var recycleTypeSet = map[string]struct{}{
	"TypedArray":      {},
	"Cursor":          {},
	"VelocityTracker": {},
	"Parcel":          {},
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *RecycleRule) Confidence() float64 { return 0.75 }

func (r *RecycleRule) check(ctx *v2.Context) {
	file := ctx.File
	idx := ctx.Idx

	// Extract property type and check if it's recyclable
	typeStr := extractPropertyTypeFlat(file, idx)
	if typeStr == "" {
		return
	}

	// Parse the type (may be wrapped in angle brackets or have space)
	typeStr = strings.TrimSpace(typeStr)
	typeStr = strings.TrimPrefix(typeStr, ":")
	typeStr = strings.TrimSpace(typeStr)

	// Check if it's one of the recyclable types (avoid matching generics like Flow<TypedArray>)
	var recycleType string
	for t := range recycleTypeSet {
		if typeStr == t {
			recycleType = t
			break
		}
	}
	if recycleType == "" {
		return
	}

	// Extract variable name using standard identifier extraction
	varName := extractIdentifierFlat(file, idx)
	if varName == "" {
		return
	}

	// Check if cleanup exists in the same scope
	if !recycleVariableHasCleanupFlat(file, idx, varName) {
		ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, 1,
			recycleType+" acquired but no cleanup found. Ensure recycle()/close()/.use {} is called in the same scope."))
	}
}

func extractPropertyTypeFlat(file *scanner.File, idx uint32) string {
	if file == nil || file.FlatType(idx) != "property_declaration" {
		return ""
	}
	// In property_declaration, the type annotation is inside variable_declaration
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "variable_declaration" {
			continue
		}
		// Look for colon followed by a type node in variable_declaration
		colonSeen := false
		for typeChild := file.FlatFirstChild(child); typeChild != 0; typeChild = file.FlatNextSib(typeChild) {
			childType := file.FlatType(typeChild)
			if childType == ":" {
				colonSeen = true
				continue
			}
			if colonSeen {
				// Return the first type-like node after colon
				if childType == "user_type" || childType == "simple_identifier" ||
					childType == "nullable_type" || childType == "function_type" ||
					childType == "parenthesized_type" || childType == "type_identifier" {
					return file.FlatNodeString(typeChild, nil)
				}
			}
		}
	}
	return ""
}

func extractPropertyNameFlat(file *scanner.File, idx uint32) string {
	if file == nil || file.FlatType(idx) != "property_declaration" {
		return ""
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" {
			return file.FlatNodeText(child)
		}
	}
	return ""
}

func recycleVariableHasCleanupFlat(file *scanner.File, idx uint32, varName string) bool {
	scope, ok := file.FlatParent(idx)
	if !ok {
		return false
	}

	end := file.FlatEndByte(idx)
	for i := 0; i < file.FlatChildCount(scope); i++ {
		child := file.FlatChild(scope, i)
		if file.FlatStartByte(child) <= end {
			continue
		}

		childText := file.FlatNodeText(child)
		if strings.Contains(childText, varName+".recycle()") ||
			strings.Contains(childText, varName+".close()") ||
			strings.Contains(childText, varName+".use") {
			return true
		}
	}

	return false
}

// =============================================================================
// I18N Rules
// =============================================================================

// ByteOrderMarkRule detects BOM (byte order mark) in files.
type ByteOrderMarkRule struct{ AndroidRule }

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 — the BOM check is a literal three-byte compare at the start
// of the file content. No heuristic path.
func (r *ByteOrderMarkRule) Confidence() float64 { return 0.95 }

func (r *ByteOrderMarkRule) check(ctx *v2.Context) {
	file := ctx.File
	// BOM is the first 3 bytes: EF BB BF (UTF-8 BOM)
	if len(file.Content) >= 3 &&
		file.Content[0] == 0xEF && file.Content[1] == 0xBB && file.Content[2] == 0xBF {
		ctx.Emit(r.Finding(file, 1, 1,
			"File contains a UTF-8 byte order mark (BOM). Remove the BOM for consistency."))
		return
	}
}
