package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// NullCheckOnMutablePropertyRule detects null check on var property.
// ---------------------------------------------------------------------------
type NullCheckOnMutablePropertyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — distinguishing
// val vs var requires type resolution of the receiver. Heuristic fallback
// uses declaration patterns. Classified per roadmap/17.
func (r *NullCheckOnMutablePropertyRule) Confidence() float64 { return api.ConfidenceMedium }

// check follows detekt: an `if` whose condition holds `name != null` for a mutable property reports when the
// property is also read inside that `if` (the smart cast the check hopes for does not hold).
func (r *NullCheckOnMutablePropertyRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	cond := ifExpressionCondition(file, idx)
	if cond == 0 {
		return
	}
	reported := map[string]bool{}
	for _, name := range nonNullCheckedNames(file, cond, nil) {
		if reported[name] {
			continue
		}
		fq, ok := mutablePropertyFqName(file, idx, name)
		if !ok || !ifReadsPropertyOutsideNullChecks(file, idx, idx, name) {
			continue
		}
		reported[name] = true
		ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			fmt.Sprintf("Null-check is being called on mutable property '%s'.", fq)))
	}
}

func ifExpressionCondition(file *scanner.File, ifExpr uint32) uint32 {
	open := false
	for c := file.FlatFirstChild(ifExpr); c != 0; c = file.FlatNextSib(c) {
		switch {
		case file.FlatType(c) == "(":
			open = true
		case open && file.FlatIsNamed(c):
			return c
		}
	}
	return 0
}

// isBinaryExpressionNode matches PSI's KtBinaryExpression: every infix operator form except casts and `is`.
func isBinaryExpressionNode(file *scanner.File, n uint32) bool {
	switch file.FlatType(n) {
	case "equality_expression", "conjunction_expression", "disjunction_expression", "comparison_expression",
		"additive_expression", "multiplicative_expression", "elvis_expression", "infix_expression", "range_expression":
		return true
	}
	return false
}

// nonNullCheckName returns the name checked by `name != null` or `null != name` on a bare reference.
func nonNullCheckName(file *scanner.File, n uint32) (string, bool) {
	if file.FlatType(n) != "equality_expression" || file.FlatChildCount(n) != 3 || file.FlatType(file.FlatChild(n, 1)) != "!=" {
		return "", false
	}
	left, right := file.FlatChild(n, 0), file.FlatChild(n, 2)
	if file.FlatType(left) == "null" && file.FlatType(right) == "simple_identifier" {
		return file.FlatNodeText(right), true
	}
	if file.FlatType(right) == "null" && file.FlatType(left) == "simple_identifier" {
		return file.FlatNodeText(left), true
	}
	return "", false
}

func nonNullCheckedNames(file *scanner.File, n uint32, into []string) []string {
	if name, ok := nonNullCheckName(file, n); ok {
		return append(into, name)
	}
	if !isBinaryExpressionNode(file, n) {
		return into
	}
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		if isBinaryExpressionNode(file, c) {
			into = nonNullCheckedNames(file, c, into)
		}
	}
	return into
}

// isNullComparisonOperand reports a bare reference that is the operand of `== null` or `!= null`.
func isNullComparisonOperand(file *scanner.File, ref uint32) bool {
	parent, ok := file.FlatParent(ref)
	if !ok || file.FlatType(parent) != "equality_expression" || file.FlatChildCount(parent) != 3 {
		return false
	}
	op := file.FlatType(file.FlatChild(parent, 1))
	if op != "!=" && op != "==" {
		return false
	}
	return file.FlatType(file.FlatChild(parent, 0)) == "null" || file.FlatType(file.FlatChild(parent, 2)) == "null"
}

// ifReadsPropertyOutsideNullChecks reports a read of name inside ifExpr that is not a null comparison and does
// not belong to a nested `if` that checks the same name itself.
func ifReadsPropertyOutsideNullChecks(file *scanner.File, owner, n uint32, name string) bool {
	if n != owner && file.FlatType(n) == "if_expression" {
		if c := ifExpressionCondition(file, n); c != 0 {
			for _, checked := range nonNullCheckedNames(file, c, nil) {
				if checked == name {
					return false
				}
			}
		}
	}
	if file.FlatType(n) == "simple_identifier" && file.FlatNodeText(n) == name && !isNullComparisonOperand(file, n) {
		if parent, ok := file.FlatParent(n); ok {
			switch file.FlatType(parent) {
			case "navigation_suffix", "variable_declaration", "parameter", "class_parameter":
				// A selector or a declaration is not a read of this property by its simple name.
				return false
			case "value_argument":
				if next := file.FlatNextSib(n); next != 0 && file.FlatType(next) == "=" {
					return false
				}
			}
		}
		return true
	}
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		if ifReadsPropertyOutsideNullChecks(file, owner, c, name) {
			return true
		}
	}
	return false
}

// mutablePropertyFqName resolves a bare name at ref to a mutable property (a `var`, a `var` constructor
// property, or a property with a custom getter) declared in this file, returning its qualified name.
func mutablePropertyFqName(file *scanner.File, ref uint32, name string) (string, bool) {
	if localBindingShadows(file, ref, name) {
		return "", false
	}
	for p, ok := file.FlatParent(ref); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "class_declaration", "object_declaration":
			if mutable, found := classMemberProperty(file, p, name); found {
				if !mutable {
					return "", false
				}
				return qualifiedPropertyName(file, p, name), true
			}
		case "source_file":
			for c := file.FlatFirstChild(p); c != 0; c = file.FlatNextSib(c) {
				if file.FlatType(c) == "property_declaration" && propertyDeclarationName(file, c) == name {
					if propertyDeclarationIsVar(file, c) || propertyHasGetter(file, c) {
						return qualifiedPropertyName(file, 0, name), true
					}
					return "", false
				}
			}
		}
	}
	return "", false
}

func propertyHasGetter(file *scanner.File, prop uint32) bool {
	_, ok := file.FlatFindChild(prop, "getter")
	return ok
}

// classMemberProperty looks for a property declared by the class: a constructor property, a body property or a
// companion property. mutable is true for `var` and for custom-getter properties.
func classMemberProperty(file *scanner.File, class uint32, name string) (mutable, found bool) {
	if ctor, ok := file.FlatFindChild(class, "primary_constructor"); ok {
		for c := file.FlatFirstChild(ctor); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) != "class_parameter" {
				continue
			}
			if id, ok := file.FlatFindChild(c, "simple_identifier"); !ok || file.FlatNodeText(id) != name {
				continue
			}
			if kind, ok := file.FlatFindChild(c, "binding_pattern_kind"); ok {
				return file.FlatNodeText(kind) == "var", true
			}
		}
	}
	for body := file.FlatFirstChild(class); body != 0; body = file.FlatNextSib(body) {
		if t := file.FlatType(body); t != "class_body" && t != "enum_class_body" {
			continue
		}
		for m := file.FlatFirstChild(body); m != 0; m = file.FlatNextSib(m) {
			switch file.FlatType(m) {
			case "property_declaration":
				if propertyDeclarationName(file, m) == name {
					return propertyDeclarationIsVar(file, m) || propertyHasGetter(file, m), true
				}
			case "companion_object":
				if mutable, found := classMemberProperty(file, m, name); found {
					return mutable, true
				}
			}
		}
	}
	return false, false
}

// localBindingShadows reports a parameter or local declaration of name before ref in the enclosing function.
func localBindingShadows(file *scanner.File, ref uint32, name string) bool {
	var scope uint32
	for p, ok := file.FlatParent(ref); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "secondary_constructor", "anonymous_initializer", "getter", "setter":
			scope = p
		}
		if scope != 0 {
			break
		}
	}
	if scope == 0 {
		return false
	}
	shadowed := false
	refStart := file.FlatStartByte(ref)
	file.FlatWalkAllNodes(scope, func(n uint32) {
		if shadowed || file.FlatStartByte(n) >= refStart {
			return
		}
		switch file.FlatType(n) {
		case "parameter", "variable_declaration":
			if id, ok := file.FlatFindChild(n, "simple_identifier"); ok && file.FlatNodeText(id) == name {
				shadowed = true
			}
		}
	})
	return shadowed
}

// qualifiedPropertyName renders package.Outer.Inner.name for a member, or package.name for a top-level property.
func qualifiedPropertyName(file *scanner.File, class uint32, name string) string {
	parts := []string{name}
	for p := class; p != 0; {
		if t := file.FlatType(p); t == "class_declaration" || t == "object_declaration" {
			if id, ok := file.FlatFindChild(p, "type_identifier"); ok {
				parts = append([]string{file.FlatNodeText(id)}, parts...)
			}
		}
		parent, ok := file.FlatParent(p)
		if !ok {
			break
		}
		p = parent
	}
	if header, ok := file.FlatFindChild(0, "package_header"); ok {
		if id, ok := file.FlatFindChild(header, "identifier"); ok {
			parts = append([]string{strings.TrimSpace(file.FlatNodeText(id))}, parts...)
		}
	}
	return strings.Join(parts, ".")
}

// ---------------------------------------------------------------------------
// NullableToStringCallRule detects .toString() on nullable.
// ---------------------------------------------------------------------------
type NullableToStringCallRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — needs resolver
// to know whether the receiver is nullable; heuristic fallback matches
// common null-returning APIs. Classified per roadmap/17.
func (r *NullableToStringCallRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NullableToStringCallRule) NodeTypes() []string {
	return []string{"call_expression", "string_literal"}
}

func (r *NullableToStringCallRule) check(ctx *api.Context) {
	if ctx.Resolver == nil {
		return
	}
	switch ctx.File.FlatType(ctx.Idx) {
	case "call_expression":
		r.checkNullableToStringCall(ctx)
	case "string_literal":
		r.checkNullableStringTemplate(ctx)
	}
}

func (r *NullableToStringCallRule) checkNullableToStringCall(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if flatCallExpressionName(file, idx) != "toString" {
		return
	}
	nav, args := flatCallExpressionParts(file, idx)
	if nav == 0 || flatCallHasArguments(file, args) {
		return
	}
	if flatNavigationLastSuffixOperator(file, nav) == "?." {
		return
	}
	receiver := flatUnwrapParenExpr(file, flatNullCheckNavigationReceiver(file, nav))
	receiverType, ok := flatCompilerOrKnownResolvedType(file, ctx.Resolver, receiver)
	if !ok || !receiverType.IsNullable() {
		return
	}
	confidence, ok := flatNullableToStringCallConfidence(ctx, idx, receiver)
	if !ok {
		return
	}
	if flatSameFileNullableToStringExtensionMatches(file, receiverType.Name) {
		return
	}

	finding := r.Finding(file, calleeLine(file, idx), calleeCol(file, idx),
		"Calling toString() on a nullable receiver may produce the string \"null\". Use a safe call with an explicit fallback instead.")
	finding.Confidence = confidence
	ctx.Emit(finding)
}

func (r *NullableToStringCallRule) checkNullableStringTemplate(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		expr := flatStringTemplateInterpolationExpression(file, child)
		if expr == 0 {
			continue
		}
		if !flatTemplateExpressionIsResolvedNullable(file, ctx.Resolver, expr) {
			continue
		}
		ctx.Emit(r.Finding(file, file.FlatRow(child)+1, file.FlatCol(child)+1,
			"Interpolating a nullable expression may produce the string \"null\". Use an explicit fallback instead."))
	}
}

func flatCallHasArguments(file *scanner.File, args uint32) bool {
	return file != nil && args != 0 && file.FlatNamedChildCount(args) > 0
}

func flatNullableToStringCallConfidence(ctx *api.Context, call uint32, receiver uint32) (float64, bool) {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok {
		return 0, false
	}
	if target.Resolved {
		if target.QualifiedName == "kotlin.toString" || target.QualifiedName == "kotlin.Any.toString" {
			return 0.95, true
		}
		return 0, false
	}
	if _, hasOracle := ctx.Resolver.(*oracle.CompositeResolver); hasOracle {
		return 0, false
	}
	if !flatNullableToStringHasLocalEvidence(ctx.File, ctx.Resolver, receiver) {
		return 0, false
	}
	return 0.80, true
}

func flatNullableToStringHasLocalEvidence(file *scanner.File, resolver typeinfer.TypeResolver, receiver uint32) bool {
	if file == nil || resolver == nil || receiver == 0 {
		return false
	}
	switch file.FlatType(receiver) {
	case "simple_identifier", "this_expression":
		return flatReferenceHasSameFileTarget(file, receiver)
	case "call_expression":
		return flatCallHasResolvedTarget(file, resolver, receiver)
	case "navigation_expression":
		_, ok := flatNavigationResolvedMemberType(file, resolver, receiver)
		return ok
	default:
		return false
	}
}

func flatNavigationLastSuffixOperator(file *scanner.File, nav uint32) string {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" {
		return ""
	}
	var suffix uint32
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "navigation_suffix" {
			suffix = child
		}
	}
	for child := file.FlatFirstChild(suffix); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case ".", "?.":
			return file.FlatType(child)
		}
	}
	return ""
}

func flatSameFileNullableToStringExtensionMatches(file *scanner.File, receiverType string) bool {
	if file == nil || receiverType == "" {
		return false
	}
	matched := false
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if matched || file.FlatType(candidate) != "function_declaration" || flatFunctionName(file, candidate) != "toString" {
			return
		}
		name, nullable := flatFunctionReceiverTypeInfo(file, candidate)
		matched = nullable && (name == receiverType || name == "Any")
	})
	return matched
}

func flatFunctionReceiverTypeInfo(file *scanner.File, fn uint32) (string, bool) {
	if file == nil || fn == 0 || file.FlatType(fn) != "function_declaration" {
		return "", false
	}
	var name string
	var nullable bool
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "nullable_type":
			nullable = true
			name = flatLastIdentifierInNode(file, child)
		case "user_type", "type_identifier":
			if name == "" {
				name = flatLastIdentifierInNode(file, child)
			}
		case ".":
			return name, nullable
		case "function_value_parameters", "function_body":
			return "", false
		}
	}
	return "", false
}

func flatStringTemplateInterpolationExpression(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 {
		return 0
	}
	switch file.FlatType(idx) {
	case "interpolated_identifier":
		return idx
	case "interpolated_expression", "line_string_expression", "multi_line_string_expression":
		if file.FlatNamedChildCount(idx) > 0 {
			return file.FlatNamedChild(idx, 0)
		}
	}
	return 0
}

func flatTemplateExpressionIsResolvedNullable(file *scanner.File, resolver typeinfer.TypeResolver, expr uint32) bool {
	if file == nil || resolver == nil || expr == 0 {
		return false
	}
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "postfix_expression":
		if flatPostfixOperatorIs(file, expr, "!!") {
			return false
		}
	case "navigation_expression":
		if flatNavigationLastSuffixOperator(file, expr) == "?." {
			return true
		}
	case "call_expression":
		if nav, _ := flatCallExpressionParts(file, expr); nav != 0 && flatNavigationLastSuffixOperator(file, nav) == "?." {
			return true
		}
	case "interpolated_identifier", "simple_identifier":
		// The compiler's verdict at the reference position knows smart casts.
		if resolved := resolver.ResolveFlatNode(expr, file); resolved != nil && resolved.Resolved && resolved.Kind != typeinfer.TypeUnknown {
			return resolved.IsNullable()
		}
		resolved := resolver.ResolveByNameFlat(file.FlatNodeString(expr, nil), expr, file)
		return resolved != nil && resolved.Kind != typeinfer.TypeUnknown && resolved.IsNullable()
	}
	resolved, ok := flatKnownResolvedType(file, resolver, expr)
	return ok && resolved.IsNullable()
}

func flatPostfixOperatorIs(file *scanner.File, postfix uint32, op string) bool {
	for c := file.FlatFirstChild(postfix); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == op || (file.FlatType(c) == "postfix_unary_operator" && file.FlatNodeText(c) == op) {
			return true
		}
	}
	return false
}

// flatCompilerOrKnownResolvedType prefers the type the compiler recorded at a plain reference, which already
// reflects smart casts (and their absence on mutable properties), over the source-level guess.
func flatCompilerOrKnownResolvedType(file *scanner.File, resolver typeinfer.TypeResolver, idx uint32) (*typeinfer.ResolvedType, bool) {
	if _, ok := resolver.(*oracle.CompositeResolver); ok && file.FlatType(idx) == "simple_identifier" {
		if resolved := resolver.ResolveFlatNode(idx, file); resolved != nil && resolved.Resolved && resolved.Kind != typeinfer.TypeUnknown && resolved.Kind != typeinfer.TypeGeneric {
			return resolved, true
		}
	}
	return flatKnownResolvedType(file, resolver, idx)
}
