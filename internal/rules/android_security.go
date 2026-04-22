package rules

// Android Lint rules for Security, Performance, Accessibility, I18N, and RTL categories.
// Ported from AOSP Android Lint.
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

// Additional category constants not in android.go
const (
	ALCRTL AndroidLintCategory = "rtl"
)

// =============================================================================
// Security Rules
// =============================================================================

// AddJavascriptInterfaceRule detects addJavascriptInterface() calls.
type AddJavascriptInterfaceRule struct{ AndroidRule }

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *AddJavascriptInterfaceRule) Confidence() float64 { return 0.75 }

func (r *AddJavascriptInterfaceRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if strings.Contains(line, "addJavascriptInterface(") || strings.Contains(line, "addJavascriptInterface (") {
			ctx.Emit(r.Finding(file, i+1, 1,
				"addJavascriptInterface called. This can introduce XSS vulnerabilities on older Android versions."))
		}
	}
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

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *TrustedServerRule) Confidence() float64 { return 0.75 }

func (r *TrustedServerRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		// Detect common trust-all patterns
		if strings.Contains(line, "TrustAllCertificates") ||
			strings.Contains(line, "AllowAllHostnameVerifier") ||
			strings.Contains(line, "ALLOW_ALL_HOSTNAME_VERIFIER") ||
			strings.Contains(line, "trustAllCerts") ||
			strings.Contains(line, "X509TrustManager") && strings.Contains(line, "object") {
			ctx.Emit(r.Finding(file, i+1, 1,
				"Trusting all certificates or hostnames is insecure. Use proper certificate validation."))
		}
	}
}


// WorldReadableFilesRule detects MODE_WORLD_READABLE usage.
type WorldReadableFilesRule struct{ AndroidRule }

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WorldReadableFilesRule) Confidence() float64 { return 0.75 }

func (r *WorldReadableFilesRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if strings.Contains(line, "MODE_WORLD_READABLE") {
			ctx.Emit(r.Finding(file, i+1, 1,
				"MODE_WORLD_READABLE is insecure. Use more restrictive file permissions."))
		}
	}
}


// WorldWriteableFilesRule detects MODE_WORLD_WRITEABLE usage.
type WorldWriteableFilesRule struct{ AndroidRule }

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WorldWriteableFilesRule) Confidence() float64 { return 0.75 }

func (r *WorldWriteableFilesRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if strings.Contains(line, "MODE_WORLD_WRITEABLE") || strings.Contains(line, "MODE_WORLD_WRITABLE") {
			ctx.Emit(r.Finding(file, i+1, 1,
				"MODE_WORLD_WRITEABLE is insecure. Use more restrictive file permissions."))
		}
	}
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


// FloatMathRule detects deprecated FloatMath usage.
type FloatMathRule struct{ AndroidRule }

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *FloatMathRule) Confidence() float64 { return 0.75 }

func (r *FloatMathRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if strings.Contains(line, "FloatMath.") {
			ctx.Emit(r.Finding(file, i+1, 1,
				"FloatMath is deprecated. Use kotlin.math or java.lang.Math instead."))
		}
	}
}


// HandlerLeakRule detects non-static inner Handler classes that may leak.
type HandlerLeakRule struct{ AndroidRule }

var handlerClassRe = regexp.MustCompile(`(?:inner\s+)?class\s+\w+.*:\s*Handler\s*\(`)
var handlerInnerRe = regexp.MustCompile(`\binner\s+class\s+\w+`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *HandlerLeakRule) Confidence() float64 { return 0.75 }

func (r *HandlerLeakRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if handlerClassRe.MatchString(line) && handlerInnerRe.MatchString(line) {
			ctx.Emit(r.Finding(file, i+1, 1,
				"This Handler class should be static or leaks might occur. Use a WeakReference to the outer class."))
		}
		// Also detect anonymous Handler() object expressions
		if strings.Contains(line, "object : Handler(") {
			ctx.Emit(r.Finding(file, i+1, 1,
				"Anonymous Handler may leak the enclosing class. Use a static inner class with a WeakReference."))
		}
	}
}


// RecycleRule detects missing recycle()/close() calls for resources.
type RecycleRule struct{ AndroidRule }

var recycleTargets = []string{
	"obtainStyledAttributes", "obtainAttributes",
	"obtainTypedArray", "obtain(",
}
var recycleTypes = []string{
	"TypedArray", "Cursor", "VelocityTracker", "Parcel",
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
	fullContent := strings.Join(file.Lines, "\n")

	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		for _, target := range recycleTargets {
			if strings.Contains(line, target) {
				// Check if recycle() or close() appears in the same function scope
				if !strings.Contains(fullContent, "recycle()") && !strings.Contains(fullContent, ".close()") {
					ctx.Emit(r.Finding(file, i+1, 1,
						"Resource obtained but recycle()/close() not found. Ensure the resource is properly released."))
				}
			}
		}
		for _, typ := range recycleTypes {
			// Detect variable declarations like: val x = TypedArray or val x: TypedArray
			if strings.Contains(line, typ) && (strings.Contains(line, "val ") || strings.Contains(line, "var ")) {
				if !strings.Contains(fullContent, ".recycle()") && !strings.Contains(fullContent, ".close()") &&
					!strings.Contains(fullContent, ".use {") && !strings.Contains(fullContent, ".use{") {
					ctx.Emit(r.Finding(file, i+1, 1,
						typ+" acquired but no recycle()/close()/use{} found. Ensure the resource is properly released."))
				}
			}
		}
	}
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

