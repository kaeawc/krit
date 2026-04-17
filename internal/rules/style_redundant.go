package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// RedundantVisibilityModifierRule detects explicit `public` keyword.
type RedundantVisibilityModifierRule struct {
	FlatDispatchBase
	BaseRule
}

var redundantPublicRe = regexp.MustCompile(`\bpublic\b`)

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *RedundantVisibilityModifierRule) Confidence() float64 { return 0.75 }

func (r *RedundantVisibilityModifierRule) NodeTypes() []string { return []string{"modifiers"} }

func (r *RedundantVisibilityModifierRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Check for "public" and absence of "override" using AST child walking.
	// This node IS a "modifiers" node, so walk its children directly.
	hasPublic := false
	hasOverride := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		childText := file.FlatNodeText(child)
		if childText == "public" {
			hasPublic = true
		}
		if childText == "override" {
			hasOverride = true
		}
		// Modifier keywords may be wrapped (e.g. visibility_modifier > "public")
		for j := 0; j < file.FlatChildCount(child); j++ {
			gcText := file.FlatNodeText(file.FlatChild(child, j))
			if gcText == "public" {
				hasPublic = true
			}
			if gcText == "override" {
				hasOverride = true
			}
		}
	}
	if hasPublic && !hasOverride {
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			"Redundant 'public' modifier. Public is the default visibility in Kotlin.")
		// Find the visibility_modifier child with "public"
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			if file.FlatType(child) == "visibility_modifier" && file.FlatNodeTextEquals(child, "public") {
				startByte := int(file.FlatStartByte(child))
				endByte := int(file.FlatEndByte(child))
				// Also consume trailing whitespace
				for endByte < len(file.Content) && (file.Content[endByte] == ' ' || file.Content[endByte] == '\t') {
					endByte++
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   startByte,
					EndByte:     endByte,
					Replacement: "",
				}
				break
			}
		}
		return []scanner.Finding{f}
	}
	return nil
}

// RedundantConstructorKeywordRule detects unnecessary `constructor` keyword.
// Flags primary constructors that use the explicit `constructor` keyword when
// there are no annotations or visibility modifiers on the constructor itself.
type RedundantConstructorKeywordRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *RedundantConstructorKeywordRule) Confidence() float64 { return 0.75 }

func (r *RedundantConstructorKeywordRule) NodeTypes() []string {
	return []string{"class_declaration"}
}

func (r *RedundantConstructorKeywordRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	ctor := file.FlatFindChild(idx, "primary_constructor")
	if ctor == 0 {
		return nil
	}
	if file.FlatFindChild(ctor, "modifiers") != 0 {
		return nil
	}

	// Check whether the constructor text contains the explicit "constructor" keyword.
	ctorText := file.FlatNodeText(ctor)
	if !strings.Contains(ctorText, "constructor") {
		return nil
	}

	f := r.Finding(file, file.FlatRow(ctor)+1, 1,
		"Redundant 'constructor' keyword. Remove it when there are no annotations or visibility modifiers.")

	// Auto-fix: remove " constructor" keeping only the parameter list.
	// Walk back from constructor start to consume preceding whitespace.
	startByte := int(file.FlatStartByte(ctor))
	for startByte > 0 && (file.Content[startByte-1] == ' ' || file.Content[startByte-1] == '\t') {
		startByte--
	}
	// Find the parameter list (class_parameters) inside the constructor node.
	paramList := file.FlatFindChild(ctor, "class_parameters")
	if paramList != 0 {
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   startByte,
			EndByte:     int(file.FlatStartByte(paramList)),
			Replacement: "",
		}
	}

	return []scanner.Finding{f}
}

// RedundantExplicitTypeRule detects type annotations where the type is obvious.
// With type inference: uses ResolveNode on both the declared type and the initializer
// expression. If both resolve to the same FQN, the explicit type is redundant.
type RedundantExplicitTypeRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *RedundantExplicitTypeRule) SetResolver(res typeinfer.TypeResolver) {
	r.resolver = res
}

// Confidence reports a tier-2 (medium) base confidence — inferring
// whether the type annotation is necessary requires the resolver; fallback
// is conservative. Classified per roadmap/17.
func (r *RedundantExplicitTypeRule) Confidence() float64 { return 0.75 }

func (r *RedundantExplicitTypeRule) NodeTypes() []string { return []string{"property_declaration"} }

func (r *RedundantExplicitTypeRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Must have both an explicit type annotation and an initializer
	var typeNode uint32
	var initExpr uint32
	hasEquals := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "user_type", "nullable_type":
			typeNode = child
		case "=":
			hasEquals = true
		default:
			if hasEquals && initExpr == 0 && file.FlatType(child) != "property_delegate" {
				initExpr = child
			}
		}
		// Also check inside variable_declaration
		if file.FlatType(child) == "variable_declaration" {
			for j := 0; j < file.FlatChildCount(child); j++ {
				gc := file.FlatChild(child, j)
				if t := file.FlatType(gc); t == "user_type" || t == "nullable_type" {
					typeNode = gc
				}
			}
		}
	}
	if typeNode == 0 || initExpr == 0 {
		return nil
	}

	// --- Resolver-based matching (preferred) ---
	if r.resolver != nil {
		declaredType := r.resolver.ResolveFlatNode(typeNode, file)
		inferredType := r.resolver.ResolveFlatNode(initExpr, file)
		if declaredType.Kind != typeinfer.TypeUnknown && inferredType.Kind != typeinfer.TypeUnknown {
			match := false
			if declaredType.FQN != "" && inferredType.FQN != "" {
				match = declaredType.FQN == inferredType.FQN && declaredType.Nullable == inferredType.Nullable
			} else if declaredType.Name != "" && inferredType.Name != "" {
				match = declaredType.Name == inferredType.Name && declaredType.Nullable == inferredType.Nullable
			}
			if match {
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"Redundant explicit type. Type can be inferred from the initializer.")
				f.Fix = r.buildFixFlat(typeNode, file)
				return []scanner.Finding{f}
			}
			return nil
		}
		// Fall through to literal matching if resolver returned unknown
	}

	// --- Fallback: literal pattern matching via AST nodes ---
	typeText := file.FlatNodeText(typeNode)
	initType := file.FlatType(initExpr)
	initText := file.FlatNodeText(initExpr)

	matched := false
	switch initType {
	case "string_literal":
		matched = typeText == "String"
	case "boolean_literal":
		matched = typeText == "Boolean"
	case "character_literal":
		matched = typeText == "Char"
	case "integer_literal":
		if strings.HasSuffix(initText, "L") || strings.HasSuffix(initText, "l") {
			matched = typeText == "Long"
		} else {
			matched = typeText == "Int"
		}
	case "real_literal":
		if strings.HasSuffix(initText, "f") || strings.HasSuffix(initText, "F") {
			matched = typeText == "Float"
		} else {
			matched = typeText == "Double"
		}
	case "call_expression":
		// val x: Foo = Foo(...) — constructor call matches type name
		callee := file.FlatFindChild(initExpr, "simple_identifier")
		if callee != 0 && file.FlatNodeTextEquals(callee, typeText) {
			matched = true
		}
	case "simple_identifier":
		// val x: Foo = SomeRef — only match if name reference equals type text
		if initText == typeText {
			matched = true
		}
	}

	if matched {
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			"Redundant explicit type. Type can be inferred from the initializer.")
		f.Fix = r.buildFixFlat(typeNode, file)
		return []scanner.Finding{f}
	}
	return nil
}

func (r *RedundantExplicitTypeRule) buildFixFlat(typeNode uint32, file *scanner.File) *scanner.Fix {
	typeStart := int(file.FlatStartByte(typeNode))
	typeEnd := int(file.FlatEndByte(typeNode))
	colonPos := typeStart - 1
	for colonPos >= 0 && (file.Content[colonPos] == ' ' || file.Content[colonPos] == '\t') {
		colonPos--
	}
	if colonPos >= 0 && file.Content[colonPos] == ':' {
		startRemove := colonPos
		for startRemove > 0 && (file.Content[startRemove-1] == ' ' || file.Content[startRemove-1] == '\t') {
			startRemove--
		}
		return &scanner.Fix{
			ByteMode:    true,
			StartByte:   startRemove,
			EndByte:     typeEnd,
			Replacement: "",
		}
	}
	return nil
}

// UnnecessaryParenthesesRule detects unnecessary parentheses around expressions.
// Matches detekt's UnnecessaryParentheses: flags parens around return values,
// if/when conditions, assignments, double-wrapped parens, and other contexts
// where the parentheses add no value.
type UnnecessaryParenthesesRule struct {
	FlatDispatchBase
	BaseRule
	AllowForUnclearPrecedence bool // if true, allow parens that clarify operator precedence
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *UnnecessaryParenthesesRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryParenthesesRule) NodeTypes() []string {
	return []string{"parenthesized_expression"}
}

func (r *UnnecessaryParenthesesRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	parent, ok := file.FlatParent(idx)
	if !ok {
		return nil
	}

	// Find the inner expression (skip "(" and ")" tokens).
	var inner uint32
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		if child != 0 {
			inner = child
			break
		}
	}
	if inner == 0 {
		return nil
	}

	parentType := file.FlatType(parent)

	// Never flag parens inside delegated_super_type (matches detekt).
	if parentType == "delegation_specifier" || parentType == "delegated_super_type" {
		return nil
	}

	redundant := false

	switch parentType {
	case "jump_expression":
		// return (x), throw (x) — parens always unnecessary.
		redundant = true

	case "parenthesized_expression":
		// Double parens: ((x)) — inner parens always unnecessary.
		redundant = true

	case "property_declaration", "variable_declaration":
		// val x = (expr) — parens unnecessary around entire RHS.
		redundant = true

	case "assignment":
		// x = (expr) — parens unnecessary around entire RHS.
		redundant = true

	case "value_argument", "value_arguments":
		// foo((expr)) — parens unnecessary around a single argument
		// unless it's a lambda (parenthesized lambda prevents trailing lambda syntax).
		if t := file.FlatType(inner); t == "lambda_literal" || t == "annotated_lambda" {
			redundant = false
		} else {
			redundant = true
		}

	case "if_expression":
		// The condition of an `if` is already wrapped in parens by syntax.
		// if ((x > 0)) — the inner parenthesized_expression is redundant.
		redundant = unnParensIsIfConditionFlat(file, idx, parent)

	case "when_expression":
		// when ((x)) — parens around the subject are unnecessary.
		redundant = unnParensIsWhenSubjectFlat(file, idx, parent)

	case "when_condition":
		// Parens inside a when condition: when (x) { (0) -> ... }
		redundant = true

	case "indexing_expression":
		// a[(expr)] — parens around index are unnecessary.
		redundant = true

	case "statements":
		// Top-level expression statement: (expr) on its own line.
		redundant = true

	default:
		// For other contexts, parens are redundant only if the inner
		// expression is a simple identifier, literal, string, or already
		// grouped (call_expression, navigation_expression, etc.) — i.e.,
		// removing the parens won't change precedence.
		redundant = unnParensInnerIsSafeFlat(file, inner)
	}

	if !redundant {
		return nil
	}

	// If AllowForUnclearPrecedence is set, keep parens that clarify
	// operator precedence (inner is binary op with a binary-op parent).
	if r.AllowForUnclearPrecedence && unnParensClarifyPrecedenceFlat(file, idx, inner) {
		return nil
	}

	innerText := file.FlatNodeText(inner)
	nodeText := file.FlatNodeText(idx)
	msg := fmt.Sprintf("Unnecessary parentheses in %s. Can be replaced with: %s", nodeText, innerText)

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)

	// Auto-fix: replace the parenthesized_expression bytes with the inner expression text.
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: innerText,
	}
	return []scanner.Finding{f}
}

func unnParensIsIfConditionFlat(file *scanner.File, node, parent uint32) bool {
	for i := 0; i < file.FlatChildCount(parent); i++ {
		child := file.FlatChild(parent, i)
		if child == 0 {
			continue
		}
		if t := file.FlatType(child); t == "control_structure_body" || t == "{" {
			return false
		}
		if child == node {
			return true
		}
	}
	return false
}

func unnParensIsWhenSubjectFlat(file *scanner.File, node, parent uint32) bool {
	for i := 0; i < file.FlatChildCount(parent); i++ {
		child := file.FlatChild(parent, i)
		if child == 0 {
			continue
		}
		if t := file.FlatType(child); t == "when_entry" || t == "{" {
			return false
		}
		if child == node {
			return true
		}
	}
	return false
}

func unnParensInnerIsSafeFlat(file *scanner.File, inner uint32) bool {
	switch file.FlatType(inner) {
	case "simple_identifier", "integer_literal", "long_literal",
		"real_literal", "boolean_literal", "character_literal",
		"string_literal", "null_literal", "this_expression",
		"super_expression", "call_expression", "navigation_expression",
		"indexing_expression", "parenthesized_expression",
		"object_literal", "lambda_literal", "when_expression",
		"if_expression", "try_expression", "collection_literal":
		return true
	}
	return false
}

// Binary operator node types in tree-sitter Kotlin.
var unnParensBinaryExprTypes = map[string]bool{
	"multiplicative_expression": true,
	"additive_expression":       true,
	"range_expression":          true,
	"infix_expression":          true,
	"elvis_expression":          true,
	"check_expression":          true,
	"comparison_expression":     true,
	"equality_expression":       true,
	"conjunction_expression":    true,
	"disjunction_expression":    true,
}

// Precedence rank (higher = binds tighter).
var unnParensBinaryPrecedence = map[string]int{
	"disjunction_expression":    1,
	"conjunction_expression":    2,
	"equality_expression":       3,
	"comparison_expression":     4,
	"check_expression":          5,
	"range_expression":          6,
	"additive_expression":       7,
	"multiplicative_expression": 8,
	"infix_expression":          9,
	"elvis_expression":          10,
}

func unnParensClarifyPrecedenceFlat(file *scanner.File, parenNode, inner uint32) bool {
	if !unnParensBinaryExprTypes[file.FlatType(inner)] {
		if file.FlatType(inner) == "prefix_expression" {
			outerParent, ok := file.FlatParent(parenNode)
			if ok && unnParensBinaryExprTypes[file.FlatType(outerParent)] {
				return true
			}
		}
		return false
	}
	outerParent, ok := file.FlatParent(parenNode)
	for ok && file.FlatType(outerParent) == "parenthesized_expression" {
		outerParent, ok = file.FlatParent(outerParent)
	}
	if !ok || !unnParensBinaryExprTypes[file.FlatType(outerParent)] {
		return false
	}
	innerPrec := unnParensBinaryPrecedence[file.FlatType(inner)]
	outerPrec := unnParensBinaryPrecedence[file.FlatType(outerParent)]
	return innerPrec != outerPrec
}

// UnnecessaryInheritanceRule detects `: Any()`.
type UnnecessaryInheritanceRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *UnnecessaryInheritanceRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryInheritanceRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *UnnecessaryInheritanceRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Look for delegation_specifier children that are `: Any()`
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		text := file.FlatNodeText(child)
		if text != "Any()" {
			continue
		}
		f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			"Unnecessary inheritance from 'Any'. All classes extend Any implicitly.")
		// Remove the `: Any()` portion — find the colon before the delegation_specifier
		startByte := int(file.FlatStartByte(child))
		endByte := int(file.FlatEndByte(child))
		// Walk backwards from the delegation_specifier to remove the `: ` prefix
		for sb := startByte - 1; sb >= 0; sb-- {
			ch := file.Content[sb]
			if ch == ':' {
				startByte = sb
				break
			}
			if ch != ' ' && ch != '\t' {
				break
			}
		}
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   startByte,
			EndByte:     endByte,
			Replacement: "",
		}
		return []scanner.Finding{f}
	}
	return nil
}

// UnnecessaryInnerClassRule detects inner classes that don't use the outer reference.
type UnnecessaryInnerClassRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *UnnecessaryInnerClassRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryInnerClassRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *UnnecessaryInnerClassRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	mods := file.FlatFindChild(idx, "modifiers")
	body := file.FlatFindChild(idx, "class_body")
	if mods == 0 || body == 0 {
		return nil
	}
	// Verify the "inner" modifier is present.
	if !strings.Contains(file.FlatNodeText(mods), "inner") {
		return nil
	}
	bodyText := file.FlatNodeText(body)
	// Check if the body references this@OuterClass or the outer class's members
	if !strings.Contains(bodyText, "this@") && !strings.Contains(bodyText, "@") {
		name := extractIdentifierFlat(file, idx)
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Inner class '%s' does not use the outer class reference. Remove 'inner' modifier.", name))
		modsText := file.FlatNodeText(mods)
		newMods := strings.Replace(modsText, "inner ", "", 1)
		if newMods == modsText {
			newMods = strings.Replace(modsText, "inner", "", 1)
		}
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(mods)),
			EndByte:     int(file.FlatEndByte(mods)),
			Replacement: newMods,
		}
		return []scanner.Finding{f}
	}
	return nil
}

// OptionalUnitRule detects explicit `: Unit` return types on functions
// and redundant `return Unit` statements inside function bodies.
type OptionalUnitRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *OptionalUnitRule) Confidence() float64 { return 0.75 }

func (r *OptionalUnitRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *OptionalUnitRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding

	// 1. Check for explicit `: Unit` return type annotation.
	// In the tree-sitter AST, function_declaration children include a ":"
	// token followed by a user_type node when a return type is specified.
	colonIdx := -1
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == ":" {
			colonIdx = i
		}
		if colonIdx >= 0 && file.FlatType(child) == "user_type" {
			typeText := file.FlatNodeText(child)
			if typeText == "Unit" {
				f := r.Finding(file, file.FlatRow(child)+1,
					file.FlatCol(child)+1,
					"Unit return type is optional and can be omitted.")
				// Remove ": Unit" including the colon and any surrounding whitespace
				colonNode := file.FlatChild(idx, colonIdx)
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(colonNode)),
					EndByte:     int(file.FlatEndByte(child)),
					Replacement: "",
				}
				findings = append(findings, f)
			}
			break
		}
	}

	// 2. Check for `return Unit` statements inside the function body using compiled query.
	body := file.FlatFindChild(idx, "function_body")
	if body != 0 {
		file.FlatWalkNodes(body, "jump_expression", func(jump uint32) {
			if file.FlatChildCount(jump) < 2 {
				return
			}
			first := file.FlatChild(jump, 0)
			if file.FlatType(first) != "return" {
				return
			}
			second := file.FlatChild(jump, 1)
			if file.FlatType(second) == "simple_identifier" && file.FlatNodeTextEquals(second, "Unit") {
				f := r.Finding(file, file.FlatRow(jump)+1,
					file.FlatCol(jump)+1,
					"return Unit is redundant and can be replaced with return.")
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatEndByte(first)),
					EndByte:     int(file.FlatEndByte(second)),
					Replacement: "",
				}
				findings = append(findings, f)
			}
		})
	}

	return findings
}

// UnnecessaryBackticksRule detects unnecessary backtick-quoted identifiers.
type UnnecessaryBackticksRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *UnnecessaryBackticksRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryBackticksRule) NodeTypes() []string { return []string{"simple_identifier"} }

func (r *UnnecessaryBackticksRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if len(text) < 3 || text[0] != '`' || text[len(text)-1] != '`' {
		return nil
	}
	inner := text[1 : len(text)-1]

	// Backticks are needed for keywords and all-underscore identifiers.
	if isKotlinKeyword(inner) || isAllUnderscores(inner) {
		return nil
	}

	// Must be a valid Kotlin identifier without backticks.
	if !isValidKotlinIdentifier(inner) {
		return nil
	}

	// Inside a string template, removing backticks may merge with adjacent text.
	// e.g. "$`foo`bar" — removing backticks yields "$foobar" (different meaning).
	endByte := int(file.FlatEndByte(idx))
	if endByte < len(file.Content) && isInsideStringTemplateFlat(file, idx) {
		nextCh := file.Content[endByte]
		if isIdentChar(nextCh) {
			return nil
		}
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Unnecessary backticks around '%s'.", inner))
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     endByte,
		Replacement: inner,
	}
	return []scanner.Finding{f}
}

func isAllUnderscores(s string) bool {
	for _, ch := range s {
		if ch != '_' {
			return false
		}
	}
	return len(s) > 0
}

func isValidKotlinIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, ch := range s {
		if i == 0 {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_') {
				return false
			}
		} else {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
				return false
			}
		}
	}
	return true
}

func isIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func isInsideStringTemplateFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		t := file.FlatType(p)
		if t == "line_string_literal" || t == "multi_line_string_literal" {
			return true
		}
	}
	return false
}

func isKotlinKeyword(s string) bool {
	switch s {
	case "as", "break", "class", "continue", "do", "else", "false", "for", "fun",
		"if", "in", "interface", "is", "null", "object", "package", "return",
		"super", "this", "throw", "true", "try", "typealias", "typeof", "val",
		"var", "when", "while":
		return true
	}
	return false
}

// UselessCallOnNotNullRule detects `.orEmpty()`, `.isNullOrEmpty()`, `.isNullOrBlank()`
// on definitely non-null receivers, and `listOfNotNull()`/`setOfNotNull()` with all
// non-null arguments. Mirrors detekt's UselessCallOnNotNull.
type UselessCallOnNotNullRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *UselessCallOnNotNullRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — flags ?-safe
// calls on provably non-null receivers; resolver-dependent. Classified per
// roadmap/17.
func (r *UselessCallOnNotNullRule) Confidence() float64 { return 0.75 }

func (r *UselessCallOnNotNullRule) NodeTypes() []string { return []string{"call_expression"} }

// uselessNullCalls maps method names that are useless on non-null receivers to
// their replacement. Empty string means "remove the call entirely".
var uselessNullCalls = map[string]string{
	"orEmpty":       "",
	"isNullOrEmpty": "isEmpty",
	"isNullOrBlank": "isBlank",
}

// orEmptyValidTypes lists types that actually define .orEmpty().
var orEmptyValidTypes = map[string]bool{
	"String": true, "List": true, "Set": true, "Map": true,
	"Sequence":    true,
	"MutableList": true, "MutableSet": true, "MutableMap": true,
}

// isNullOrValidTypes lists types that define .isNullOrEmpty() / .isNullOrBlank().
var isNullOrValidTypes = map[string]bool{
	"String": true, "CharSequence": true,
	"List": true, "Set": true, "Map": true, "Collection": true,
	"MutableList": true, "MutableSet": true, "MutableMap": true,
}

// ofNotNullReplacements maps factory functions to their non-null equivalents.
var ofNotNullReplacements = map[string]string{
	"listOfNotNull": "listOf",
	"setOfNotNull":  "setOf",
}

// nullableStdlibCallMarkers are substrings that, when present in an
// argument expression, signal that the argument's value is nullable
// because it's the result of a stdlib function known to return T?.
// Used by UselessCallOnNotNull's *OfNotNull check to avoid false
// positives when the resolver doesn't encode the nullable return type.
var nullableStdlibCallMarkers = []string{
	".takeIf", ".takeUnless",
	".firstOrNull", ".lastOrNull", ".singleOrNull",
	".findOrNull", ".maxOrNull", ".minOrNull",
	".getOrNull", ".randomOrNull",
	".maxByOrNull", ".minByOrNull", ".maxWithOrNull", ".minWithOrNull",
	".toIntOrNull", ".toLongOrNull", ".toDoubleOrNull",
	".toFloatOrNull", ".toByteOrNull", ".toShortOrNull",
	".toBigIntegerOrNull", ".toBigDecimalOrNull",
	".toBooleanStrictOrNull", ".toUIntOrNull", ".toULongOrNull",
}

// containsNullableStdlibCall reports whether the expression text
// contains any call to a stdlib function that returns T?.
func containsNullableStdlibCall(text string) bool {
	for _, m := range nullableStdlibCallMarkers {
		if strings.Contains(text, m) {
			return true
		}
	}
	return false
}

func flatCallExpressionMethodSpan(file *scanner.File, idx uint32, methodName string) (int, int, bool) {
	if file == nil || idx == 0 || methodName == "" {
		return 0, 0, false
	}
	text := file.FlatNodeText(idx)
	needle := "." + methodName
	pos := strings.LastIndex(text, needle)
	if pos < 0 {
		return 0, 0, false
	}
	start := int(file.FlatStartByte(idx)) + pos + 1
	return start, start + len(methodName), true
}

// nonNullFactoryCalls are call prefixes that always produce non-null collections/strings.
var nonNullFactoryCalls = []string{
	"listOf(", "setOf(", "mapOf(",
	"mutableListOf(", "mutableSetOf(", "mutableMapOf(",
	"sequenceOf(", "emptyList(", "emptySet(", "emptyMap(",
}

func (r *UselessCallOnNotNullRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if file.FlatType(idx) != "call_expression" {
		return nil
	}
	navExpr, args := flatCallExpressionParts(file, idx)
	if navExpr != 0 {
		methodName := flatNavigationExpressionLastIdentifier(file, navExpr)
		replacement, isUseless := uselessNullCalls[methodName]
		if isUseless {
			receiverNode := file.FlatNamedChild(navExpr, 0)
			if receiverNode != 0 {
				nonNull := false
				recType := file.FlatType(receiverNode)
				if recType == "string_literal" || recType == "line_string_literal" || recType == "multi_line_string_literal" {
					nonNull = true
				} else if recType == "call_expression" {
					callText := file.FlatNodeText(receiverNode)
					for _, prefix := range nonNullFactoryCalls {
						if strings.HasPrefix(callText, prefix) {
							nonNull = true
							break
						}
					}
				}
				if recType == "navigation_expression" {
					return nil
				}
				recText := file.FlatNodeText(receiverNode)
				if strings.Contains(recText, "?.") {
					return nil
				}
				if !nonNull && r.resolver != nil {
					resolved := r.resolver.ResolveFlatNode(receiverNode, file)
					if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
						validTypes := orEmptyValidTypes
						if methodName == "isNullOrEmpty" || methodName == "isNullOrBlank" {
							validTypes = isNullOrValidTypes
						}
						if !validTypes[resolved.Name] {
							return nil
						}
						if !resolved.IsNullable() {
							nonNull = true
						}
					}
				}
				if nonNull {
					var msg string
					if replacement == "" {
						msg = fmt.Sprintf("Useless call to %s on non-null type. The value is already non-null.", methodName)
					} else {
						msg = fmt.Sprintf("Replace %s with %s — the receiver is non-null.", methodName, replacement)
					}
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
					if replacement == "" {
						if start, _, ok := flatCallExpressionMethodSpan(file, idx, methodName); ok {
							f.Fix = &scanner.Fix{
								ByteMode:    true,
								StartByte:   start - 1,
								EndByte:     int(file.FlatEndByte(idx)),
								Replacement: "",
							}
						}
					} else if start, end, ok := flatCallExpressionMethodSpan(file, idx, methodName); ok {
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   start,
							EndByte:     end,
							Replacement: replacement,
						}
					}
					return []scanner.Finding{f}
				}
			}
		}
	}
	if args != 0 && r.resolver != nil {
		calleeName := flatCallExpressionName(file, idx)
		replacementName, ok := ofNotNullReplacements[calleeName]
		if !ok {
			return nil
		}
		allNonNull := true
		argCount := 0
		for i := 0; i < file.FlatChildCount(args); i++ {
			va := file.FlatChild(args, i)
			if file.FlatType(va) != "value_argument" {
				continue
			}
			argCount++
			expr := flatValueArgumentExpression(file, va)
			if expr == 0 {
				allNonNull = false
				break
			}
			exprText := file.FlatNodeText(expr)
			if file.FlatType(expr) == "spread_expression" || strings.Contains(exprText, "?.") || file.FlatType(expr) == "navigation_expression" || containsNullableStdlibCall(exprText) {
				allNonNull = false
				break
			}
			resolved := r.resolver.ResolveFlatNode(expr, file)
			if resolved == nil || resolved.Kind == typeinfer.TypeUnknown || resolved.IsNullable() {
				allNonNull = false
				break
			}
		}
		if allNonNull && argCount > 0 {
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				fmt.Sprintf("Replace %s with %s — all arguments are non-null.", calleeName, replacementName))
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(idx)),
				EndByte:     int(file.FlatEndByte(idx)),
				Replacement: replacementName,
			}
			return []scanner.Finding{f}
		}
	}
	return nil
}
