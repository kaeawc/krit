package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// VarCouldBeValRule detects var properties that are never reassigned.
type VarCouldBeValRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreLateinitVar bool // if true, skip lateinit var declarations
}

// Confidence reports a tier-2 (medium) base confidence because this
// rule uses per-file scope-reassignment tracking instead of full
// control-flow analysis. It correctly handles the common case (var
// declared and never reassigned in its enclosing scope) but misses
// nuances that flow analysis would catch — e.g. vars reassigned only
// on one branch of a when, or through a captured lambda. Medium
// confidence reflects the known analysis gap.
func (r *VarCouldBeValRule) Confidence() float64 { return api.ConfidenceMedium }

func flatSubtreeHasNodeType(file *scanner.File, idx uint32, nodeType string) bool {
	if file == nil || idx == 0 || nodeType == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if found {
			return
		}
		if file.FlatType(candidate) == nodeType {
			found = true
		}
	})
	return found
}

func (r *VarCouldBeValRule) reassignedNamesFlat(scope uint32, file *scanner.File) map[string]bool {
	return filefacts.NodeFact(fileFactsCache(), file, scope, slotScopeReassignments, func() map[string]bool {
		reassigned := make(map[string]bool)
		file.FlatWalkAllNodes(scope, func(child uint32) {
			switch file.FlatType(child) {
			case "assignment", "assignment_expression", "augmented_assignment":
				if file.FlatChildCount(child) == 0 {
					return
				}
				lhs := file.FlatChild(child, 0)
				if name, ok := reassignedLocalOrThisNameFlat(file, lhs); ok {
					reassigned[name] = true
				}
			case "postfix_expression":
				if name, ok := reassignedPostfixNameFlat(file, child); ok {
					reassigned[name] = true
				}
			case "prefix_expression":
				if name, ok := reassignedPrefixNameFlat(file, child); ok {
					reassigned[name] = true
				}
			}
		})
		return reassigned
	})
}

func reassignedLocalOrThisNameFlat(file *scanner.File, lhs uint32) (string, bool) {
	lhs = flatUnwrapParenExpr(file, lhs)
	switch file.FlatType(lhs) {
	case "simple_identifier":
		return file.FlatNodeText(lhs), true
	case "directly_assignable_expression":
		if name, ok := reassignedThisQualifiedNameFlat(file, lhs); ok {
			return name, true
		}
		if file.FlatNamedChildCount(lhs) == 1 {
			return reassignedLocalOrThisNameFlat(file, file.FlatNamedChild(lhs, 0))
		}
	case "navigation_expression":
		return reassignedThisReceiverNameFlat(file, lhs)
	}
	return "", false
}

func reassignedThisReceiverNameFlat(file *scanner.File, nav uint32) (string, bool) {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" || file.FlatNamedChildCount(nav) == 0 {
		return "", false
	}
	return reassignedThisQualifiedNameFlat(file, nav)
}

func reassignedThisQualifiedNameFlat(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 || file.FlatNamedChildCount(idx) == 0 {
		return "", false
	}
	first := flatUnwrapParenExpr(file, file.FlatNamedChild(idx, 0))
	if file.FlatType(first) != "this_expression" && !file.FlatNodeTextEquals(first, "this") && !strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(first)), "this@") {
		return "", false
	}

	name := ""
	suffixes := 0
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "navigation_suffix" {
			continue
		}
		for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
			if file.FlatIsNamed(gc) && file.FlatType(gc) == "simple_identifier" {
				name = file.FlatNodeText(gc)
				suffixes++
			}
		}
	}
	return name, suffixes == 1 && name != ""
}

// enclosingTypeQualifiersFlat returns the outermost enclosing class/object
// declaration node for a property and the set of simple type names that can be
// used to qualify a write to one of its members (the enclosing object name, the
// enclosing class name, and — for a companion-object member — the enclosing
// class name). The outermost node bounds the subtree that must be scanned for
// qualified reassignments, because a sibling nested class can write
// `EnclosingType.member = ...` outside the member's immediate class_body.
func enclosingTypeQualifiersFlat(file *scanner.File, propertyIdx uint32) (uint32, map[string]bool) {
	if file == nil || propertyIdx == 0 {
		return 0, nil
	}
	names := make(map[string]bool)
	outermost := uint32(0)
	cur := propertyIdx
	for {
		parent, ok := file.FlatParent(cur)
		if !ok || parent == 0 {
			break
		}
		switch file.FlatType(parent) {
		case "class_declaration", "object_declaration":
			if name := file.FlatChildTextOrEmpty(parent, "type_identifier"); name != "" {
				names[name] = true
			} else if name := file.FlatChildTextOrEmpty(parent, "simple_identifier"); name != "" {
				names[name] = true
			}
			outermost = parent
		}
		cur = parent
	}
	if len(names) == 0 {
		return 0, nil
	}
	return outermost, names
}

// reassignedViaEnclosingTypeQualifierFlat reports whether varName is written
// through a qualified assignment whose receiver is one of the enclosing type
// names (e.g. `AppDependencies.provider = x` or `BackupProgressService.title =
// x`). It scans the outermost enclosing type subtree and inspects assignment,
// augmented-assignment, and prefix/postfix increment targets.
func reassignedViaEnclosingTypeQualifierFlat(file *scanner.File, scope uint32, names map[string]bool, varName string) bool {
	if file == nil || scope == 0 || varName == "" || len(names) == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(scope, func(child uint32) {
		if found {
			return
		}
		var target uint32
		switch file.FlatType(child) {
		case "assignment", "assignment_expression", "augmented_assignment":
			if file.FlatChildCount(child) == 0 {
				return
			}
			target = file.FlatChild(child, 0)
		case "postfix_expression", "prefix_expression":
			hasMutation := false
			for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatNodeTextEquals(gc, "++") || file.FlatNodeTextEquals(gc, "--") {
					hasMutation = true
				} else if target == 0 && file.FlatIsNamed(gc) {
					target = gc
				}
			}
			if !hasMutation {
				return
			}
			if file.FlatType(child) == "prefix_expression" {
				target = prefixMutationOperandFlat(file, target)
			}
		default:
			return
		}
		if qualifiedAssignTargetMatchesFlat(file, target, names, varName) {
			found = true
		}
	})
	return found
}

// qualifiedAssignTargetMatchesFlat reports whether an assignment LHS is a
// qualified access of the form `<EnclosingType>.<varName>` where
// <EnclosingType> is in names. The LHS may be parsed either as a
// directly_assignable_expression (receiver simple_identifier + navigation_suffix)
// or as a navigation_expression, depending on context.
func qualifiedAssignTargetMatchesFlat(file *scanner.File, target uint32, names map[string]bool, varName string) bool {
	if file == nil || target == 0 {
		return false
	}
	target = flatUnwrapParenExpr(file, target)
	switch file.FlatType(target) {
	case "directly_assignable_expression":
		// A bare local LHS nests a single child (handled elsewhere); a
		// qualified LHS holds the receiver plus its navigation_suffix.
		if file.FlatNamedChildCount(target) == 1 {
			return qualifiedAssignTargetMatchesFlat(file, file.FlatNamedChild(target, 0), names, varName)
		}
	case "navigation_expression":
		if file.FlatNamedChildCount(target) == 0 {
			return false
		}
	default:
		return false
	}
	first := flatUnwrapParenExpr(file, file.FlatNamedChild(target, 0))
	if file.FlatType(first) != "simple_identifier" || !names[file.FlatNodeText(first)] {
		return false
	}
	selector := ""
	suffixes := 0
	for c := file.FlatFirstChild(target); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) != "navigation_suffix" {
			continue
		}
		for gc := file.FlatFirstChild(c); gc != 0; gc = file.FlatNextSib(gc) {
			if file.FlatIsNamed(gc) && file.FlatType(gc) == "simple_identifier" {
				selector = file.FlatNodeText(gc)
				suffixes++
			}
		}
	}
	return suffixes == 1 && selector == varName
}

func reassignedPostfixNameFlat(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 || file.FlatType(idx) != "postfix_expression" {
		return "", false
	}
	var operand uint32
	hasMutation := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatNodeTextEquals(child, "++") || file.FlatNodeTextEquals(child, "--") {
			hasMutation = true
			continue
		}
		if operand == 0 && file.FlatIsNamed(child) {
			operand = child
		}
	}
	if !hasMutation {
		return "", false
	}
	return reassignedLocalOrThisNameFlat(file, operand)
}

func reassignedPrefixNameFlat(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 || file.FlatType(idx) != "prefix_expression" {
		return "", false
	}
	hasMutation := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatNodeTextEquals(child, "++") || file.FlatNodeTextEquals(child, "--") {
			hasMutation = true
			continue
		}
		if hasMutation && file.FlatIsNamed(child) {
			return reassignedLocalOrThisNameFlat(file, prefixMutationOperandFlat(file, child))
		}
	}
	return "", false
}

// prefixMutationOperandFlat returns the real target of a prefix `++`/`--`.
// tree-sitter-kotlin attaches the prefix operator to a whole binary
// expression (`++count <= max` parses as `++(count <= max)`), but Kotlin
// prefix operators bind tighter than every binary operator, so the mutated
// operand is the leftmost operand of that expression.
func prefixMutationOperandFlat(file *scanner.File, operand uint32) uint32 {
	for operand != 0 {
		switch file.FlatType(operand) {
		case "comparison_expression", "equality_expression", "additive_expression",
			"multiplicative_expression", "range_expression", "infix_expression",
			"elvis_expression", "conjunction_expression", "disjunction_expression",
			"check_expression", "as_expression":
			if file.FlatNamedChildCount(operand) == 0 {
				return operand
			}
			operand = file.FlatNamedChild(operand, 0)
		default:
			return operand
		}
	}
	return operand
}

// companionEnclosingClassBodyFlat returns the class_body of the class that
// declares the companion object owning classBody, or 0 when classBody is not
// a companion object's body. Companion members are accessible unqualified
// from the enclosing class, so writes there reassign them.
func companionEnclosingClassBodyFlat(file *scanner.File, classBody uint32) uint32 {
	companion, ok := file.FlatParent(classBody)
	if !ok || file.FlatType(companion) != "companion_object" {
		return 0
	}
	outerBody, ok := file.FlatParent(companion)
	if !ok || file.FlatType(outerBody) != "class_body" {
		return 0
	}
	return outerBody
}

// companionMemberWrittenFromOuterFlat reports whether the enclosing class
// body writes name as an unqualified companion member. A write only counts
// when no declaration between it and the outer class body (a local, a
// parameter, a loop variable, or an outer-class member of the same name)
// shadows the companion property, since Kotlin resolves those first.
func companionMemberWrittenFromOuterFlat(file *scanner.File, outerBody uint32, name string) bool {
	found := false
	file.FlatWalkAllNodes(outerBody, func(n uint32) {
		if found {
			return
		}
		var got string
		var ok bool
		switch file.FlatType(n) {
		case "assignment", "assignment_expression", "augmented_assignment":
			if file.FlatChildCount(n) == 0 {
				return
			}
			got, ok = reassignedLocalOrThisNameFlat(file, file.FlatChild(n, 0))
		case "postfix_expression":
			got, ok = reassignedPostfixNameFlat(file, n)
		case "prefix_expression":
			got, ok = reassignedPrefixNameFlat(file, n)
		default:
			return
		}
		if ok && got == name && !nameShadowedBeforeFlat(file, n, outerBody, name) {
			found = true
		}
	})
	return found
}

// nameShadowedBeforeFlat reports whether a declaration of name in node's
// scopes, up to and including stop, hides an outer property of that name at
// node: a property declared earlier in an enclosing block or class body, a
// function or lambda parameter, or a for-loop variable.
func nameShadowedBeforeFlat(file *scanner.File, node, stop uint32, name string) bool {
	nodeStart := file.FlatStartByte(node)
	for cur := node; ; {
		p, ok := file.FlatParent(cur)
		if !ok {
			return false
		}
		for i := 0; i < file.FlatChildCount(p); i++ {
			c := file.FlatChild(p, i)
			switch file.FlatType(c) {
			case "property_declaration":
				if file.FlatStartByte(c) >= nodeStart {
					continue
				}
				for j := 0; j < file.FlatChildCount(c); j++ {
					d := file.FlatChild(c, j)
					if file.FlatType(d) == "variable_declaration" && extractIdentifierFlat(file, d) == name {
						return true
					}
				}
			case "variable_declaration":
				if extractIdentifierFlat(file, c) == name {
					return true
				}
			case "function_value_parameters", "lambda_parameters":
				for j := 0; j < file.FlatChildCount(c); j++ {
					param := file.FlatChild(c, j)
					switch file.FlatType(param) {
					case "parameter", "variable_declaration":
						if extractIdentifierFlat(file, param) == name {
							return true
						}
					}
				}
			}
		}
		if p == stop {
			return false
		}
		cur = p
	}
}
