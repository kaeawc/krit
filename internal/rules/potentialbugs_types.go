package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func walkFlatClassMembers(file *scanner.File, parent uint32, fn func(uint32)) {
	if file == nil || parent == 0 || fn == nil {
		return
	}
	file.FlatForEachChild(parent, func(child uint32) {
		switch file.FlatType(child) {
		case "function_declaration":
			fn(child)
		case "class_member_declarations":
			walkFlatClassMembers(file, child, fn)
		}
	})
}

func simpleTypeName(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, "?")
	if idx := strings.Index(text, "<"); idx >= 0 {
		text = text[:idx]
	}
	if idx := strings.LastIndex(text, "."); idx >= 0 {
		text = text[idx+1:]
	}
	return strings.TrimSpace(text)
}

// ---------------------------------------------------------------------------
// EqualsAlwaysReturnsTrueOrFalseRule detects equals() that always returns true/false.
// ---------------------------------------------------------------------------
type EqualsAlwaysReturnsTrueOrFalseRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs types rule. Detection pattern-matches type-related
// constructs; resolver usage when available improves precision but
// fallback is heuristic. Classified per roadmap/17.
func (r *EqualsAlwaysReturnsTrueOrFalseRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// EqualsWithHashCodeExistRule detects equals without hashCode or vice versa.
// ---------------------------------------------------------------------------
type EqualsWithHashCodeExistRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs types rule. Detection pattern-matches type-related
// constructs; resolver usage when available improves precision but
// fallback is heuristic. Classified per roadmap/17.
func (r *EqualsWithHashCodeExistRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// WrongEqualsTypeParameterRule detects equals(other: String) instead of Any?.
// ---------------------------------------------------------------------------
type WrongEqualsTypeParameterRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *WrongEqualsTypeParameterRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if !file.FlatHasModifier(idx, "override") {
		return
	}
	if extractIdentifierFlat(file, idx) != "equals" {
		return
	}
	params, ok := file.FlatFindChild(idx, "function_value_parameters")
	if !ok {
		return
	}
	var firstParam uint32
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "parameter" {
			firstParam = child
			break
		}
	}
	if firstParam == 0 {
		return
	}
	var typeNode uint32
	isNullable := false
	for child := file.FlatFirstChild(firstParam); child != 0; child = file.FlatNextSib(child) {
		ct := file.FlatType(child)
		if ct == "nullable_type" {
			typeNode = child
			isNullable = true
			break
		}
		if ct == "user_type" {
			typeNode = child
			break
		}
	}
	if typeNode == 0 {
		return
	}
	typeText := file.FlatNodeText(typeNode)
	typeName := typeText
	if i := strings.Index(typeName, "?"); i >= 0 {
		typeName = typeName[:i]
	}
	if i := strings.Index(typeName, "<"); i >= 0 {
		typeName = typeName[:i]
	}
	typeName = strings.TrimSpace(typeName)
	if isNullable && typeName == "Any" {
		return
	}
	f := r.Finding(file, file.FlatRow(firstParam)+1, file.FlatCol(firstParam)+1,
		fmt.Sprintf("equals() parameter type is '%s' instead of 'Any?'. This does not properly override Any.equals().", typeText))
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(typeNode)),
		EndByte:     int(file.FlatEndByte(typeNode)),
		Replacement: "Any?",
	}
	ctx.Emit(f)
}

// ---------------------------------------------------------------------------
// CharArrayToStringCallRule detects charArray.toString() calls.
// ---------------------------------------------------------------------------
type CharArrayToStringCallRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CharArrayToStringCallRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if flatCallExpressionName(file, idx) != "toString" {
		return
	}
	navExpr, args := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return
	}
	// Verify 0 arguments
	if args != 0 {
		for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "value_argument" {
				return
			}
		}
	}
	// Get receiver: first named non-navigation_suffix child of navigation_expression
	var receiver uint32
	for child := file.FlatFirstChild(navExpr); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) != "navigation_suffix" {
			receiver = child
			break
		}
	}
	if receiver == 0 {
		return
	}
	if ctx.Resolver != nil {
		receiverText := file.FlatNodeText(receiver)
		simpleName := receiverText
		if dotIdx := strings.LastIndex(simpleName, "."); dotIdx >= 0 {
			simpleName = simpleName[dotIdx+1:]
		}
		resolved := ctx.Resolver.ResolveByNameFlat(simpleName, idx, file)
		if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
			if resolved.Name == "CharArray" || resolved.FQN == "kotlin.CharArray" {
				// Resolver confirmed CharArray type — high confidence.
				r.emitCharArrayFinding(ctx, idx, file, receiver, 0.9)
			}
			return
		}
	}
	// No resolver: heuristic fallback on variable name / declaration.
	if charArrayReceiverFlat(file, receiver) {
		r.emitCharArrayFinding(ctx, idx, file, receiver, 0.8)
	}
}

func (r *CharArrayToStringCallRule) emitCharArrayFinding(ctx *api.Context, callIdx uint32, file *scanner.File, receiver uint32, confidence float64) {
	receiverText := file.FlatNodeText(receiver)
	f := r.Finding(file, file.FlatRow(callIdx)+1, file.FlatCol(callIdx)+1,
		"Calling toString() on a CharArray does not return the string representation. Use String(charArray) instead.")
	f.Confidence = confidence
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(callIdx)),
		EndByte:     int(file.FlatEndByte(callIdx)),
		Replacement: "String(" + receiverText + ")",
	}
	ctx.Emit(f)
}

// charArrayReceiverFlat returns true when receiver appears to be a CharArray:
// either a direct charArrayOf() call, or a same-file identifier whose declaration
// has a CharArray type annotation or charArrayOf() initializer.
func charArrayReceiverFlat(file *scanner.File, receiver uint32) bool {
	rt := file.FlatType(receiver)

	// Direct charArrayOf() call expression
	if rt == "call_expression" {
		return flatCallExpressionName(file, receiver) == "charArrayOf"
	}
	if rt != "simple_identifier" {
		return false
	}
	receiverName := file.FlatNodeText(receiver)

	found := false
	// Check function parameters with CharArray type annotation
	file.FlatWalkNodes(0, "parameter", func(pIdx uint32) {
		if found {
			return
		}
		id, ok := file.FlatFindChild(pIdx, "simple_identifier")
		if !ok || file.FlatNodeText(id) != receiverName {
			return
		}
		for child := file.FlatFirstChild(pIdx); child != 0; child = file.FlatNextSib(child) {
			if charArrayTypeNode(file, child) {
				found = true
				return
			}
		}
	})
	if found {
		return true
	}
	// Check property/local variable declarations
	file.FlatWalkNodes(0, "property_declaration", func(pIdx uint32) {
		if found {
			return
		}
		varDecl, ok := file.FlatFindChild(pIdx, "variable_declaration")
		if !ok {
			return
		}
		id, ok := file.FlatFindChild(varDecl, "simple_identifier")
		if !ok || file.FlatNodeText(id) != receiverName {
			return
		}
		// Explicit CharArray type annotation
		for child := file.FlatFirstChild(varDecl); child != 0; child = file.FlatNextSib(child) {
			if charArrayTypeNode(file, child) {
				found = true
				return
			}
		}
		// charArrayOf() initializer
		if init := propertyInitializerExpression(file, pIdx); expressionContainsCallNamed(file, init, "charArrayOf") {
			found = true
		}
	})
	return found
}

func charArrayTypeNode(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "user_type", "nullable_type":
		return simpleTypeName(file.FlatNodeText(idx)) == "CharArray"
	default:
		return false
	}
}

func expressionContainsCallNamed(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 || name == "" {
		return false
	}
	if file.FlatType(idx) == "call_expression" && flatCallExpressionName(file, idx) == name {
		return true
	}
	found := false
	file.FlatWalkNodes(idx, "call_expression", func(call uint32) {
		if !found && flatCallExpressionName(file, call) == name {
			found = true
		}
	})
	return found
}

// ---------------------------------------------------------------------------
// ImplicitUnitReturnTypeRule detects block-body functions without explicit Unit return type.
// ---------------------------------------------------------------------------
type ImplicitUnitReturnTypeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence because this
// rule fires on block-body function_declarations lacking an explicit
// return type. That includes @Composable functions, which conventionally
// omit ': Unit' — a convention mismatch rather than a bug, but one that
// produces noise on Compose codebases. Medium confidence keeps it off
// default-strict gates without taking it out of the rule set.
func (r *ImplicitUnitReturnTypeRule) Confidence() float64 { return api.ConfidenceMedium }
