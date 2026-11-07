package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

type UseArrayLiteralsInAnnotationsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseArrayLiteralsInAnnotationsRule) Confidence() float64 { return api.ConfidenceMedium }

type UseSumOfInsteadOfFlatMapSizeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseSumOfInsteadOfFlatMapSizeRule) Confidence() float64 { return api.ConfidenceMedium }

var sumOfSourceCalls = map[string]bool{"flatMap": true, "flatten": true, "map": true}

func sumOfNavSelectorFlat(file *scanner.File, nav uint32) string {
	for i := file.FlatChildCount(nav) - 1; i >= 0; i-- {
		child := file.FlatChild(nav, i)
		if file.FlatType(child) == "simple_identifier" {
			return file.FlatNodeText(child)
		}
		if file.FlatType(child) == "navigation_suffix" {
			for j := 0; j < file.FlatChildCount(child); j++ {
				gc := file.FlatChild(child, j)
				if file.FlatType(gc) == "simple_identifier" {
					return file.FlatNodeText(gc)
				}
			}
		}
	}
	return ""
}

type UseLetRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseLetRule) Confidence() float64 { return api.ConfidenceMedium }

type UseDataClassRule struct {
	FlatDispatchBase
	BaseRule
	AllowVars bool
	// ExcludeAnnotatedClasses are detekt-style globs of annotations that exempt a class.
	ExcludeAnnotatedClasses []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseDataClassRule) Confidence() float64 { return api.ConfidenceMedium }

// useDataClassDefaultFunctions are the functions a data class generates, which detekt allows in the body.
var useDataClassDefaultFunctions = map[string]bool{"equals": true, "hashCode": true, "toString": true, "copy": true}

// useDataClassInterfacesInFile lists the interfaces declared at the top level of the file.
func useDataClassInterfacesInFile(file *scanner.File) map[string]bool {
	interfaces := map[string]bool{}
	for top := file.FlatFirstChild(0); top != 0; top = file.FlatNextSib(top) {
		if file.FlatType(top) == "class_declaration" && file.FlatHasChildOfType(top, "interface") {
			interfaces[extractIdentifierFlat(file, top)] = true
		}
	}
	return interfaces
}

// useDataClassSupertypesAreSimple reports that every supertype is an interface of the same file that is not delegated to.
func useDataClassSupertypesAreSimple(file *scanner.File, class uint32) bool {
	var interfaces map[string]bool
	for c := file.FlatFirstChild(class); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) != "delegation_specifier" {
			continue
		}
		if interfaces == nil {
			interfaces = useDataClassInterfacesInFile(file)
		}
		if strings.Contains(file.FlatNodeText(c), " by") {
			return false
		}
		userType, ok := file.FlatFindChild(c, "user_type")
		if !ok || userType == 0 {
			return false
		}
		name := ""
		for t := file.FlatFirstChild(userType); t != 0; t = file.FlatNextSib(t) {
			if file.FlatType(t) == "type_identifier" {
				name = file.FlatNodeText(t)
			}
		}
		if !interfaces[name] {
			return false
		}
	}
	return true
}

func (r *UseDataClassRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	for _, m := range []string{"data", "enum", "annotation", "sealed", "inline", "value", "inner", "expect", "abstract", "open"} {
		if file.FlatHasModifier(idx, m) {
			return
		}
	}
	if file.FlatHasChildOfType(idx, "interface") || file.FlatHasChildOfType(idx, "enum") {
		return
	}
	// Only private constructors (or none at all): a data class would expose them through copy().
	ctor, _ := file.FlatFindChild(idx, "primary_constructor")
	if useDataClassOnlyPrivateConstructors(file, idx, ctor) {
		return
	}
	if !useDataClassSupertypesAreSimple(file, idx) {
		return
	}
	if len(r.ExcludeAnnotatedClasses) > 0 {
		if mods, ok := file.FlatFindChild(idx, "modifiers"); ok && mods != 0 {
			if annotatedParamMatches(file, longParamAnnotationNames(file, mods), compileAnnotationPatterns(r.ExcludeAnnotatedClasses)) {
				return
			}
		}
	}
	ctorMember, ctorVar, ok := useDataClassConstructorProperties(file, ctor)
	if !ok {
		return
	}
	bodyMember, bodyVar, ok := useDataClassBodyProperties(file, idx)
	if !ok {
		return
	}
	hasMember, hasVar := ctorMember || bodyMember, ctorVar || bodyVar
	if !hasMember || (r.AllowVars && hasVar) {
		return
	}
	ctx.EmitAt(declLine(file, idx), declCol(file, idx),
		"The class "+extractIdentifierFlat(file, idx)+" defines no functionality and only holds data. Consider converting it to a data class.")
}

// useDataClassOnlyPrivateConstructors reports a class whose primary constructor is missing or private and whose
// secondary constructors are all private.
func useDataClassOnlyPrivateConstructors(file *scanner.File, idx, ctor uint32) bool {
	if ctor != 0 && !file.FlatHasModifier(ctor, "private") {
		return false
	}
	onlyPrivate := true
	if body, _ := file.FlatFindChild(idx, "class_body"); body != 0 {
		forEachDirectClassMemberFlat(file, body, func(m uint32) {
			if file.FlatType(m) == "secondary_constructor" && !file.FlatHasModifier(m, "private") {
				onlyPrivate = false
			}
		})
	}
	return onlyPrivate
}

// useDataClassConstructorProperties reports whether the primary constructor declares val or var properties; ok is
// false when a parameter is not a property.
func useDataClassConstructorProperties(file *scanner.File, ctor uint32) (hasMember, hasVar, ok bool) {
	if ctor == 0 {
		return false, false, true
	}
	nonProperty := false
	file.FlatWalkNodes(ctor, "class_parameter", func(p uint32) {
		binding, _ := file.FlatFindChild(p, "binding_pattern_kind")
		switch {
		case binding != 0 && file.FlatHasChildOfType(binding, "val"):
			hasMember = true
		case binding != 0 && file.FlatHasChildOfType(binding, "var"):
			hasMember, hasVar = true, true
		default:
			nonProperty = true
		}
	})
	return hasMember, hasVar, !nonProperty
}

// useDataClassBodyProperties reports whether the class body declares properties and var properties; ok is false
// when the body has a delegated property or a function other than the data class defaults.
func useDataClassBodyProperties(file *scanner.File, idx uint32) (hasMember, hasVar, ok bool) {
	body, _ := file.FlatFindChild(idx, "class_body")
	if body == 0 {
		return false, false, true
	}
	blocked := false
	forEachDirectClassMemberFlat(file, body, func(m uint32) {
		switch file.FlatType(m) {
		case "function_declaration":
			if !useDataClassDefaultFunctions[extractIdentifierFlat(file, m)] {
				blocked = true
			}
		case "property_declaration":
			hasMember = true
			if file.FlatHasChildOfType(m, "property_delegate") {
				blocked = true
			}
			if binding, ok := file.FlatFindChild(m, "binding_pattern_kind"); ok && binding != 0 && file.FlatHasChildOfType(binding, "var") {
				hasVar = true
			}
		}
	})
	return hasMember, hasVar, !blocked
}

type UseIfInsteadOfWhenRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreWhenContainingVariableDeclaration bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseIfInsteadOfWhenRule) Confidence() float64 { return api.ConfidenceMedium }

type UseIfEmptyOrIfBlankRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *UseIfEmptyOrIfBlankRule) Confidence() float64 { return api.ConfidenceMedium }

var ifEmptyOrBlankMethods = map[string]struct {
	replacement string
	negated     bool
}{"isEmpty": {"ifEmpty", false}, "isBlank": {"ifBlank", false}, "isNotEmpty": {"ifEmpty", true}, "isNotBlank": {"ifBlank", true}}

type ExplicitCollectionElementAccessMethodRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *ExplicitCollectionElementAccessMethodRule) Confidence() float64 { return api.ConfidenceMedium }

func explicitCollectionAccessReceiverSupported(ctx *api.Context, receiver uint32, method string) bool {
	if ctx.File == nil || receiver == 0 {
		return false
	}
	if typ, ok := semantics.ExpressionType(ctx, receiver); ok && explicitCollectionAccessResolvedTypeSupports(typ, method) {
		return true
	}
	if typ, _, ok := flatNullOrEmptyExplicitReceiverType(ctx.File, receiver); ok && explicitCollectionAccessTypeSupports(ctx.File, typ, method) {
		return true
	}
	return explicitCollectionAccessInitializerSupports(ctx.File, receiver, method)
}

func explicitCollectionAccessResolvedTypeSupports(typ semantics.TypeInfo, method string) bool {
	if explicitCollectionAccessTypeNameSupports(typ.Name, method) || explicitCollectionAccessTypeNameSupports(typ.FQN, method) {
		return true
	}
	if typ.Type == nil {
		return false
	}
	for _, name := range explicitCollectionAccessSupportedTypes(method) {
		if typ.Type.IsSubtypeOf(name) {
			return true
		}
	}
	return false
}

func explicitCollectionAccessTypeSupports(file *scanner.File, typ string, method string) bool {
	name := explicitCollectionAccessSimpleTypeName(typ)
	if explicitCollectionAccessTypeNameSupports(name, method) || explicitCollectionAccessTypeNameSupports(typ, method) {
		return true
	}
	for _, fqn := range explicitCollectionAccessImportedTypes(name, method) {
		if fileImportsFQN(file, fqn) {
			return true
		}
	}
	return false
}

func explicitCollectionAccessTypeNameSupports(typ string, method string) bool {
	name := explicitCollectionAccessSimpleTypeName(typ)
	for _, supported := range explicitCollectionAccessSupportedTypes(method) {
		if name == explicitCollectionAccessSimpleTypeName(supported) || typ == supported {
			return true
		}
	}
	return false
}

func explicitCollectionAccessSupportedTypes(method string) []string {
	getTypes := []string{
		"kotlin.String", "String",
		"kotlin.Array", "Array",
		"kotlin.collections.List", "List",
		"kotlin.collections.MutableList", "MutableList",
		"kotlin.collections.Map", "Map",
		"kotlin.collections.MutableMap", "MutableMap",
		"java.util.List", "java.util.Map",
		"ByteArray", "ShortArray", "IntArray", "LongArray", "FloatArray", "DoubleArray", "BooleanArray", "CharArray",
		"ImmutableList", "PersistentList", "ImmutableMap", "PersistentMap",
	}
	if method == "put" {
		return []string{
			"kotlin.collections.MutableMap", "MutableMap", "java.util.Map", "HashMap", "LinkedHashMap",
			"java.util.HashMap", "java.util.LinkedHashMap", "PersistentMap",
		}
	}
	if method == "set" {
		return []string{
			"kotlin.Array", "Array",
			"kotlin.collections.MutableList", "MutableList",
			"kotlin.collections.MutableMap", "MutableMap",
			"java.util.List", "java.util.Map",
			"ByteArray", "ShortArray", "IntArray", "LongArray", "FloatArray", "DoubleArray", "BooleanArray", "CharArray",
		}
	}
	return getTypes
}

func explicitCollectionAccessImportedTypes(simple string, method string) []string {
	if method != "get" {
		return nil
	}
	switch simple {
	case "ImmutableList":
		return []string{"kotlinx.collections.immutable.ImmutableList"}
	case "PersistentList":
		return []string{"kotlinx.collections.immutable.PersistentList"}
	case "ImmutableMap":
		return []string{"kotlinx.collections.immutable.ImmutableMap"}
	case "PersistentMap":
		return []string{"kotlinx.collections.immutable.PersistentMap"}
	}
	return nil
}

func explicitCollectionAccessSimpleTypeName(typ string) string {
	typ = strings.TrimSpace(typ)
	typ = strings.TrimSuffix(typ, "?")
	if idx := strings.Index(typ, "<"); idx >= 0 {
		typ = typ[:idx]
	}
	if idx := strings.LastIndex(typ, "."); idx >= 0 {
		typ = typ[idx+1:]
	}
	return strings.TrimSpace(typ)
}

func explicitCollectionAccessInitializerSupports(file *scanner.File, receiver uint32, method string) bool {
	path := flatNullOrEmptyReferencePath(file, receiver)
	if len(path) != 1 {
		return false
	}
	name := path[0]
	var supported bool
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if supported || file.FlatType(candidate) != "property_declaration" || flatNullOrEmptyDeclarationName(file, candidate) != name {
			return
		}
		if !semantics.SameEnclosingOwner(file, candidate, receiver) {
			return
		}
		firstCall := uint32(0)
		file.FlatWalkNodes(candidate, "call_expression", func(call uint32) {
			if firstCall == 0 {
				firstCall = call
			}
		})
		supported = explicitCollectionAccessFactorySupports(flatCallExpressionName(file, firstCall), method)
	})
	return supported
}

func explicitCollectionAccessFactorySupports(name string, method string) bool {
	if method == "put" {
		return name == "mutableMapOf" || name == "hashMapOf" || name == "linkedMapOf"
	}
	switch name {
	case "listOf", "mapOf", "arrayOf":
		return method == "get"
	case "mutableListOf", "mutableMapOf", "hashMapOf", "linkedMapOf",
		"byteArrayOf", "shortArrayOf", "intArrayOf", "longArrayOf", "floatArrayOf", "doubleArrayOf", "booleanArrayOf", "charArrayOf":
		return true
	}
	return false
}

type AlsoCouldBeApplyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *AlsoCouldBeApplyRule) Confidence() float64 { return api.ConfidenceMedium }

type EqualsNullCallRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule for collection and data-flow idioms. The detection
// is pattern-based and the suggested replacement's readability is a style
// preference. Classified per roadmap/17.
func (r *EqualsNullCallRule) Confidence() float64 { return api.ConfidenceMedium }

// useLetIsNullBlock mirrors detekt: a lone `null` or a block whose only statement is `null`.
func useLetIsNullBlock(file *scanner.File, body uint32) bool {
	if body == 0 {
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(body))
	if strings.HasPrefix(text, "{") {
		stmts, ok := file.FlatFindChild(body, "statements")
		if !ok || stmts == 0 {
			return false
		}
		// `null` is an anonymous token in the grammar, so count every child except separators and comments.
		var only uint32
		count := 0
		for c := file.FlatFirstChild(stmts); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == ";" || isFlatCommentNode(file, c) {
				continue
			}
			only = c
			count++
		}
		if count != 1 {
			return false
		}
		text = strings.TrimSpace(file.FlatNodeText(only))
		for strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
			text = strings.TrimSpace(text[1 : len(text)-1])
		}
	}
	return text == "null"
}

// useLetReportable reports `if (x == null) null else ...` and `if (x != null) ... else null`, which ?.let {} replaces.
func useLetReportable(file *scanner.File, ifIdx uint32) bool {
	cond, _ := flatThrowExtractCondAndBody(file, ifIdx)
	if cond == 0 || file.FlatType(cond) != "equality_expression" || file.FlatChildCount(cond) != 3 {
		return false
	}
	lhs, op, rhs := file.FlatChild(cond, 0), strings.TrimSpace(file.FlatNodeText(file.FlatChild(cond, 1))), file.FlatChild(cond, 2)
	if strings.TrimSpace(file.FlatNodeText(lhs)) != "null" && strings.TrimSpace(file.FlatNodeText(rhs)) != "null" {
		return false
	}
	thenBody, elseBody := ifRawBodies(file, ifIdx)
	switch op {
	case "==":
		return useLetIsNullBlock(file, thenBody)
	case "!=":
		return useLetIsNullBlock(file, elseBody)
	}
	return false
}

// callIsStatement reports whether the call is a statement of a block or a control-structure body.
func callIsStatement(file *scanner.File, idx uint32) bool {
	p, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	switch file.FlatType(p) {
	case "statements", "control_structure_body":
		return true
	}
	return false
}

// equalsCallNeedsParens reports whether replacing the call with `a == null` could change parsing.
func equalsCallNeedsParens(file *scanner.File, idx uint32) bool {
	p, ok := file.FlatParent(idx)
	if !ok {
		return true
	}
	switch file.FlatType(p) {
	case "statements", "control_structure_body", "parenthesized_expression", "value_argument",
		"property_declaration", "assignment", "jump_expression", "function_body", "if_expression", "while_statement":
		return false
	}
	return true
}
