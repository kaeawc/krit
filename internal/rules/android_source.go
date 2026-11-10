package rules

// Android Lint source-only rules: 13 checks that analyze Kotlin/Java source
// using tree-sitter AST or line scanning. No XML infrastructure needed.
//
// Ported from AOSP Android Lint detectors:
//   FragmentDetector, GetSignaturesDetector, JavaPerformance (SparseArray, ValueOf),
//   LogDetector (LONG_TAG, WRONG_TAG), NonInternationalizedSmsDetector,
//   ServiceCastDetector, ToastDetector, ViewConstructorDetector, ViewTagDetector,
//   WrongImportDetector, LayoutInflationDetector.

import (
	"strconv"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// =====================================================================
// 1. FragmentConstructorRule
// =====================================================================

// kotlinMapExtensions are Kotlin Map extension functions that SparseArray does not offer.
var kotlinMapExtensions = map[string]bool{
	"getOrPut": true, "getOrElse": true, "mapValues": true, "mapKeys": true, "filterKeys": true, "filterValues": true,
	"toMap": true, "toList": true, "withDefault": true, "plus": true, "minus": true, "asSequence": true, "forEach": true,
	"entries": true, "keys": true, "values": true, "iterator": true, "getOrDefault": true, "putIfAbsent": true,
	"computeIfAbsent": true, "computeIfPresent": true, "compute": true, "merge": true, "putAll": true,
	"containsValue": true, "filter": true, "map": true, "flatMap": true, "toMutableMap": true,
}

// hashMapEscapesAsMap reports a HashMap that is typed as a Map, passed as an argument, returned, or used
// with Kotlin map extensions, so it cannot become a SparseArray.
func hashMapEscapesAsMap(file *scanner.File, call uint32) bool {
	for cur, ok := file.FlatParent(call); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "value_argument", "jump_expression":
			return true
		case "property_declaration":
			name := extractIdentifierFlat(file, cur)
			if typ := extractPropertyTypeFlat(file, cur); strings.Contains(typ, "Map<") && !strings.Contains(typ, "HashMap<") {
				return true
			}
			scope, ok := file.FlatParent(cur)
			if !ok || name == "" {
				return false
			}
			escapes := false
			file.FlatWalkNodes(scope, "simple_identifier", func(ident uint32) {
				if escapes || !file.FlatNodeTextEquals(ident, name) {
					return
				}
				parent, ok := file.FlatParent(ident)
				if !ok {
					return
				}
				switch file.FlatType(parent) {
				case "value_argument", "jump_expression":
					escapes = true
				case "function_body":
					// `fun f() = map` returns the variable.
					escapes = true
				case "for_statement":
					// Iterating the map itself needs Map's iterator.
					escapes = file.FlatNamedChild(parent, 0) != ident
				case "navigation_expression":
					if file.FlatNamedChild(parent, 0) == ident && kotlinMapExtensions[navigationMemberName(file, parent)] {
						escapes = true
					}
				}
			})
			return escapes
		case "assignment":
			// `target = HashMap()` where the target is declared as a Map.
			target := file.FlatNamedChild(cur, 0)
			if target == 0 || file.FlatType(target) != "directly_assignable_expression" {
				return false
			}
			return fileDeclaresMapProperty(file, strings.TrimSpace(file.FlatNodeText(target)))
		case "function_body":
			// An expression body is returned as the declared type.
			fn, ok := file.FlatParent(cur)
			if !ok || file.FlatType(fn) != "function_declaration" {
				return false
			}
			typ := extractFunctionReturnTypeNameFlat(file, fn)
			return (strings.HasPrefix(typ, "Map<") || strings.HasPrefix(typ, "MutableMap<")) && !strings.Contains(typ, "HashMap<")
		case "statements", "class_body", "lambda_literal", "function_declaration":
			return false
		}
	}
	return false
}

// fileDeclaresMapProperty reports a property named name declared in file with a Map or MutableMap type.
func fileDeclaresMapProperty(file *scanner.File, name string) bool {
	found := false
	file.FlatWalkNodes(0, "property_declaration", func(prop uint32) {
		if found || extractIdentifierFlat(file, prop) != name {
			return
		}
		typ := extractPropertyTypeFlat(file, prop)
		found = (strings.HasPrefix(typ, "Map<") || strings.HasPrefix(typ, "MutableMap<")) && !strings.Contains(typ, "HashMap<")
	})
	return found
}

// navigationMemberName returns the name selected by a navigation_expression such as `map.getOrPut`.
func navigationMemberName(file *scanner.File, nav uint32) string {
	return flatNavigationExpressionLastIdentifier(file, nav)
}

// fragmentLaunchedFromInstance reports a launchFragment call that is handed an instance of className,
// the way fragment tests build one with constructor arguments.
func fragmentLaunchedFromInstance(file *scanner.File, className string) bool {
	if className == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "call_expression", func(call uint32) {
		if found {
			return
		}
		switch flatCallNameAny(file, call) {
		case "launchFragment", "launchFragmentInContainer":
			found = strings.Contains(file.FlatNodeText(call), className+"(")
		}
	})
	return found
}

// FragmentConstructorRule flags Fragment subclasses that have non-default
// (parameterized) constructors without a no-arg constructor. The Android
// framework re-instantiates fragments via reflection using the no-arg ctor.
type FragmentConstructorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *FragmentConstructorRule) Confidence() float64 { return api.ConfidenceMedium }

// fragmentSuperclasses covers common Fragment base classes.
var fragmentSuperclasses = []string{
	"Fragment", "DialogFragment", "ListFragment", "PreferenceFragment",
	"PreferenceFragmentCompat", "BottomSheetDialogFragment",
}

type fragmentConstructorState struct {
	hasNoArgCtor bool
	hasParamCtor bool
	// hasInjectCtor is true when a constructor is DI-annotated, which implies a custom FragmentFactory.
	hasInjectCtor bool
}

// fragmentCtorIsInjectAnnotatedFlat reports whether a constructor header carries @Inject or @AssistedInject.
func fragmentCtorIsInjectAnnotatedFlat(file *scanner.File, ctor uint32) bool {
	text := file.FlatNodeText(ctor)
	if paren := strings.Index(text, "("); paren >= 0 {
		text = text[:paren]
	}
	for _, ann := range strings.Fields(text) {
		name := androidAnnotationSimpleName(ann)
		if strings.HasPrefix(ann, "@") && (name == "Inject" || name == "AssistedInject") {
			return true
		}
	}
	return false
}

func fragmentConstructorStateFlat(file *scanner.File, classDecl uint32) fragmentConstructorState {
	state := fragmentConstructorState{}
	hasPrimary, hasSecondary, implicitPrimary := false, false, false
	// record notes one constructor. A primary constructor whose parameters all have defaults, or a
	// secondary one annotated @JvmOverloads, also gets a JVM no-arg overload. Visibility is not considered.
	record := func(ctor uint32, count, defaults int, overloads bool) {
		if fragmentCtorIsInjectAnnotatedFlat(file, ctor) {
			state.hasInjectCtor = true
		}
		if count == 0 || (defaults == count && overloads) {
			state.hasNoArgCtor = true
			return
		}
		state.hasParamCtor = true
	}
	for child := file.FlatFirstChild(classDecl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "delegation_specifier":
			// `: Base()` with no explicit constructor means an implicit no-arg primary constructor.
			if _, ok := file.FlatFindChild(child, "constructor_invocation"); ok {
				implicitPrimary = true
			}
		case "primary_constructor":
			hasPrimary = true
			count, defaults := constructorParameterDefaultsFlat(file, child, "class_parameter")
			record(child, count, defaults, true)
		case "class_body":
			file.FlatWalkNodes(child, "secondary_constructor", func(ctor uint32) {
				if nearestClassBodyOwnerFlat(file, ctor) != classDecl {
					return
				}
				hasSecondary = true
				count, defaults := constructorParameterDefaultsFlat(file, ctor, "parameter")
				record(ctor, count, defaults, fragmentCtorHeaderHasAnnotation(file, ctor, "JvmOverloads"))
			})
		}
	}
	// Without a primary constructor Kotlin generates no implicit no-arg constructor.
	if !hasPrimary && (implicitPrimary || !hasSecondary) {
		state.hasNoArgCtor = true
	}
	return state
}

// fragmentCtorHeaderHasAnnotation reports whether the constructor header (before its parameter list) carries @name.
func fragmentCtorHeaderHasAnnotation(file *scanner.File, ctor uint32, name string) bool {
	text := file.FlatNodeText(ctor)
	if paren := strings.Index(text, "("); paren >= 0 {
		text = text[:paren]
	}
	for _, w := range strings.Fields(text) {
		if strings.HasPrefix(w, "@") && androidAnnotationSimpleName(w) == name {
			return true
		}
	}
	return false
}

// nearestClassBodyOwnerFlat returns the class_declaration whose body directly holds node, or 0.
func nearestClassBodyOwnerFlat(file *scanner.File, node uint32) uint32 {
	for cur, ok := file.FlatParent(node); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration":
			return cur
		}
	}
	return 0
}

func constructorParameterDefaultsFlat(file *scanner.File, ctor uint32, paramType string) (count int, defaults int) {
	file.FlatWalkNodes(ctor, paramType, func(param uint32) {
		if nearestConstructorAncestorFlat(file, param) != ctor {
			return
		}
		count++
		if parameterHasDefaultValueFlat(file, param) {
			defaults++
		}
	})
	return count, defaults
}

func nearestConstructorAncestorFlat(file *scanner.File, idx uint32) uint32 {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "primary_constructor", "secondary_constructor":
			return cur
		case "class_declaration", "class_body":
			return 0
		}
	}
	return 0
}

func parameterHasDefaultValueFlat(file *scanner.File, param uint32) bool {
	// A function or secondary-constructor parameter keeps its `= default` as following siblings.
	if next := file.FlatNextSib(param); next != 0 && !file.FlatIsNamed(next) && file.FlatNodeTextEquals(next, "=") {
		return true
	}
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) && file.FlatNodeTextEquals(child, "=") {
			return true
		}
	}
	return false
}

// =====================================================================
// 2. GetSignaturesRule
// =====================================================================

// GetSignaturesRule flags use of the deprecated PackageManager.GET_SIGNATURES
// constant. GET_SIGNING_CERTIFICATES should be used instead (API 28+).
type GetSignaturesRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *GetSignaturesRule) Confidence() float64 { return api.ConfidenceMedium }

const getSignaturesFlagValue int64 = 0x40

func getSignaturesCallUsesDeprecatedFlag(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 || flatCallExpressionName(file, call) != "getPackageInfo" {
		return false
	}
	if apiGuardedByVersionCheckFlat(file, call) {
		return false
	}
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return false
	}
	flagArg := flatPositionalValueArgument(file, args, 1)
	if flagArg == 0 {
		flagArg = flatNamedValueArgument(file, args, "flags")
	}
	if flagArg == 0 {
		return false
	}
	expr := flatValueArgumentExpression(file, flagArg)
	if getSignaturesExprContainsFlag(file, expr) {
		return true
	}
	name := flatReferenceSimpleName(file, expr)
	if name == "" {
		return false
	}
	fn, ok := flatEnclosingFunction(file, call)
	if !ok {
		return false
	}
	return functionLocalInitializerContainsGetSignatures(file, fn, name, call)
}

// getSignaturesJavaCallUsesDeprecatedFlag reports a Java getPackageInfo call whose flags argument has GET_SIGNATURES.
func getSignaturesJavaCallUsesDeprecatedFlag(file *scanner.File, call uint32) bool {
	if wrongViewCastCallName(file, call) != "getPackageInfo" {
		return false
	}
	args := javaArgumentExpressions(file, call)
	if len(args) < 2 {
		return false
	}
	staticImport := sourceImportsOrMentions(file, "android.content.pm.PackageManager.GET_SIGNATURES")
	found := false
	visit := func(node uint32) {
		switch file.FlatType(node) {
		case "identifier":
			if staticImport && file.FlatNodeText(node) == "GET_SIGNATURES" {
				found = true
			}
		case "field_access":
			text := strings.TrimSpace(file.FlatNodeText(node))
			found = found || text == "PackageManager.GET_SIGNATURES" || strings.HasSuffix(text, ".PackageManager.GET_SIGNATURES")
		case "decimal_integer_literal", "hex_integer_literal":
			if v, ok := parseIntegerLiteralForFlag(file.FlatNodeText(node)); ok && v&getSignaturesFlagValue != 0 {
				found = true
			}
		}
	}
	visit(args[1])
	file.FlatWalkAllNodes(args[1], visit)
	return found
}

func getSignaturesExprContainsFlag(file *scanner.File, expr uint32) bool {
	if file == nil || expr == 0 {
		return false
	}
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "simple_identifier":
		// A bare GET_SIGNATURES must be imported from PackageManager to be the platform flag.
		return file.FlatNodeText(expr) == "GET_SIGNATURES" &&
			(sourceImportsOrMentions(file, "android.content.pm.PackageManager.GET_SIGNATURES") ||
				sourceImportsOrMentions(file, "android.content.pm.PackageManager.*"))
	case "navigation_expression":
		if flatNavigationExpressionLastIdentifier(file, expr) != "GET_SIGNATURES" {
			return false
		}
		qualifier := strings.TrimSpace(file.FlatNodeText(file.FlatNamedChild(expr, 0)))
		return qualifier == "PackageManager" || strings.HasSuffix(qualifier, ".PackageManager")
	case "integer_literal", "long_literal", "hex_literal":
		v, ok := parseIntegerLiteralForFlag(file.FlatNodeText(expr))
		return ok && (v&getSignaturesFlagValue) != 0
	}
	found := false
	file.FlatWalkAllNodes(expr, func(node uint32) {
		if found || node == expr {
			return
		}
		if getSignaturesExprContainsFlag(file, node) {
			found = true
		}
	})
	return found
}

func flatReferenceSimpleName(file *scanner.File, expr uint32) string {
	if file == nil || expr == 0 {
		return ""
	}
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "simple_identifier":
		return file.FlatNodeText(expr)
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifier(file, expr)
	default:
		return ""
	}
}

func functionLocalInitializerContainsGetSignatures(file *scanner.File, fn uint32, name string, before uint32) bool {
	if file == nil || fn == 0 || name == "" {
		return false
	}
	found := false
	targetRow := file.FlatRow(before)
	file.FlatWalkNodes(fn, "property_declaration", func(decl uint32) {
		if found || file.FlatRow(decl) > targetRow || propertyDeclarationName(file, decl) != name {
			return
		}
		if getSignaturesExprContainsFlag(file, propertyInitializerExpression(file, decl)) {
			found = true
		}
	})
	return found
}

func parseIntegerLiteralForFlag(text string) (int64, bool) {
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(strings.TrimSuffix(text, "L"), "l")
	text = strings.ReplaceAll(text, "_", "")
	if text == "" {
		return 0, false
	}
	base := 10
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		base = 16
		text = text[2:]
	} else if strings.HasPrefix(text, "0b") || strings.HasPrefix(text, "0B") {
		base = 2
		text = text[2:]
	}
	v, err := strconv.ParseInt(text, base, 64)
	return v, err == nil
}

// =====================================================================
// 3. SparseArrayRule
// =====================================================================

// SparseArrayRule flags HashMap<Integer, ...> / HashMap<Int, ...> usage where
// SparseArray or SparseIntArray would be more efficient on Android.
type SparseArrayRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SparseArrayRule) Confidence() float64 { return api.ConfidenceHigher }

// =====================================================================
// 4. UseValueOfRule
// =====================================================================

// UseValueOfRule flags direct constructor calls for boxed primitive types
// (e.g., Integer(42), Boolean(true)) where valueOf() should be used instead
// for object reuse.
type UseValueOfRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *UseValueOfRule) Confidence() float64 { return api.ConfidenceHigher }

var boxedPrimitiveConstructors = map[string]bool{
	"Integer":   true,
	"Long":      true,
	"Float":     true,
	"Double":    true,
	"Byte":      true,
	"Boolean":   true,
	"Character": true,
}

// boxedConstructorName returns the boxed type name of a bare or java.lang-qualified type reference.
func boxedConstructorName(text string) (string, bool) {
	name := strings.TrimPrefix(strings.TrimSpace(text), "java.lang.")
	return name, boxedPrimitiveConstructors[name]
}

// useValueOfMessage mirrors Lint's message; multi-line arguments are elided.
func useValueOfMessage(typeName, arg string) string {
	if strings.Contains(arg, "\n") {
		arg = "..."
	}
	return "Use `" + typeName + ".valueOf(" + arg + ")` instead"
}

// checkUseValueOfKotlinCall handles the Kotlin `Integer(42)` form
// (a `call_expression` whose callee is a bare boxed-primitive identifier).
func checkUseValueOfKotlinCall(ctx *api.Context, r *UseValueOfRule, file *scanner.File, idx uint32) {
	// The callee is a bare boxed type or the same type qualified with java.lang.
	firstNamed := file.FlatFirstChild(idx)
	for firstNamed != 0 && !file.FlatIsNamed(firstNamed) {
		firstNamed = file.FlatNextSib(firstNamed)
	}
	if firstNamed == 0 {
		return
	}
	typeName, ok := boxedConstructorName(file.FlatNodeText(firstNamed))
	if !ok {
		return
	}
	args := flatCallKeyArguments(file, idx)
	argCount := 0
	var firstArg uint32
	for a := file.FlatFirstChild(args); a != 0; a = file.FlatNextSib(a) {
		if file.FlatType(a) == "value_argument" {
			argCount++
			if firstArg == 0 {
				firstArg = a
			}
		}
	}
	if argCount != 1 {
		return
	}
	argText := ""
	if firstArg != 0 {
		argText = file.FlatNodeText(firstArg)
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, useValueOfMessage(typeName, argText))
	if firstArg != 0 {
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(idx)),
			EndByte:     int(file.FlatEndByte(idx)),
			Replacement: typeName + ".valueOf(" + argText + ")",
		}
	}
	ctx.Emit(f)
}

// checkUseValueOfJavaNew handles the Java `new Integer(42)` form
// (an `object_creation_expression`).
func checkUseValueOfJavaNew(ctx *api.Context, r *UseValueOfRule, file *scanner.File, idx uint32) {
	// Locate the type identifier and reject anonymous-class forms
	// (`new Integer(42) { ... }`) by skipping any node with a class_body child.
	if body, _ := file.FlatFindChild(idx, "class_body"); body != 0 {
		return
	}
	var typeNode uint32
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier":
			typeNode = child
		case "scoped_type_identifier", "scoped_identifier":
			typeNode = child
		case "generic_type":
			return
		}
		if typeNode != 0 {
			break
		}
	}
	if typeNode == 0 {
		return
	}
	typeName, ok := boxedConstructorName(file.FlatNodeText(typeNode))
	if !ok {
		return
	}
	args, _ := file.FlatFindChild(idx, "argument_list")
	if args == 0 {
		return
	}
	argCount := 0
	var firstArg uint32
	for a := file.FlatFirstChild(args); a != 0; a = file.FlatNextSib(a) {
		if !file.FlatIsNamed(a) {
			continue
		}
		argCount++
		if firstArg == 0 {
			firstArg = a
		}
	}
	if argCount != 1 {
		return
	}
	argText := file.FlatNodeText(firstArg)
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, useValueOfMessage(typeName, argText))
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: typeName + ".valueOf(" + argText + ")",
	}
	ctx.Emit(f)
}

// =====================================================================
// 5. LogTagLengthRule (LogDetector.LONG_TAG)
// =====================================================================

// LogTagLengthRule flags Log.x() calls where the tag string literal exceeds
// 23 characters (the maximum enforced by older Android versions).
type LogTagLengthRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LogTagLengthRule) Confidence() float64 { return api.ConfidenceHigher }

// resolveLogTagStringValue resolves the string value of a Log tag
// argument expression. It handles direct string literals as well as
// references to a `const val` / `val` declaration whose initializer
// is a non-interpolated string literal. Returns ("", false) when the
// value cannot be resolved.
func resolveLogTagStringValue(file *scanner.File, expr uint32) (string, bool) {
	if file == nil || expr == 0 {
		return "", false
	}
	expr = flatUnwrapParenExpr(file, expr)
	if file.FlatType(expr) == "string_literal" {
		if flatContainsStringInterpolation(file, expr) {
			return "", false
		}
		return stringLiteralContent(file, expr), true
	}
	name := flatReferenceSimpleName(file, expr)
	if name == "" {
		return "", false
	}
	return findConstStringPropertyValue(file, name)
}

// findConstStringPropertyValue scans every property_declaration in the
// file for one named `name` whose initializer is a non-interpolated
// string literal, and returns its content.
func findConstStringPropertyValue(file *scanner.File, name string) (string, bool) {
	if file == nil || name == "" {
		return "", false
	}
	value := ""
	found := false
	file.FlatWalkNodes(0, "property_declaration", func(node uint32) {
		if found {
			return
		}
		if propertyDeclarationName(file, node) != name {
			return
		}
		init := propertyInitializerExpression(file, node)
		if init == 0 || file.FlatType(init) != "string_literal" {
			return
		}
		if flatContainsStringInterpolation(file, init) {
			return
		}
		value = stringLiteralContent(file, init)
		found = true
	})
	return value, found
}

// =====================================================================
// 6. LogTagMismatchRule (LogDetector.WRONG_TAG)
// =====================================================================

// LogTagMismatchRule flags Log.x(TAG, ...) calls where the TAG companion
// constant value doesn't match the enclosing class name.
type LogTagMismatchRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LogTagMismatchRule) Confidence() float64 { return api.ConfidenceMedium }

// findDirectCompanionTag searches the direct children of a class_declaration
// for a companion_object containing a TAG constant, returning its string value.
// This avoids matching TAG constants defined in nested classes.
func findDirectCompanionTagFlat(file *scanner.File, classNode uint32) string {
	for i := 0; i < file.FlatChildCount(classNode); i++ {
		child := file.FlatChild(classNode, i)
		if file.FlatType(child) == "class_body" {
			for j := 0; j < file.FlatChildCount(child); j++ {
				member := file.FlatChild(child, j)
				if file.FlatType(member) == "companion_object" {
					if tag := directCompanionTagValueFlat(file, member); tag != "" {
						return tag
					}
				}
			}
			break
		}
	}
	return ""
}

func directCompanionTagValueFlat(file *scanner.File, companion uint32) string {
	tag := ""
	file.FlatWalkNodes(companion, "property_declaration", func(prop uint32) {
		if tag != "" || extractIdentifierFlat(file, prop) != "TAG" {
			return
		}
		if !nodeIsDirectlyInsideFlat(file, prop, companion) {
			return
		}
		expr := propertyInitializerExpression(file, prop)
		if expr == 0 {
			return
		}
		switch file.FlatType(expr) {
		case "string_literal":
			if flatContainsStringInterpolation(file, expr) {
				return
			}
			tag = stringLiteralContent(file, expr)
		case "navigation_expression":
			// `Foo::class.java.simpleName` — treat the referenced class
			// name as the effective tag value so it can be compared to
			// the enclosing class.
			if name := classLiteralSimpleNameRef(file, expr); name != "" {
				tag = name
			}
		}
	})
	return tag
}

// classLiteralSimpleNameRef extracts `Foo` from an expression of the form
// `Foo::class.java.simpleName` (or the Kotlin-only `Foo::class.simpleName`).
// Returns "" if the expression is anything else.
func classLiteralSimpleNameRef(file *scanner.File, expr uint32) string {
	if file.FlatType(expr) != "navigation_expression" {
		return ""
	}
	if flatNavigationExpressionLastIdentifier(file, expr) != "simpleName" {
		return ""
	}
	var ref uint32
	file.FlatWalkNodes(expr, "callable_reference", func(n uint32) {
		if ref == 0 {
			ref = n
		}
	})
	if ref == 0 {
		return ""
	}
	for c := file.FlatFirstChild(ref); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "type_identifier", "simple_identifier":
			return file.FlatNodeText(c)
		}
	}
	return ""
}

func nodeIsDirectlyInsideFlat(file *scanner.File, node uint32, container uint32) bool {
	for cur, ok := file.FlatParent(node); ok; cur, ok = file.FlatParent(cur) {
		if cur == container {
			return true
		}
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration", "function_declaration", "lambda_literal":
			return false
		}
	}
	return false
}

// =====================================================================
// 7. NonInternationalizedSmsRule
// =====================================================================

// NonInternationalizedSmsRule flags SmsManager.sendTextMessage() calls that
// may not handle internationalization of phone numbers or message encoding.
type NonInternationalizedSmsRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *NonInternationalizedSmsRule) Confidence() float64 { return api.ConfidenceMedium }

func nonInternationalizedSmsCallFlat(file *scanner.File, call uint32) bool {
	name := flatCallExpressionName(file, call)
	if name != "sendTextMessage" && name != "sendMultipartTextMessage" {
		return false
	}
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return false
	}
	receiver := file.FlatNamedChild(navExpr, 0)
	if !smsManagerReceiverFlat(file, receiver, call) {
		return false
	}
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return false
	}
	arg := flatPositionalValueArgument(file, args, 0)
	if arg == 0 {
		return false
	}
	expr := flatUnwrapParenExpr(file, flatValueArgumentExpression(file, arg))
	if expr == 0 || file.FlatType(expr) != "string_literal" {
		return false
	}
	if flatContainsStringInterpolation(file, expr) {
		return false
	}
	return !strings.HasPrefix(stringLiteralContent(file, expr), "+")
}

func smsManagerReceiverFlat(file *scanner.File, receiver uint32, call uint32) bool {
	if path := contactsIdentifierPathFlat(file, receiver); len(path) > 0 && path[0] == "SmsManager" {
		return true
	}
	name := flatReferenceSimpleName(file, receiver)
	if name == "" {
		return false
	}
	if name == "smsManager" {
		return true
	}
	return contactsSameOwnerDeclarationHasTypeFlat(file, call, name, "SmsManager")
}

// =====================================================================
// 8. ServiceCastRule
// =====================================================================

// ServiceCastRule flags getSystemService() calls cast to the wrong type.
// For example, getSystemService(ALARM_SERVICE) as PowerManager.
type ServiceCastRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ServiceCastRule) Confidence() float64 { return api.ConfidenceMedium }

// serviceCastMap maps service constant names to their manager types, as in Lint's
// ServiceCastDetector (package names stripped).
//
//nolint:gosec // G101 false positive: system service names, not credentials.
var serviceCastMap = map[string]string{
	"ACCESSIBILITY_SERVICE":            "AccessibilityManager",
	"ACCOUNT_SERVICE":                  "AccountManager",
	"ACTIVITY_SERVICE":                 "ActivityManager",
	"ALARM_SERVICE":                    "AlarmManager",
	"APPWIDGET_SERVICE":                "AppWidgetManager",
	"APP_OPS_SERVICE":                  "AppOpsManager",
	"APP_SEARCH_SERVICE":               "AppSearchManager",
	"AUDIO_SERVICE":                    "AudioManager",
	"BATTERY_SERVICE":                  "BatteryManager",
	"BIOMETRIC_SERVICE":                "BiometricManager",
	"BLOB_STORE_SERVICE":               "BlobStoreManager",
	"BLUETOOTH_SERVICE":                "BluetoothManager",
	"CAMERA_SERVICE":                   "CameraManager",
	"CAPTIONING_SERVICE":               "CaptioningManager",
	"CARRIER_CONFIG_SERVICE":           "CarrierConfigManager",
	"CLIPBOARD_SERVICE":                "ClipboardManager",
	"COMPANION_DEVICE_SERVICE":         "CompanionDeviceManager",
	"CONNECTIVITY_DIAGNOSTICS_SERVICE": "ConnectivityDiagnosticsManager",
	"CONNECTIVITY_SERVICE":             "ConnectivityManager",
	"CONSUMER_IR_SERVICE":              "ConsumerIrManager",
	"CONTACT_KEYS_SERVICE":             "E2eeContactKeysManager",
	"CREDENTIAL_SERVICE":               "CredentialManager",
	"CROSS_PROFILE_APPS_SERVICE":       "CrossProfileApps",
	"DEVICE_LOCK_SERVICE":              "DeviceLockManager",
	"DEVICE_POLICY_SERVICE":            "DevicePolicyManager",
	"DISPLAY_HASH_SERVICE":             "DisplayHashManager",
	"DISPLAY_SERVICE":                  "DisplayManager",
	"DOMAIN_VERIFICATION_SERVICE":      "DomainVerificationManager",
	"DOWNLOAD_SERVICE":                 "DownloadManager",
	"DROPBOX_SERVICE":                  "DropBoxManager",
	"EUICC_SERVICE":                    "EuiccManager",
	"FILE_INTEGRITY_SERVICE":           "FileIntegrityManager",
	"FINGERPRINT_SERVICE":              "FingerprintManager",
	"GAME_SERVICE":                     "GameManager",
	"GRAMMATICAL_INFLECTION_SERVICE":   "GrammaticalInflectionManager",
	"HARDWARE_PROPERTIES_SERVICE":      "HardwarePropertiesManager",
	"HEALTHCONNECT_SERVICE":            "HealthConnectManager",
	"INPUT_METHOD_SERVICE":             "InputMethodManager",
	"INPUT_SERVICE":                    "InputManager",
	"IPSEC_SERVICE":                    "IpSecManager",
	"JOB_SCHEDULER_SERVICE":            "JobScheduler",
	"KEYGUARD_SERVICE":                 "KeyguardManager",
	"LAUNCHER_APPS_SERVICE":            "LauncherApps",
	"LAYOUT_INFLATER_SERVICE":          "LayoutInflater",
	"LOCALE_SERVICE":                   "LocaleManager",
	"LOCATION_SERVICE":                 "LocationManager",
	"MEDIA_COMMUNICATION_SERVICE":      "MediaCommunicationManager",
	"MEDIA_METRICS_SERVICE":            "MediaMetricsManager",
	"MEDIA_PROJECTION_SERVICE":         "MediaProjectionManager",
	"MEDIA_ROUTER_SERVICE":             "MediaRouter",
	"MEDIA_SESSION_SERVICE":            "MediaSessionManager",
	"MIDI_SERVICE":                     "MidiManager",
	"NETWORK_STATS_SERVICE":            "NetworkStatsManager",
	"NFC_SERVICE":                      "NfcManager",
	"NOTIFICATION_SERVICE":             "NotificationManager",
	"NSD_SERVICE":                      "NsdManager",
	"OVERLAY_SERVICE":                  "OverlayManager",
	"PEOPLE_SERVICE":                   "PeopleManager",
	"PERFORMANCE_HINT_SERVICE":         "PerformanceHintManager",
	"PERSISTENT_DATA_BLOCK_SERVICE":    "PersistentDataBlockManager",
	"POWER_SERVICE":                    "PowerManager",
	"PRINT_SERVICE":                    "PrintManager",
	"PROFILING_SERVICE":                "ProfilingManager",
	"RESTRICTIONS_SERVICE":             "RestrictionsManager",
	"ROLE_SERVICE":                     "RoleManager",
	"SEARCH_SERVICE":                   "SearchManager",
	"SECURITY_STATE_SERVICE":           "SecurityStateManager",
	"SENSOR_SERVICE":                   "SensorManager",
	"SHORTCUT_SERVICE":                 "ShortcutManager",
	"STATUS_BAR_SERVICE":               "StatusBarManager",
	"STORAGE_SERVICE":                  "StorageManager",
	"STORAGE_STATS_SERVICE":            "StorageStatsManager",
	"SYSTEM_HEALTH_SERVICE":            "SystemHealthManager",
	"TELECOM_SERVICE":                  "TelecomManager",
	"TELEPHONY_IMS_SERVICE":            "ImsManager",
	"TELEPHONY_SERVICE":                "TelephonyManager",
	"TELEPHONY_SUBSCRIPTION_SERVICE":   "SubscriptionManager",
	"TEXT_CLASSIFICATION_SERVICE":      "TextClassificationManager",
	"TEXT_SERVICES_MANAGER_SERVICE":    "TextServicesManager",
	"TV_AD_SERVICE":                    "TvAdManager",
	"TV_INPUT_SERVICE":                 "TvInputManager",
	"TV_INTERACTIVE_APP_SERVICE":       "TvInteractiveAppManager",
	"UI_MODE_SERVICE":                  "UiModeManager",
	"USAGE_STATS_SERVICE":              "UsageStatsManager",
	"USB_SERVICE":                      "UsbManager",
	"USER_SERVICE":                     "UserManager",
	"VIBRATOR_MANAGER_SERVICE":         "VibratorManager",
	"VIBRATOR_SERVICE":                 "Vibrator",
	"VIRTUAL_DEVICE_SERVICE":           "VirtualDeviceManager",
	"VPN_MANAGEMENT_SERVICE":           "VpnManager",
	"WALLPAPER_SERVICE":                "WallpaperManager",
	"WIFI_AWARE_SERVICE":               "WifiAwareManager",
	"WIFI_P2P_SERVICE":                 "WifiP2pManager",
	"WIFI_RTT_RANGING_SERVICE":         "WifiRttManager",
	"WIFI_SERVICE":                     "WifiManager",
	"WINDOW_SERVICE":                   "WindowManager",
}

// =====================================================================
// 9. ToastRule (ShowToast)
// =====================================================================

// ToastRule flags Toast.makeText() calls where .show() is never called
// on the result. The Toast will not be displayed without show().
type ToastRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ToastRule) Confidence() float64 { return api.ConfidenceHigh }

func toastMakeTextIsShown(file *scanner.File, call uint32) bool {
	if ancestorCallNameMatches(file, call, "show") {
		return true
	}
	if ancestorScopeLambdaFinalizes(file, call, showCallName, nil) {
		return true
	}
	if toastResultEscapes(file, call) {
		return true
	}
	toastVar := initializerAssignedName(file, call)
	if toastVar == "" {
		return false
	}
	if fn, ok := flatEnclosingFunction(file, call); ok {
		return functionHasReceiverCallAfter(file, fn, call, toastVar, showCallName, nil) ||
			localVarFinalized(file, fn, call, toastVar, showCallName, nil, 0)
	}
	if cls, ok := flatEnclosingAncestor(file, call, "class_declaration", "object_declaration"); ok {
		return classHasReceiverShowCall(file, cls, toastVar)
	}
	return false
}

// toastResultEscapes reports whether the Toast returned by makeText is handed
// to the caller (returned, passed as an argument, assigned, or used as an
// expression body), so the caller is responsible for showing it.
func toastResultEscapes(file *scanner.File, call uint32) bool {
	node := call
	for {
		parent, ok := file.FlatParent(node)
		if !ok {
			return false
		}
		switch file.FlatType(parent) {
		case "parenthesized_expression", "as_expression", "navigation_expression":
			if file.FlatType(parent) == "navigation_expression" && file.FlatFirstChild(parent) != node {
				return false
			}
			node = parent
		case "jump_expression", "value_argument", "assignment", "function_body":
			return file.FlatType(parent) != "jump_expression" || strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(parent)), "return")
		default:
			return false
		}
	}
}

func classHasReceiverShowCall(file *scanner.File, cls uint32, receiverName string) bool {
	found := false
	file.FlatWalkNodes(cls, "call_expression", func(call uint32) {
		if found || flatCallExpressionName(file, call) != "show" {
			return
		}
		if flatReceiverNameFromCall(file, call) == receiverName {
			found = true
		}
	})
	return found
}
