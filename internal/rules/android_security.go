package rules

// Android Lint rules for Security, Performance, Accessibility, I18N, and RTL categories.
// Ported from AOSP Android Lint.
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/

import (
	"regexp"
	"strconv"
	"strings"

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

func (r *AddJavascriptInterfaceRule) check(ctx *v2.Context) {
	if ctx == nil || ctx.File == nil || ctx.Idx == 0 {
		return
	}
	file := ctx.File
	if file.FlatType(ctx.Idx) != "call_expression" {
		return
	}
	if flatCallExpressionName(file, ctx.Idx) != "addJavascriptInterface" {
		return
	}
	navExpr, _ := flatCallExpressionParts(file, ctx.Idx)
	if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return
	}
	receiverExpr := file.FlatNamedChild(navExpr, 0)
	confidence, ok := addJavascriptInterfaceReceiverConfidence(ctx, receiverExpr)
	if !ok {
		return
	}
	line := file.FlatRow(ctx.Idx) + 1
	col := file.FlatCol(ctx.Idx) + 1
	f := r.Finding(file, line, col,
		"addJavascriptInterface called. This can introduce XSS vulnerabilities on older Android versions.")
	f.Confidence = confidence
	ctx.Emit(f)
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
type GrantAllUrisRule struct{ AndroidRule }

var grantUriRe = regexp.MustCompile(`\bgrantUriPermission[s]?\b`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *GrantAllUrisRule) Confidence() float64 { return 0.75 }

func (r *GrantAllUrisRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if grantUriRe.MatchString(line) {
			ctx.Emit(r.Finding(file, i+1, 1,
				"Overly broad URI permission grant. Consider restricting to specific URIs."))
		}
	}
}


// SecureRandomRule detects java.util.Random usage where SecureRandom should be used.
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
	if file.FlatType(ctx.Idx) != "call_expression" {
		return
	}
	navExpr, _ := flatCallExpressionParts(file, ctx.Idx)
	insecure := false
	if navExpr != 0 {
		ids := flatNavigationIdentifierParts(file, navExpr)
		if len(ids) == 3 && ids[0] == "java" && ids[1] == "util" && ids[2] == "Random" {
			insecure = true
		}
	} else if flatCallExpressionName(file, ctx.Idx) == "Random" && secureRandomImportsJavaUtilRandom(file) {
		insecure = true
	}
	if !insecure {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"Using java.util.Random. Use java.security.SecureRandom for security-sensitive operations."))
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
	if file.FlatType(ctx.Idx) != "simple_identifier" {
		return false
	}
	if file.FlatNodeText(ctx.Idx) != want {
		return false
	}
	// Skip declaration sites (unlikely but harmless): `val MODE_WORLD_READABLE = ...`.
	if parent, ok := file.FlatParent(ctx.Idx); ok {
		if file.FlatType(parent) == "variable_declaration" {
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
	"Paint":              true,
	"Rect":               true,
	"RectF":              true,
	"Path":               true,
	"Matrix":             true,
	"LinearGradient":     true,
	"RadialGradient":     true,
	"SweepGradient":      true,
	"Bitmap":             true,
	"PorterDuffXfermode": true,
	"Shader":             true,
	"ColorFilter":        true,
	"PorterDuffColorFilter": true,
	"BitmapShader":       true,
	"ComposeShader":      true,
	"Region":             true,
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
type FieldGetterRule struct{ AndroidRule }

var fieldGetterCallRe = regexp.MustCompile(`\.get[A-Z]\w*\(`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *FieldGetterRule) Confidence() float64 { return 0.75 }

func (r *FieldGetterRule) check(ctx *v2.Context) {
	file := ctx.File
	inLoop := false
	braceDepth := 0
	loopStartDepth := 0
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if !inLoop {
			if strings.HasPrefix(trimmed, "for ") || strings.HasPrefix(trimmed, "for(") ||
				strings.HasPrefix(trimmed, "while ") || strings.HasPrefix(trimmed, "while(") {
				inLoop = true
				loopStartDepth = braceDepth
			}
		}
		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
		if inLoop {
			if fieldGetterCallRe.MatchString(line) && !strings.HasPrefix(trimmed, "//") {
				ctx.Emit(r.Finding(file, i+1, 1,
					"Getter call inside loop. Use direct field access for better performance."))
			}
			if braceDepth <= loopStartDepth {
				inLoop = false
			}
		}
	}
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

func (r *HandlerLeakRule) NodeTypes() []string { return []string{"class_declaration", "object_literal"} }

func (r *HandlerLeakRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	nodeType := file.FlatType(idx)

	if nodeType == "class_declaration" {
		if !file.FlatHasModifier(idx, "inner") {
			return
		}
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			if handlerSupertypeIsHandler(file, child) {
				ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"This Handler class should be static or leaks might occur. Use a WeakReference to the outer class."))
				return
			}
		}
		return
	}

	if nodeType == "object_literal" {
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			if handlerSupertypeIsHandler(file, child) {
				ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Anonymous Handler may leak the enclosing class. Use a static inner class with a WeakReference."))
				return
			}
		}
	}
}

// handlerSupertypeIsHandler extracts supertype from delegation_specifier and checks if it's "Handler".
func handlerSupertypeIsHandler(file *scanner.File, delegIdx uint32) bool {
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
	return lastIdent == "Handler"
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


