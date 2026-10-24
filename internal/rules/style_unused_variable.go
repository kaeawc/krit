package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnusedVariableRule detects local variables that are never used.
type UnusedVariableRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames *regexp.Regexp
}

// Confidence reports a tier-2 (medium) base confidence. The rule uses
// scope-aware AST reference matching without type inference, so it skips
// unresolved local cases instead of emitting high-confidence findings.
func (r *UnusedVariableRule) Confidence() float64 { return api.ConfidenceMedium }

type unusedVariableReferenceMatch uint8

const (
	unusedVariableNoMatch unusedVariableReferenceMatch = iota
	unusedVariableMatchesTarget
	unusedVariableMatchesOtherDeclaration
	unusedVariableUnknownMatch
)

type unusedVariableDecl struct {
	name     string
	stmt     uint32
	nameNode uint32
	emitNode uint32
	scope    uint32
}

var unusedVariableClassHeaderBeforeConstructorPattern = regexp.MustCompile(`\bclass\s+[A-Za-z_][A-Za-z0-9_]*[^{};=]*$`)

func unusedVariableDeclaration(file *scanner.File, idx uint32) (unusedVariableDecl, bool) {
	var target unusedVariableDecl
	if file == nil || idx == 0 {
		return target, false
	}
	nodeType := file.FlatType(idx)
	stmt := idx
	nameNode := uint32(0)
	switch nodeType {
	case "property_declaration":
		if strings.Contains(file.FlatNodeText(idx), "@JvmField") {
			return target, false
		}
		if unusedVariablePropertyHasVisibilityModifier(file, idx) {
			return target, false
		}
		if !strings.Contains(file.FlatNodeText(idx), "=") {
			return target, false
		}
		if _, ok := file.FlatFindChild(idx, "multi_variable_declaration"); ok {
			return target, false
		}
		if file.FlatHasModifier(idx, "override") || unusedVariablePropertyHasAccessor(file, idx) {
			return target, false
		}
		varDecl, _ := file.FlatFindChild(idx, "variable_declaration")
		if varDecl == 0 {
			return target, false
		}
		nameNode, _ = file.FlatFindChild(varDecl, "simple_identifier")
	case "variable_declaration":
		parent, ok := file.FlatParent(idx)
		if !ok || file.FlatType(parent) != "multi_variable_declaration" {
			return target, false
		}
		grand, ok := file.FlatParent(parent)
		if !ok {
			return target, false
		}
		switch file.FlatType(grand) {
		case "property_declaration":
			stmt = grand
			nameNode, _ = file.FlatFindChild(idx, "simple_identifier")
		case "for_statement":
			// `for ((a, b) in xs) { ... }`: bindings live only inside the
			// loop body. The pattern (multi_variable_declaration) sits in
			// the for header, so the scope is the for's control body,
			// and the "statement end" for filtering pre-decl refs is the
			// end of the pattern itself.
			stmt = parent
			nameNode, _ = file.FlatFindChild(idx, "simple_identifier")
		default:
			return target, false
		}
	default:
		return target, false
	}
	if nameNode == 0 {
		return target, false
	}
	name := unusedVariableIdentifierName(file, nameNode)
	if name == "" {
		return target, false
	}
	if unusedVariableIsConstructorBodyParseArtifact(file, stmt) {
		return target, false
	}
	if !file.FlatHasAncestorOfType(stmt, "function_body") &&
		!file.FlatHasAncestorOfType(stmt, "lambda_literal") &&
		!file.FlatHasAncestorOfType(stmt, "anonymous_initializer") &&
		!file.FlatHasAncestorOfType(stmt, "secondary_constructor") {
		return target, false
	}
	if !unusedVariableIsLocalProperty(file, stmt) {
		return target, false
	}
	scope, ok := unusedVariableDeclScope(file, stmt)
	if !ok {
		return target, false
	}
	return unusedVariableDecl{
		name:     name,
		stmt:     stmt,
		nameNode: nameNode,
		emitNode: idx,
		scope:    scope,
	}, true
}

// unusedVariableDeclScope returns the lexical scope a declaration's
// references must fall inside. For a for-loop destructuring pattern the
// scope is the loop's control body; for plain locals it is the nearest
// statements / function_body / lambda block.
func unusedVariableDeclScope(file *scanner.File, stmt uint32) (uint32, bool) {
	if file.FlatType(stmt) == "multi_variable_declaration" {
		if forStmt, ok := file.FlatParent(stmt); ok && file.FlatType(forStmt) == "for_statement" {
			if body, ok := file.FlatFindChild(forStmt, "control_structure_body"); ok {
				return body, true
			}
			return 0, false
		}
	}
	return unusedVariableLexicalScope(file, stmt)
}

func unusedVariablePropertyHasVisibilityModifier(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	return file.FlatHasModifier(idx, "public") ||
		file.FlatHasModifier(idx, "internal") ||
		file.FlatHasModifier(idx, "protected") ||
		file.FlatHasModifier(idx, "private")
}

func unusedVariablePropertyHasAccessor(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "property_declaration" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(idx, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "getter", "setter", "property_declaration_body":
			found = true
		}
	})
	return found
}

func unusedVariableIsConstructorBodyParseArtifact(file *scanner.File, idx uint32) bool {
	for a, ok := file.FlatParent(idx); ok; a, ok = file.FlatParent(a) {
		switch file.FlatType(a) {
		case "call_expression":
			if flatCallNameAny(file, a) != "constructor" {
				continue
			}
			if unusedVariableCallNestedInExecutableScope(file, a) {
				return false
			}
			start := int(file.FlatStartByte(a))
			if start <= 0 || start > len(file.Content) {
				return false
			}
			prefixStart := start - 500
			if prefixStart < 0 {
				prefixStart = 0
			}
			prefix := string(file.Content[prefixStart:start])
			if unusedVariableClassHeaderBeforeConstructorPattern.MatchString(prefix) {
				return true
			}
			return false
		case "function_declaration", "anonymous_function":
			return false
		case "source_file":
			return false
		}
	}
	return false
}

func unusedVariableCallNestedInExecutableScope(file *scanner.File, call uint32) bool {
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "anonymous_function":
			return true
		case "lambda_literal":
			return true
		case "source_file":
			return false
		}
	}
	return false
}

func unusedVariableIsLocalProperty(file *scanner.File, idx uint32) bool {
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	parentType := file.FlatType(parent)
	if parentType == "source_file" ||
		parentType == "class_body" || parentType == "enum_class_body" ||
		parentType == "companion_object" || parentType == "object_declaration" ||
		parentType == "class_member_declaration" || parentType == "class_member_declarations" {
		return false
	}
	for a, ok := file.FlatParent(idx); ok; a, ok = file.FlatParent(a) {
		t := file.FlatType(a)
		if t == "delegation_specifier" || t == "explicit_delegation" {
			return false
		}
		if t == "function_body" || t == "function_declaration" ||
			t == "anonymous_function" ||
			t == "control_structure_body" || t == "anonymous_initializer" ||
			t == "secondary_constructor" {
			return true
		}
		if t == "lambda_literal" {
			// A lambda_literal is only a real local scope when it is a
			// lambda *expression* — i.e., its parent is a call/argument-like
			// context. Tree-sitter sometimes emits a lambda_literal node when
			// recovering from a malformed class header (e.g. `class Foo\n
			// private constructor(...)` parses the class body as a lambda
			// inside an expression). In that case the "properties" inside
			// are real class members, not locals.
			if !unusedVariableLambdaIsExpression(file, a) {
				return false
			}
			return true
		}
		if t == "class_body" || t == "enum_class_body" ||
			t == "companion_object" || t == "object_declaration" ||
			t == "class_member_declaration" || t == "class_member_declarations" ||
			t == "class_declaration" || t == "source_file" {
			return false
		}
	}
	return false
}

// unusedVariableUnaryContinuationUsesName reports whether the target's
// initializer ends with `... + name` or `... - name` where the operator
// starts on a new line *after* the preceding operand ends — the syntactic
// shape of a user-intended unary-prefix expression statement that the
// parser bound into the previous declaration's initializer.
func unusedVariableUnaryContinuationUsesName(file *scanner.File, target unusedVariableDecl) bool {
	if file == nil || target.stmt == 0 || target.name == "" {
		return false
	}
	if file.FlatType(target.stmt) != "property_declaration" {
		return false
	}
	found := false
	file.FlatWalkNodes(target.stmt, "additive_expression", func(node uint32) {
		if found || file.FlatChildCount(node) != 3 {
			return
		}
		lhs, op, rhs := file.FlatChild(node, 0), file.FlatChild(node, 1), file.FlatChild(node, 2)
		opType := file.FlatType(op)
		if opType != "+" && opType != "-" {
			return
		}
		lhsEndRow := file.FlatRow(lhs) + strings.Count(file.FlatNodeText(lhs), "\n")
		if file.FlatRow(op) <= lhsEndRow {
			return
		}
		// The unary-prefixed continuation `+expr` was bound by tree-sitter into
		// the declaration's initializer as `lhs <op> rhs`. The target name may be
		// the entire operand (`+factoryCall`) or nested inside it as a call
		// argument or a reference within a trailing lambda (`+render(typeKey)`,
		// `+apply { instance.append(...) }`). Treat any genuine reference to the
		// name anywhere in the operand subtree as a use.
		if unusedVariableContinuationOperandUsesName(file, rhs, target.name) {
			found = true
		}
	})
	return found
}

// unusedVariableContinuationOperandUsesName reports whether `operand` — the
// right-hand side of a unary-prefixed continuation that tree-sitter folded
// into a preceding declaration's initializer — contains a genuine reference to
// `name`. A bare-identifier operand is matched directly; otherwise the operand
// subtree is walked for an identifier (or string-template reference) that names
// the target in a real reference position (not a label, type, or declaration).
func unusedVariableContinuationOperandUsesName(file *scanner.File, operand uint32, name string) bool {
	if file == nil || operand == 0 || name == "" {
		return false
	}
	if file.FlatType(operand) == "simple_identifier" {
		return unusedVariableIdentifierName(file, operand) == name
	}
	found := false
	for _, nodeType := range []string{"simple_identifier", "interpolated_identifier", "line_str_ref", "multi_line_str_ref"} {
		file.FlatWalkNodes(operand, nodeType, func(candidate uint32) {
			if found {
				return
			}
			if unusedVariableReferenceName(file, candidate) != name {
				return
			}
			if file.FlatType(candidate) == "simple_identifier" {
				isReference, known := unusedVariableIsReferenceIdentifier(file, candidate)
				if !known || !isReference {
					return
				}
			}
			found = true
		})
		if found {
			break
		}
	}
	return found
}

// unusedVariableScopeHasParseError reports whether `scope` (or any of its
// descendants) is a tree-sitter ERROR node. Rules that need full visibility
// into a scope (so that references can attach to declarations) must abstain
// when the parse is incomplete, since references can land in orphan subtrees.
//
// Backed by a per-file fact: the set of ERROR start bytes. The common case
// (no parse errors in the file) short-circuits to O(1).
func unusedVariableScopeHasParseError(file *scanner.File, scope uint32) bool {
	if file == nil || scope == 0 {
		return false
	}
	errorStarts := filefacts.FileFact(fileFactsCache(), file, slotUnusedVariableHasParseError, func() []uint32 {
		var starts []uint32
		file.FlatWalkAllNodes(0, func(candidate uint32) {
			if file.FlatType(candidate) == "ERROR" {
				starts = append(starts, file.FlatStartByte(candidate))
			}
		})
		return starts
	})
	if len(errorStarts) == 0 {
		return false
	}
	scopeStart := file.FlatStartByte(scope)
	scopeEnd := file.FlatEndByte(scope)
	for _, start := range errorStarts {
		if start >= scopeStart && start < scopeEnd {
			return true
		}
	}
	return false
}

// unusedVariableLambdaIsExpression reports whether a lambda_literal node is
// in a position where Kotlin actually treats it as a lambda expression
// (function argument, property delegate, function body, etc.) rather than
// being a parse-error artifact from a malformed declaration.
func unusedVariableLambdaIsExpression(file *scanner.File, lambda uint32) bool {
	parent, ok := file.FlatParent(lambda)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "annotated_lambda", "call_suffix", "value_arguments", "value_argument",
		"function_value_parameters", "function_body", "property_delegate",
		"control_structure_body", "when_entry", "statements":
		return true
	}
	return false
}

func unusedVariableLexicalScope(file *scanner.File, stmt uint32) (uint32, bool) {
	scope, ok := file.FlatParent(stmt)
	if !ok {
		return 0, false
	}
	for {
		switch file.FlatType(scope) {
		case "statements", "function_body", "lambda_literal", "control_structure_body", "source_file":
			return scope, true
		}
		next, ok := file.FlatParent(scope)
		if !ok {
			return 0, false
		}
		scope = next
	}
}

func unusedVariableUsage(file *scanner.File, target unusedVariableDecl) (used bool, unknown bool) {
	// If the target's lookup scope contains parse errors (e.g. unsupported
	// Kotlin syntax such as `when` entries with `is X if (cond)` guards), the
	// tree-sitter tree can drop legit references into ERROR/orphan subtrees.
	// Abstain rather than risk a false positive.
	if unusedVariableScopeHasParseError(file, target.scope) {
		return false, true
	}
	// Kotlin allows a unary-prefixed expression statement (e.g. a DSL builder
	// `+factory`) to follow another expression on the next line. tree-sitter
	// (and Kotlin's own grammar) bind that into a binary additive_expression
	// inside the previous statement, so a reference like `+factoryCall` after
	// `val factoryCall = when { ... }` looks textually as if `factoryCall` is
	// referenced inside its own initializer. Recognize that pattern as a use
	// to avoid a false positive.
	if unusedVariableUnaryContinuationUsesName(file, target) {
		return true, false
	}
	stmtEnd := file.FlatEndByte(target.stmt)
	for _, nodeType := range []string{"simple_identifier", "interpolated_identifier", "line_str_ref", "multi_line_str_ref"} {
		file.FlatWalkNodes(target.scope, nodeType, func(candidate uint32) {
			if used || unknown || file.FlatStartByte(candidate) <= stmtEnd {
				return
			}
			switch unusedVariableResolveReference(file, target, candidate) {
			case unusedVariableMatchesTarget:
				used = true
			case unusedVariableUnknownMatch:
				unknown = true
			}
		})
		if used || unknown {
			break
		}
	}
	return used, unknown
}

func unusedVariableResolveReference(file *scanner.File, target unusedVariableDecl, ref uint32) unusedVariableReferenceMatch {
	if unusedVariableReferenceName(file, ref) != target.name {
		return unusedVariableNoMatch
	}
	if file.FlatType(ref) == "simple_identifier" {
		isReference, known := unusedVariableIsReferenceIdentifier(file, ref)
		if !known {
			return unusedVariableUnknownMatch
		}
		if !isReference {
			return unusedVariableNoMatch
		}
	}
	if unusedVariableImplicitItShadows(file, target, ref) {
		return unusedVariableMatchesOtherDeclaration
	}
	decl, known := unusedVariableNearestDeclaration(file, target, ref)
	if !known {
		return unusedVariableUnknownMatch
	}
	if decl == target.nameNode {
		return unusedVariableMatchesTarget
	}
	return unusedVariableMatchesOtherDeclaration
}

func unusedVariableIdentifierName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	return strings.Trim(file.FlatNodeText(idx), "`")
}

func unusedVariableReferenceName(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "simple_identifier":
		return unusedVariableIdentifierName(file, idx)
	case "interpolated_identifier", "line_str_ref", "multi_line_str_ref":
		text := strings.TrimSpace(file.FlatNodeText(idx))
		text = strings.TrimPrefix(text, "$")
		text = strings.TrimPrefix(text, "{")
		text = strings.TrimSuffix(text, "}")
		return strings.Trim(text, "`")
	default:
		return ""
	}
}

func unusedVariableIsReferenceIdentifier(file *scanner.File, idx uint32) (isReference bool, known bool) {
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false, false
	}
	switch file.FlatType(parent) {
	case "variable_declaration", "parameter", "function_declaration", "class_declaration",
		"object_declaration", "type_alias", "import_header", "package_header",
		"user_type", "nullable_type", "type_reference", "value_argument_label",
		"navigation_suffix", "annotation", "label", "lambda_parameters":
		return false, true
	}
	if next, ok := file.FlatNextSibling(idx); ok && file.FlatType(next) == "=" {
		if file.FlatType(parent) == "value_argument" {
			return false, true
		}
	}
	for a, ok := file.FlatParent(idx); ok; a, ok = file.FlatParent(a) {
		switch file.FlatType(a) {
		case "user_type", "nullable_type", "type_reference", "type_arguments",
			"type_projection", "function_type", "receiver_type", "annotation",
			"import_header", "package_header":
			return false, true
		case "property_declaration", "function_declaration", "lambda_literal",
			"statements", "control_structure_body", "function_body", "source_file":
			return true, true
		}
	}
	return false, false
}

func unusedVariableNearestDeclaration(file *scanner.File, target unusedVariableDecl, ref uint32) (uint32, bool) {
	refStart := file.FlatStartByte(ref)
	best := uint32(0)
	bestStart := uint32(0)
	// Seed with the target binding so it can match references in `target.scope`
	// even when the declaration itself sits outside the scope (e.g. a for-loop
	// destructuring pattern in the loop header). A later in-scope shadow with
	// a larger start byte still wins via the `start < bestStart` check below.
	if target.nameNode != 0 && file.FlatStartByte(target.nameNode) <= refStart {
		best = target.nameNode
		bestStart = file.FlatStartByte(target.nameNode)
	}
	file.FlatWalkAllNodes(target.scope, func(candidate uint32) {
		if file.FlatStartByte(candidate) > refStart {
			return
		}
		nameNode := unusedVariableDeclarationNameNode(file, candidate, target.name)
		if nameNode == 0 {
			return
		}
		if nameNode == ref {
			return
		}
		stmt := unusedVariableShadowDeclarationStatement(file, candidate)
		if stmt != 0 && unusedVariableNodeContains(file, stmt, ref) {
			return
		}
		scope := unusedVariableShadowScope(file, candidate)
		if scope == 0 || !unusedVariableNodeContains(file, scope, ref) {
			return
		}
		start := file.FlatStartByte(nameNode)
		if best != 0 && start < bestStart {
			return
		}
		best = nameNode
		bestStart = start
	})
	return best, best != 0
}

func unusedVariableImplicitItShadows(file *scanner.File, target unusedVariableDecl, ref uint32) bool {
	if target.name != "it" {
		return false
	}
	for a, ok := file.FlatParent(ref); ok && a != target.scope; a, ok = file.FlatParent(a) {
		if file.FlatType(a) != "lambda_literal" {
			continue
		}
		if file.FlatStartByte(a) <= file.FlatStartByte(target.stmt) {
			continue
		}
		if _, ok := file.FlatFindChild(a, "lambda_parameters"); !ok {
			return true
		}
	}
	return false
}

func unusedVariableDeclarationNameNode(file *scanner.File, idx uint32, name string) uint32 {
	switch file.FlatType(idx) {
	case "property_declaration":
		if _, ok := file.FlatFindChild(idx, "multi_variable_declaration"); ok {
			return 0
		}
		varDecl, _ := file.FlatFindChild(idx, "variable_declaration")
		if varDecl == 0 {
			return 0
		}
		ident, _ := file.FlatFindChild(varDecl, "simple_identifier")
		if ident != 0 && unusedVariableIdentifierName(file, ident) == name {
			return ident
		}
		return 0
	case "variable_declaration", "parameter":
		ident, _ := file.FlatFindChild(idx, "simple_identifier")
		if ident != 0 && unusedVariableIdentifierName(file, ident) == name {
			return ident
		}
		return 0
	default:
		return 0
	}
}

func unusedVariableShadowDeclarationStatement(file *scanner.File, idx uint32) uint32 {
	switch file.FlatType(idx) {
	case "property_declaration":
		return idx
	case "variable_declaration":
		if prop, ok := flatEnclosingAncestor(file, idx, "property_declaration"); ok {
			return prop
		}
		return idx
	case "parameter":
		return idx
	default:
		return 0
	}
}

func unusedVariableShadowScope(file *scanner.File, idx uint32) uint32 {
	switch file.FlatType(idx) {
	case "property_declaration", "variable_declaration":
		scope, ok := unusedVariableLexicalScope(file, idx)
		if ok {
			return scope
		}
	case "parameter":
		for a, ok := file.FlatParent(idx); ok; a, ok = file.FlatParent(a) {
			switch file.FlatType(a) {
			case "lambda_literal", "function_declaration", "anonymous_function":
				return a
			case "source_file":
				return 0
			}
		}
	}
	return 0
}

func unusedVariableNodeContains(file *scanner.File, ancestor, idx uint32) bool {
	return file.FlatStartByte(ancestor) <= file.FlatStartByte(idx) &&
		file.FlatEndByte(ancestor) >= file.FlatEndByte(idx)
}
