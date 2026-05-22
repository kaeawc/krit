package rules

import (
	"fmt"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// PublicToInternalLeakyAbstractionRule detects public classes whose primary
// constructor stores a single `val` delegate and whose public methods are
// mostly forwarding shims onto that delegate. Such wrappers typically leak
// the inner implementation through a nominally public API. Inactive by
// default.
type PublicToInternalLeakyAbstractionRule struct {
	FlatDispatchBase
	BaseRule
	Threshold float64
}

// Confidence is 0.70 — the heuristic still admits real false positives like
// type-safe wrappers, adapters, and DI holders. Medium confidence keeps
// reviewers honest.
func (r *PublicToInternalLeakyAbstractionRule) Confidence() float64 {
	return api.ConfidenceMediumLowPlus
}

// leakyClassExcludedModifiers lists every class-declaration modifier that
// disqualifies the rule. Visibility modifiers (private/internal/protected)
// remove the class from the public surface; class-kind modifiers
// (abstract/sealed/enum/annotation/data/value/inner) describe semantics
// that don't match the "facade over delegate" shape. `fun` lives here so
// `fun interface` declarations are skipped too.
var leakyClassExcludedModifiers = map[string]struct{}{
	"private": {}, "internal": {}, "protected": {},
	"abstract": {}, "sealed": {}, "enum": {}, "annotation": {},
	"data": {}, "value": {}, "inner": {}, "fun": {},
}

// leakyMethodNonPublicVisibility lists visibility modifiers that exclude a
// function_declaration from the counted-method surface. Override functions
// without an explicit modifier are still treated as public.
var leakyMethodNonPublicVisibility = map[string]struct{}{
	"private": {}, "internal": {}, "protected": {},
}

func (r *PublicToInternalLeakyAbstractionRule) check(ctx *api.Context) {
	file, classIdx := ctx.File, ctx.Idx
	if file == nil || classIdx == 0 || file.FlatType(classIdx) != "class_declaration" {
		return
	}
	if classHasAnyModifier(file, classIdx, leakyClassExcludedModifiers) {
		return
	}
	// Direct parent is class_body iff the class is nested inside another
	// type body; top-level classes have source_file as parent.
	if parent, ok := file.FlatParent(classIdx); ok && file.FlatType(parent) == "class_body" {
		return
	}

	primary, _ := file.FlatFindChild(classIdx, "primary_constructor")
	if primary == 0 {
		return
	}
	fieldName, fieldType, ok := leakySingleValParameter(file, primary)
	if !ok {
		return
	}

	body, _ := file.FlatFindChild(classIdx, "class_body")
	if body == 0 {
		return
	}

	total, delegating := leakyCountClassMethods(file, body, fieldName)
	if total == 0 {
		return
	}
	ratio := float64(delegating) / float64(total)
	if ratio <= r.Threshold {
		return
	}

	ctx.Emit(scanner.Finding{
		File:     file.Path,
		Line:     file.FlatRow(classIdx) + 1,
		Col:      1,
		RuleSet:  r.RuleSetName,
		Rule:     r.RuleName,
		Severity: r.Sev,
		Message: fmt.Sprintf("Public class %s delegates %d of %d methods to internal field %q; consider exposing the internal type directly or adding real behavior.",
			extractIdentifierFlat(file, classIdx), delegating, total, fieldType),
	})
}

// classHasAnyModifier reports whether the declaration at idx carries any
// modifier whose text appears in want. One pass over the modifiers child
// is cheaper than N separate FlatHasModifier calls when the rule wants to
// short-circuit on a large set of names.
func classHasAnyModifier(file *scanner.File, idx uint32, want map[string]struct{}) bool {
	mods, _ := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return false
	}
	for child := file.FlatFirstChild(mods); child != 0; child = file.FlatNextSib(child) {
		if _, hit := want[file.FlatNodeText(child)]; hit {
			return true
		}
		for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
			if _, hit := want[file.FlatNodeText(gc)]; hit {
				return true
			}
		}
	}
	return false
}

// leakySingleValParameter inspects a primary_constructor and returns the
// (name, type) of its sole class_parameter when that parameter is a `val`
// stored property. Returns ok=false for zero/multiple parameters, or when
// the parameter is constructor-scoped only / declared as `var`.
func leakySingleValParameter(file *scanner.File, primary uint32) (name string, typeName string, ok bool) {
	var param uint32
	for child := file.FlatFirstChild(primary); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "class_parameter" {
			continue
		}
		if param != 0 {
			return "", "", false
		}
		param = child
	}
	if param == 0 || !classParameterIsStoredVal(file, param) {
		return "", "", false
	}
	nameIdx, _ := file.FlatFindChild(param, "simple_identifier")
	if nameIdx == 0 {
		return "", "", false
	}
	name = file.FlatNodeText(nameIdx)
	if userType, _ := file.FlatFindChild(param, "user_type"); userType != 0 {
		if ident := flatLastChildOfType(file, userType, "type_identifier"); ident != 0 {
			typeName = file.FlatNodeText(ident)
		}
	}
	return name, typeName, true
}

// classParameterIsStoredVal returns true when a class_parameter is declared
// as a `val` stored property. Tree-sitter Kotlin wraps the `val`/`var`
// keyword in a `binding_pattern_kind` child; a constructor-only parameter
// has no such child.
func classParameterIsStoredVal(file *scanner.File, paramIdx uint32) bool {
	bpk, _ := file.FlatFindChild(paramIdx, "binding_pattern_kind")
	if bpk == 0 {
		return false
	}
	for child := file.FlatFirstChild(bpk); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "val":
			return true
		case "var":
			return false
		}
	}
	return false
}

// leakyCountClassMethods walks the direct function_declaration children of
// a class_body. Methods inside nested classes, companion objects, or local
// declarations are intentionally excluded — only the outer class's own
// surface counts toward the delegation ratio.
func leakyCountClassMethods(file *scanner.File, body uint32, fieldName string) (total, delegating int) {
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "function_declaration" {
			continue
		}
		if classHasAnyModifier(file, child, leakyMethodNonPublicVisibility) {
			continue
		}
		total++
		if leakyFunctionDelegatesTo(file, child, fieldName) {
			delegating++
		}
	}
	return total, delegating
}

// leakyFunctionDelegatesTo reports whether fn is a single-statement
// forwarding shim that calls a method on the receiver named fieldName.
// Recognized shapes:
//
//	fun foo(args) = fieldName.foo(args)                      // expression body
//	fun foo(args) { fieldName.foo(args) }                    // block body, single call
//	fun foo(args): T { return fieldName.foo(args) }          // block body, single return
func leakyFunctionDelegatesTo(file *scanner.File, fn uint32, fieldName string) bool {
	body, _ := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return false
	}
	stmt := leakyDelegationCandidate(file, body)
	if stmt == 0 {
		return false
	}
	return leakyExpressionIsDelegateCall(file, stmt, fieldName)
}

// leakyDelegationCandidate returns the single expression that represents a
// function body's delegation candidate, or 0 when the body doesn't reduce
// to one. Tree-sitter Kotlin emits both shapes under `function_body`:
// expression body (`= expr`) and block body (`{ statements }`); a single
// `return expr` is unwrapped so callers see the payload directly.
func leakyDelegationCandidate(file *scanner.File, body uint32) uint32 {
	if expr := firstNamedChildAfterToken(file, body, "="); expr != 0 {
		return expr
	}
	stmts, _ := file.FlatFindChild(body, "statements")
	if stmts == 0 {
		return 0
	}
	var only uint32
	for child := file.FlatFirstChild(stmts); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if only != 0 {
			return 0
		}
		only = child
	}
	if only == 0 {
		return 0
	}
	if file.FlatType(only) == "jump_expression" {
		return swallowedJumpExpressionValue(file, only)
	}
	return only
}

// firstNamedChildAfterToken returns the first named child of parent that
// follows a non-named token whose type equals sentinel. Used to grab the
// expression on the right of `=` in an expression-body function_body.
func firstNamedChildAfterToken(file *scanner.File, parent uint32, sentinel string) uint32 {
	seen := false
	for child := file.FlatFirstChild(parent); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			if file.FlatType(child) == sentinel {
				seen = true
			}
			continue
		}
		if seen {
			return child
		}
	}
	return 0
}

// leakyExpressionIsDelegateCall reports whether expr is `fieldName.<m>(args)`.
// Chained calls like `fieldName.first().second()` are rejected — the outer
// call's receiver is itself a call_expression, so its navigation chain has
// more than two simple_identifier segments.
func leakyExpressionIsDelegateCall(file *scanner.File, expr uint32, fieldName string) bool {
	if expr == 0 || file.FlatType(expr) != "call_expression" {
		return false
	}
	nav, _ := flatCallExpressionParts(file, expr)
	if nav == 0 || file.FlatType(nav) != "navigation_expression" {
		return false
	}
	return navigationReceiverIsBareIdentifier(file, nav, fieldName)
}

// navigationReceiverIsBareIdentifier returns true when nav is a two-segment
// navigation_expression whose receiver is the bare identifier `name`.
// Avoids the slice allocation that flatNavigationChainIdentifiers does
// when the caller only needs to know the receiver and chain length.
func navigationReceiverIsBareIdentifier(file *scanner.File, nav uint32, name string) bool {
	var receiver uint32
	suffixes := 0
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			if receiver != 0 {
				return false
			}
			receiver = child
		case "navigation_suffix":
			suffixes++
		case "navigation_expression":
			// Chained receiver (e.g. `a.b.c`): not a direct delegation.
			return false
		}
	}
	return suffixes == 1 && receiver != 0 && file.FlatNodeTextEquals(receiver, name)
}
