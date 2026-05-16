package rules

// Android Lint Performance rules. Ported from AOSP Android Lint.
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/sourceheader"
	"github.com/kaeawc/krit/internal/typeinfer"
)

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

func (r *DrawAllocationRule) check(ctx *api.Context) {
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

func (r *FieldGetterRule) check(ctx *api.Context) {
	file := ctx.File
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

func (r *FloatMathRule) check(ctx *api.Context) {
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

func (r *HandlerLeakRule) check(ctx *api.Context) {
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

func handlerClassExtendsAndroidHandler(ctx *api.Context, file *scanner.File, idx uint32) bool {
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
func handlerSupertypeIsHandler(ctx *api.Context, file *scanner.File, delegIdx uint32) bool {
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

func handlerJavaSuperclassIsHandler(ctx *api.Context, file *scanner.File, superclass uint32) bool {
	for child := file.FlatFirstChild(superclass); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "scoped_identifier":
			return handlerTypeIsAndroidHandler(ctx, file, file.FlatNodeText(child))
		}
	}
	return false
}

func handlerJavaObjectCreationIsAnonymousHandler(ctx *api.Context, file *scanner.File, idx uint32) bool {
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

func handlerTypeIsAndroidHandler(ctx *api.Context, file *scanner.File, typeName string) bool {
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
			text := sourceheader.FirstHeaderLine(file.FlatNodeText(idx), "import")
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

func (r *RecycleRule) check(ctx *api.Context) {
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
