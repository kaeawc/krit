package rules

// Android Lint Performance rules. Ported from AOSP Android Lint.
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/sourceheader"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// DrawAllocationRule detects object allocations inside onDraw/onMeasure/onLayout/layout methods,
// matching Android Lint's JavaPerformanceDetector: any constructor call, plus the Bitmap factory
// calls and Canvas.getClipBounds().
type DrawAllocationRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Without type resolution a capitalized
// call is taken as a constructor, so a capitalized factory function can be a false positive.
func (r *DrawAllocationRule) Confidence() float64 { return api.ConfidenceHigh }

// drawAllocationMethodParams maps the per-frame method names Lint inspects to their parameter types.
var drawAllocationMethodParams = map[string][]string{
	"onDraw":    {"Canvas"},
	"onMeasure": {"Int", "Int"},
	"onLayout":  {"Boolean", "Int", "Int", "Int", "Int"},
	"layout":    {"Int", "Int", "Int", "Int"},
}

const (
	drawAllocationMessage    = "Avoid object allocations during draw/layout operations (preallocate and reuse instead)"
	drawAllocationClipBounds = "Avoid object allocations during draw operations: Use `Canvas.getClipBounds(Rect)` instead of `Canvas.getClipBounds()` which allocates a temporary `Rect`"
)

func (r *DrawAllocationRule) check(ctx *api.Context) {
	file := ctx.File
	fn := ctx.Idx
	if file == nil || fn == 0 || file.FlatType(fn) != "function_declaration" {
		return
	}
	if !drawAllocationIsPerFrameMethod(file, fn) {
		return
	}
	body, _ := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return
	}
	inline := drawAllocationInlineClasses(file)
	file.FlatWalkNodes(body, "call_expression", func(call uint32) {
		msg := drawAllocationCallMessage(file, call, inline)
		if msg == "" || drawAllocationNotPerDraw(file, call, body) {
			return
		}
		ctx.EmitAt(file.FlatRow(call)+1, file.FlatCol(call)+1, msg)
	})
	file.FlatWalkNodes(body, "navigation_expression", func(nav uint32) {
		if drawAllocationIsClipBoundsProperty(file, nav) && !drawAllocationNotPerDraw(file, nav, body) {
			ctx.EmitAt(file.FlatRow(nav)+1, file.FlatCol(nav)+1, drawAllocationClipBounds)
		}
	})
}

// drawAllocationIsPerFrameMethod reports whether fn has the name and parameter types of onDraw,
// onMeasure, onLayout or layout. Lint checks no `override` modifier or View supertype.
func drawAllocationIsPerFrameMethod(file *scanner.File, fn uint32) bool {
	want, ok := drawAllocationMethodParams[flatFunctionName(file, fn)]
	if !ok {
		return false
	}
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		return false
	}
	i := 0
	for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
		if file.FlatType(p) != "parameter" {
			continue
		}
		typ, _ := file.FlatFindChild(p, "user_type")
		name := ""
		if typ != 0 {
			name = file.FlatNodeText(typ)
			name = name[strings.LastIndex(name, ".")+1:]
		}
		if i >= len(want) || name != want[i] {
			return false
		}
		i++
	}
	return i == len(want)
}

// drawAllocationInlineClasses returns the value classes declared in the file; Lint skips their
// constructors because they do not allocate.
func drawAllocationInlineClasses(file *scanner.File) map[string]bool {
	var out map[string]bool
	file.FlatWalkNodes(0, "class_declaration", func(cls uint32) {
		if !file.FlatHasModifier(cls, "value") && !hasAnnotationNamed(file, cls, "JvmInline") {
			return
		}
		if out == nil {
			out = map[string]bool{}
		}
		out[extractIdentifierFlat(file, cls)] = true
	})
	return out
}

// drawAllocationCallMessage returns the finding message for call, or "" when it does not allocate.
func drawAllocationCallMessage(file *scanner.File, call uint32, inline map[string]bool) string {
	name := flatCallExpressionName(file, call)
	if name == "" {
		return ""
	}
	nav, args := flatCallExpressionParts(file, call)
	if name[0] >= 'A' && name[0] <= 'Z' {
		if inline[name] || (nav != 0 && !drawAllocationIsPackagePath(file, nav)) {
			return ""
		}
		return drawAllocationMessage
	}
	if nav == 0 {
		return ""
	}
	recv := drawAllocationReceiverName(file, nav)
	switch {
	case recv == "Bitmap" && (name == "createBitmap" || name == "createScaledBitmap"):
		return drawAllocationMessage
	case recv == "BitmapFactory" && strings.HasPrefix(name, "decode"):
		return drawAllocationMessage
	case name == "getClipBounds" && hasZeroArguments(file, args):
		return drawAllocationClipBounds
	}
	return ""
}

// drawAllocationReceiverName returns the last identifier of the receiver in `Receiver.member`.
func drawAllocationReceiverName(file *scanner.File, nav uint32) string {
	recv := file.FlatNamedChild(nav, 0)
	if recv == 0 {
		return ""
	}
	text := file.FlatNodeText(recv)
	return text[strings.LastIndex(text, ".")+1:]
}

// drawAllocationIsPackagePath reports whether nav names a class, so that the call constructs it: lowercase package
// qualifiers as in `android.graphics.Path`, optionally followed by outer classes as in
// `android.graphics.BitmapFactory.Options`, or an outer class that the file imports or declares, as in
// `BitmapFactory.Options`.
func drawAllocationIsPackagePath(file *scanner.File, nav uint32) bool {
	text := file.FlatNodeText(nav)
	text = text[:strings.LastIndex(text, ".")+1]
	segs := strings.Split(strings.TrimSuffix(text, "."), ".")
	pkg := 0
	for pkg < len(segs) && segs[pkg] != "" && segs[pkg][0] >= 'a' && segs[pkg][0] <= 'z' {
		pkg++
	}
	for _, seg := range segs[pkg:] {
		if seg == "" || seg[0] < 'A' || seg[0] > 'Z' {
			return false
		}
	}
	if pkg > 0 || pkg == len(segs) {
		return true
	}
	if _, imported := lintImportsOf(file).bySimple[segs[0]]; imported {
		return true
	}
	return lintSameFileClass(file, segs[0]) != 0
}

// drawAllocationIsClipBoundsProperty reports `receiver.clipBounds` read as a Kotlin property.
func drawAllocationIsClipBoundsProperty(file *scanner.File, nav uint32) bool {
	if parent, ok := file.FlatParent(nav); ok && file.FlatType(parent) == "call_expression" {
		return false
	}
	suffix, ok := file.FlatFindChild(nav, "navigation_suffix")
	return ok && file.FlatNamedChildCount(nav) > 1 && strings.TrimPrefix(file.FlatNodeText(suffix), ".") == "clipBounds"
}

// drawAllocationNotPerDraw reports allocations Lint does not flag: nested class bodies and
// object literals (their methods are not per-frame methods), `throw` operands, and lazy
// initialization, where an `if` assigns a variable its own condition reads.
func drawAllocationNotPerDraw(file *scanner.File, call, body uint32) bool {
	for cur, ok := file.FlatParent(call); ok && cur != body; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_body", "object_literal", "class_declaration", "object_declaration":
			return true
		case "jump_expression":
			if strings.HasPrefix(file.FlatNodeText(cur), "throw") {
				return true
			}
		case "if_expression":
			if drawAllocationIsLazyIf(file, cur) {
				return true
			}
		}
	}
	return false
}

// drawAllocationIsLazyIf reports an if whose then-branch assigns a name that its condition mentions.
func drawAllocationIsLazyIf(file *scanner.File, ifExpr uint32) bool {
	cond := file.FlatNamedChild(ifExpr, 0)
	then, ok := file.FlatFindChild(ifExpr, "control_structure_body")
	if cond == 0 || !ok {
		return false
	}
	assigned := map[string]bool{}
	file.FlatWalkNodes(then, "assignment", func(asn uint32) {
		if target := file.FlatNamedChild(asn, 0); target != 0 {
			text := file.FlatNodeText(target)
			assigned[text[strings.LastIndex(text, ".")+1:]] = true
		}
	})
	if len(assigned) == 0 {
		return false
	}
	lazy := false
	drawAllocationConditionNames(file, cond, func(name string) {
		if assigned[name] {
			lazy = true
		}
	})
	return lazy
}

// drawAllocationConditionNames reports the variable names Lint's addReferencedVariables reads from an if condition:
// bare names, `this.x` and `super.x` selectors, and `::x.isInitialized`, through operators and parentheses. Calls and
// members of other receivers contribute nothing.
func drawAllocationConditionNames(file *scanner.File, n uint32, add func(string)) {
	switch file.FlatType(n) {
	case "simple_identifier":
		add(file.FlatNodeText(n))
	case "parenthesized_expression", "prefix_expression", "disjunction_expression", "conjunction_expression",
		"equality_expression", "comparison_expression", "additive_expression", "multiplicative_expression",
		"infix_expression", "elvis_expression", "range_expression":
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			if file.FlatIsNamed(c) {
				drawAllocationConditionNames(file, c, add)
			}
		}
	case "navigation_expression":
		recv := file.FlatNamedChild(n, 0)
		suffix, _ := file.FlatFindChild(n, "navigation_suffix")
		if recv == 0 || suffix == 0 || file.FlatNamedChildCount(n) != 2 {
			return
		}
		sel, _ := file.FlatFindChild(suffix, "simple_identifier")
		if sel == 0 {
			return
		}
		switch file.FlatType(recv) {
		case "this_expression", "super_expression":
			add(file.FlatNodeText(sel))
		case "callable_reference":
			if file.FlatNodeText(sel) == "isInitialized" && file.FlatNamedChildCount(recv) == 1 {
				if ref, ok := file.FlatFindChild(recv, "simple_identifier"); ok {
					add(file.FlatNodeText(ref))
				}
			}
		}
	}
}

// FieldGetterRule detects calls on self to a trivial getter, which are slower than reading the field.
type FieldGetterRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *FieldGetterRule) NodeTypes() []string {
	return []string{"call_expression", "method_invocation"}
}

// Confidence reports a tier-2 (medium) base confidence: the getter and field are matched in the
// same file by name and shape, without type resolution.
func (r *FieldGetterRule) Confidence() float64 { return api.ConfidenceMedium }

// fieldGetterBodyRe matches a getter body that only returns a field, as a block or an expression body.
var fieldGetterBodyRe = regexp.MustCompile(`^(?:\{\s*return\s+(?:this\.)?([A-Za-z_]\w*)\s*;?\s*\}|=\s*(?:this\.)?([A-Za-z_]\w*))$`)

func (r *FieldGetterRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	name, ok := fieldGetterSelfCallName(file, call)
	if !ok {
		return
	}
	cls, ok := flatEnclosingAncestor(file, call, "class_declaration", "object_declaration")
	if !ok {
		return
	}
	field := fieldGetterTrivialGetterField(file, cls, name)
	if field == "" || !fieldGetterClassDeclares(file, cls, field) {
		return
	}
	ctx.EmitAt(file.FlatRow(call)+1, file.FlatCol(call)+1,
		"Calling getter method `"+name+"()` on self is slower than field access (`"+field+"`).")
}

// fieldGetterSelfCallName returns the name of a zero-argument get/is call that is unqualified or on this.
func fieldGetterSelfCallName(file *scanner.File, call uint32) (string, bool) {
	var name string
	switch file.FlatType(call) {
	case "call_expression":
		name = flatCallExpressionName(file, call)
		navExpr, args := flatCallExpressionParts(file, call)
		if navExpr != 0 && strings.TrimSpace(file.FlatNodeText(file.FlatNamedChild(navExpr, 0))) != "this" {
			return "", false
		}
		if !hasZeroArguments(file, args) {
			return "", false
		}
	case "method_invocation":
		name = wrongViewCastCallName(file, call)
		if recv := javaMethodReceiverText(file, call); recv != "" && recv != "this" {
			return "", false
		}
		if len(javaArgumentExpressions(file, call)) != 0 {
			return "", false
		}
	default:
		return "", false
	}
	if !isFieldGetterName(name) && !fieldGetterIsPredicateName(name) {
		return "", false
	}
	return name, true
}

func fieldGetterIsPredicateName(name string) bool {
	return len(name) > 2 && strings.HasPrefix(name, "is") && name[2] >= 'A' && name[2] <= 'Z'
}

// fieldGetterTrivialGetterField returns the field returned by the zero-parameter method name declared
// directly in cls, or "" when there is no such method or it does more than return a field.
func fieldGetterTrivialGetterField(file *scanner.File, cls uint32, name string) string {
	body, ok := file.FlatFindChild(cls, "class_body")
	if !ok {
		return ""
	}
	for member := file.FlatFirstChild(body); member != 0; member = file.FlatNextSib(member) {
		switch file.FlatType(member) {
		case "function_declaration", "method_declaration":
		default:
			continue
		}
		if extractIdentifierFlat(file, member) != name {
			continue
		}
		var fnBody uint32
		params := 0
		for c := file.FlatFirstChild(member); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "function_value_parameters", "formal_parameters":
				params = file.FlatNamedChildCount(c)
			case "function_body", "block":
				fnBody = c
			}
		}
		if params != 0 || fnBody == 0 {
			return ""
		}
		text := strings.TrimSpace(file.FlatNodeText(fnBody))
		m := fieldGetterBodyRe.FindStringSubmatch(text)
		if m == nil {
			return ""
		}
		if m[1] != "" {
			return m[1]
		}
		return m[2]
	}
	return ""
}

// fieldGetterClassDeclares reports whether cls declares a field or property named field.
func fieldGetterClassDeclares(file *scanner.File, cls uint32, field string) bool {
	found := false
	file.FlatWalkAllNodes(cls, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "variable_declarator", "property_declaration", "class_parameter":
			found = extractIdentifierFlat(file, n) == field
		}
	})
	return found
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

// isFieldGetterName checks if a method name matches get[A-Z] pattern
func isFieldGetterName(methodName string) bool {
	if len(methodName) < 4 || !strings.HasPrefix(methodName, "get") {
		return false
	}
	return methodName[3] >= 'A' && methodName[3] <= 'Z'
}

// FloatMathRule detects deprecated FloatMath usage via AST dispatch.
type FloatMathRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence for structural match.
// With type resolver verifying FQN: 1.0. Classified per roadmap/17.
func (r *FloatMathRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *HandlerLeakRule) Confidence() float64 { return api.ConfidenceMedium }

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
			if handlerClassHasLooperConstructorParameter(file, idx) {
				return
			}
			// Java reports the class name like Lint; Kotlin keeps the declaration start the FIR twin reports.
			at := idx
			if file.Language == scanner.LangJava {
				at = handlerClassNameNode(file, idx)
			}
			ctx.Emit(r.Finding(file, file.FlatRow(at)+1, file.FlatCol(at)+1,
				"This `Handler` class should be static or leaks might occur ("+handlerClassQualifiedName(file, idx)+")"))
		}
		return
	}

	if nodeType == "object_literal" {
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			if handlerSupertypeIsHandler(ctx, file, child) {
				// Lint skips an anonymous Handler built on an explicit Looper.
				if ci, ok := file.FlatFindChild(child, "constructor_invocation"); ok {
					if args, ok := file.FlatFindChild(ci, "value_arguments"); ok && handlerArgumentsPassLooper(ctx, args) {
						return
					}
				}
				ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					handlerAnonymousMessage))
				return
			}
		}
		return
	}

	if nodeType == "object_creation_expression" {
		if !handlerJavaObjectCreationIsAnonymousHandler(ctx, file, idx) {
			return
		}
		if args, ok := file.FlatFindChild(idx, "argument_list"); ok && handlerArgumentsPassLooper(ctx, args) {
			return
		}
		ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, handlerAnonymousMessage))
	}
}

const handlerAnonymousMessage = "This `Handler` class should be static or leaks might occur (anonymous android.os.Handler)"

// handlerClassNameNode returns the identifier naming a class declaration, or the declaration itself.
func handlerClassNameNode(file *scanner.File, cls uint32) uint32 {
	for _, kind := range []string{"identifier", "type_identifier", "simple_identifier"} {
		if name, ok := file.FlatFindChild(cls, kind); ok {
			return name
		}
	}
	return cls
}

// handlerClassQualifiedName builds the dotted package and nesting name of a class.
func handlerClassQualifiedName(file *scanner.File, cls uint32) string {
	var parts []string
	for cur := cls; cur != 0; {
		if file.FlatType(cur) == "class_declaration" {
			parts = append([]string{file.FlatNodeText(handlerClassNameNode(file, cur))}, parts...)
		}
		parent, ok := file.FlatParent(cur)
		if !ok {
			break
		}
		cur = parent
	}
	// The package-qualified name, as Lint prints it.
	return qualifySourceName(sourcePackageName(file), strings.Join(parts, "."))
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

// handlerClassHasLooperConstructorParameter mirrors Lint's hasLooperConstructorParameter: a constructor of the class
// itself (primary or secondary in Kotlin) takes a parameter typed Looper, whether or not it reaches the Handler
// constructor. Lint takes such a Handler to run on a caller-chosen thread.
func handlerClassHasLooperConstructorParameter(file *scanner.File, classIdx uint32) bool {
	if file == nil || classIdx == 0 {
		return false
	}
	if file.Language == scanner.LangJava {
		body, _ := file.FlatFindChild(classIdx, "class_body")
		for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) != "constructor_declaration" {
				continue
			}
			params, _ := file.FlatFindChild(child, "formal_parameters")
			for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
				if file.FlatType(param) == "formal_parameter" && handlerJavaParameterIsLooper(file, param) {
					return true
				}
			}
		}
		return false
	}
	if ctor, ok := file.FlatFindChild(classIdx, "primary_constructor"); ok {
		for param := file.FlatFirstChild(ctor); param != 0; param = file.FlatNextSib(param) {
			if file.FlatType(param) == "class_parameter" && handlerKotlinParameterIsLooper(file, param) {
				return true
			}
		}
	}
	body, _ := file.FlatFindChild(classIdx, "class_body")
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if handlerKotlinSecondaryTakesLooper(file, child) {
			return true
		}
		// tree-sitter sometimes wraps a member in an ERROR node; the class's own members are one level down.
		if file.FlatType(child) == "ERROR" {
			for inner := file.FlatFirstChild(child); inner != 0; inner = file.FlatNextSib(inner) {
				if handlerKotlinSecondaryTakesLooper(file, inner) {
					return true
				}
			}
		}
	}
	return false
}

func handlerKotlinSecondaryTakesLooper(file *scanner.File, ctor uint32) bool {
	if file.FlatType(ctor) != "secondary_constructor" {
		return false
	}
	params, _ := file.FlatFindChild(ctor, "function_value_parameters")
	for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
		if file.FlatType(param) == "parameter" && handlerKotlinParameterIsLooper(file, param) {
			return true
		}
	}
	return false
}

// handlerKotlinParameterIsLooper reports a Kotlin parameter whose declared type is Looper, nullable or not. A vararg
// parameter is an array of Loopers, which Lint does not count.
func handlerKotlinParameterIsLooper(file *scanner.File, param uint32) bool {
	if prev, ok := file.FlatPrevSibling(param); ok && file.FlatType(prev) == "parameter_modifiers" && strings.Contains(file.FlatNodeText(prev), "vararg") {
		return false
	}
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "modifiers":
			if strings.Contains(file.FlatNodeText(child), "vararg") {
				return false
			}
		case "user_type", "nullable_type":
			return handlerTypeNameIsLooper(strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(child)), "?"))
		}
	}
	return false
}

// handlerJavaParameterIsLooper reports a Java formal parameter whose declared type is Looper.
func handlerJavaParameterIsLooper(file *scanner.File, param uint32) bool {
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "scoped_identifier":
			return handlerTypeNameIsLooper(file.FlatNodeText(child))
		}
	}
	return false
}

// handlerArgumentsPassLooper mirrors Lint's hasLooperArgument for an anonymous Handler: an argument of its superclass
// constructor call is a Looper. Without types, an argument that is a Looper accessor (`Looper.getMainLooper()`,
// `thread.looper`, `mainLooper`) or whose resolved or declared type is Looper counts.
func handlerArgumentsPassLooper(ctx *api.Context, args uint32) bool {
	file := ctx.File
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if !file.FlatIsNamed(arg) {
			continue
		}
		expr := arg
		if file.FlatType(arg) == "value_argument" {
			expr = flatValueArgumentExpression(file, arg)
		}
		if expr == 0 {
			continue
		}
		if handlerExprIsLooperArgument(ctx, expr) {
			return true
		}
	}
	return false
}

func handlerExprIsLooperArgument(ctx *api.Context, expr uint32) bool {
	file := ctx.File
	expr = flatUnwrapParenExpr(file, expr)
	if handlerExprIsLooperAccessor(file, expr) || handlerExprIsLooper(ctx, expr) {
		return true
	}
	switch file.FlatType(expr) {
	case "elvis_expression":
		for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) && handlerExprIsLooperArgument(ctx, child) {
				return true
			}
		}
	case "ternary_expression":
		for branch := 1; branch < file.FlatNamedChildCount(expr); branch++ {
			if handlerExprIsLooperArgument(ctx, file.FlatNamedChild(expr, branch)) {
				return true
			}
		}
	case "call_expression", "method_invocation":
		if file.FlatType(expr) == "call_expression" && flatCallExpressionName(file, expr) == "" {
			if inner := file.FlatNamedChild(expr, 0); file.FlatType(inner) == "call_expression" {
				return handlerExprIsLooperArgument(ctx, inner)
			}
		}
		if call, ok := lintCallAt(file, expr); ok && len(call.Args) > 0 &&
			(call.Name == "checkNotNull" || call.Name == "requireNotNull" || call.Name == "requireNonNull") {
			return handlerExprIsLooperArgument(ctx, call.Args[0])
		}
	}
	return false
}

// handlerLooperAccessors are the Android APIs that return a Looper, by the name the call or property uses.
var handlerLooperAccessors = map[string]bool{
	"getMainLooper": true, "myLooper": true, "getLooper": true, "mainLooper": true, "looper": true,
}

// handlerExprIsLooperAccessor reports a call or property read of a Looper accessor, such as `Looper.getMainLooper()`,
// `thread.looper` or a bare `mainLooper`. A bare `looper` name is left to the declared-type check.
func handlerExprIsLooperAccessor(file *scanner.File, expr uint32) bool {
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "postfix_expression":
		if file.FlatNamedChildCount(expr) > 0 {
			return handlerExprIsLooperAccessor(file, file.FlatNamedChild(expr, 0))
		}
	case "call_expression":
		return handlerLooperAccessors[flatCallExpressionName(file, expr)]
	case "navigation_expression":
		return handlerLooperAccessors[flatNavigationExpressionLastIdentifier(file, expr)]
	case "simple_identifier":
		return file.FlatNodeText(expr) == "mainLooper"
	case "method_invocation":
		if name, ok := file.FlatFindChild(expr, "identifier"); ok {
			last := name
			for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "identifier" {
					last = child
				}
			}
			return handlerLooperAccessors[file.FlatNodeText(last)]
		}
	}
	return false
}

// handlerExprIsLooper reports an expression typed android.os.Looper: by the resolver, or for a bare name, by the
// declared type of the parameter, local, or field it names.
func handlerExprIsLooper(ctx *api.Context, expr uint32) bool {
	file := ctx.File
	isName := file.FlatType(expr) == "simple_identifier" || file.FlatType(expr) == "identifier"
	if isName && handlerTypeNameIsLooper(strings.TrimSuffix(lintDeclaredTypeText(file, expr, file.FlatNodeText(expr)), "?")) {
		return true
	}
	if ctx.Resolver == nil {
		return false
	}
	typ := ctx.Resolver.ResolveFlatNode(expr, file)
	if (typ == nil || typ.Kind == typeinfer.TypeUnknown) && isName {
		typ = ctx.Resolver.ResolveByNameFlat(file.FlatNodeText(expr), expr, file)
	}
	return typ != nil && (typ.FQN == "android.os.Looper" || (typ.FQN == "" && typ.Name == "Looper"))
}

func handlerTypeNameIsLooper(typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	return typeName == "Looper" || typeName == "android.os.Looper"
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
	switch file.FlatType(first) {
	case "simple_identifier":
		// A bare name only means android.util.FloatMath when the file imports it.
		return file.FlatNodeText(first) == "FloatMath" && sourceImportsOrMentions(file, "android.util.FloatMath")
	case "navigation_expression":
		return strings.TrimSpace(file.FlatNodeText(first)) == "android.util.FloatMath"
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
func (r *RecycleRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *RecycleRule) check(ctx *api.Context) {
	file := ctx.File
	idx := ctx.Idx

	// Extract property type and check if it's recyclable
	typeStr := extractPropertyTypeFlat(file, idx)
	if typeStr == "" {
		typeStr = recycleResolvedInitializerType(ctx)
	}
	if typeStr == "" {
		return
	}

	// Parse the type (may be wrapped in angle brackets or have space)
	typeStr = strings.TrimSpace(typeStr)
	typeStr = strings.TrimPrefix(typeStr, ":")
	typeStr = strings.TrimSpace(typeStr)
	typeStr = strings.TrimSuffix(typeStr, "?")

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

	// Only a local acquired from a call owns the resource; class members and aliases do not.
	if parent, ok := file.FlatParent(idx); !ok || file.FlatType(parent) == "class_body" || file.FlatType(parent) == "source_file" {
		return
	}
	init := propertyInitializerExpression(file, idx)
	if init == 0 || file.FlatType(init) == "simple_identifier" || !recycleInitializerHasCall(file, init) {
		return
	}
	// Returning or passing the resource hands ownership to the caller.
	if recycleVariableEscapesFlat(file, idx, varName) {
		return
	}

	// Check if cleanup exists in the same scope
	if !recycleVariableHasCleanupFlat(file, idx, varName) {
		ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			recycleMessage(recycleType)))
	}
}

// recycleResolvedInitializerFQNs maps the resolved type of an inferred local to its recycle type.
var recycleResolvedInitializerFQNs = map[string]string{
	"android.content.res.TypedArray": "TypedArray",
	"android.database.Cursor":        "Cursor",
	"android.view.VelocityTracker":   "VelocityTracker",
	"android.os.Parcel":              "Parcel",
}

// recycleResolvedInitializerType returns the recycle type of `val x = call()` when the resolver knows the call's type.
func recycleResolvedInitializerType(ctx *api.Context) string {
	if ctx.Resolver == nil {
		return ""
	}
	init := propertyInitializerExpression(ctx.File, ctx.Idx)
	if init == 0 || ctx.File.FlatType(init) == "simple_identifier" {
		return ""
	}
	resolved := ctx.Resolver.ResolveFlatNode(init, ctx.File)
	if resolved == nil {
		return ""
	}
	return recycleResolvedInitializerFQNs[strings.TrimSuffix(resolved.FQN, "?")]
}

func recycleInitializerHasCall(file *scanner.File, init uint32) bool {
	if file.FlatType(init) == "call_expression" {
		return true
	}
	found := false
	file.FlatWalkNodes(init, "call_expression", func(uint32) { found = true })
	return found
}

// recycleVariableEscapesFlat reports whether the variable is used after its declaration other than as the receiver of a call.
func recycleVariableEscapesFlat(file *scanner.File, idx uint32, varName string) bool {
	scope, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	end := file.FlatEndByte(idx)
	escapes := false
	file.FlatWalkAllNodes(scope, func(n uint32) {
		if escapes || file.FlatStartByte(n) <= end || file.FlatType(n) != "simple_identifier" || !file.FlatNodeTextEquals(n, varName) {
			return
		}
		if p, ok := file.FlatParent(n); ok && file.FlatType(p) == "navigation_expression" && file.FlatNamedChild(p, 0) == n {
			return
		}
		escapes = true
	})
	return escapes
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
			if childType == "ERROR" {
				if declared := recoveredFieldPropertyType(file.FlatNodeText(typeChild)); declared != "" {
					return declared
				}
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

// recoveredFieldPropertyType reads the property type out of the ERROR node the
// grammar recovers for `val s: T field: F = ...`, where the node holds `T field`
// (or `: T field`) because the grammar predates explicit backing fields.
func recoveredFieldPropertyType(errText string) string {
	text := strings.TrimSpace(errText)
	text = strings.TrimSpace(strings.TrimSuffix(text, ":"))
	text, ok := strings.CutSuffix(text, "field")
	if !ok {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), ":"))
}

// recycleCleanupMethodNames is the set of callees that finalize a recyclable
// Android resource (TypedArray, Cursor, VelocityTracker, Parcel) when invoked
// on the resource itself.
var recycleCleanupMethodNames = map[string]bool{
	"recycle": true,
	"close":   true,
	"use":     true,
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

		if subtreeHasCallOnReceiver(file, child, varName, recycleCleanupMethodNames) {
			return true
		}
	}

	return false
}

// recycleMessage is CleanupDetector's wording: cursors and similar resources are freed with close(), the rest
// are recycled.
func recycleMessage(resourceType string) string {
	switch resourceType {
	case "Cursor":
		return "This `Cursor` should be freed up after use with `#close()`"
	case "TypedArray", "VelocityTracker", "Parcel", "Message", "MotionEvent", "Bitmap":
		return "This `" + resourceType + "` should be recycled after use with `#recycle()`"
	}
	return "This `" + resourceType + "` should be recycled after use with `#recycle()`"
}
