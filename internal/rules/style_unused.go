package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnusedImportRule detects import statements where the imported name is not used.
type UnusedImportRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule.
// Detection uses a structural per-file reference-name index, but it does not
// fully resolve ambiguous imports.
func (r *UnusedImportRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnusedImportRule) hasReferenceName(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	names := filefacts.FileFact(fileFactsCache(), file, slotUnusedImportRefs, func() map[string]struct{} {
		return collectReferenceNamesOutsideImports(file)
	})
	_, ok := names[name]
	return ok
}

func collectReferenceNamesOutsideImports(file *scanner.File) map[string]struct{} {
	names := make(map[string]struct{})
	file.FlatWalkAllNodes(0, func(n uint32) {
		if nodeHasAncestorTypeFlat(file, n, "import_header", "package_header") {
			return
		}
		t := file.FlatType(n)
		if t != "simple_identifier" && t != "type_identifier" && t != "navigation_expression" && t != "user_type" {
			return
		}
		if name := semantics.ReferenceName(file, n); name != "" {
			// `foo` and foo resolve to the same symbol; canonicalize for lookup.
			names[strings.Trim(name, "`")] = struct{}{}
		}
	})
	return names
}

// UnusedParameterRule detects function parameters that are never used in the body.
type UnusedParameterRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames *regexp.Regexp
}

// Confidence reports a tier-1 (high) base confidence. Parameter usage is
// detected from reference-shaped AST identifiers with local shadowing handled
// structurally, so comments, strings, and substring collisions do not count as
// usage.
func (r *UnusedParameterRule) Confidence() float64 { return api.ConfidenceVeryHigh }

type unusedParameterReferenceMatch uint8

const (
	unusedParameterNoMatch unusedParameterReferenceMatch = iota
	unusedParameterMatchesParam
	unusedParameterMatchesOtherDeclaration
	unusedParameterUnknownMatch
)

func unusedParameterUsageFlat(file *scanner.File, scope uint32, paramIdx uint32, paramName string, paramIsFunctionType bool) (used bool, unknown bool) {
	if file == nil || scope == 0 || paramIdx == 0 || paramName == "" {
		return false, false
	}
	for _, nodeType := range []string{"simple_identifier", "interpolated_identifier", "line_str_ref", "multi_line_str_ref"} {
		file.FlatWalkNodes(scope, nodeType, func(candidate uint32) {
			if used || unknown {
				return
			}
			switch unusedParameterResolveReferenceFlat(file, scope, paramIdx, candidate, paramName, paramIsFunctionType) {
			case unusedParameterMatchesParam:
				used = true
			case unusedParameterUnknownMatch:
				unknown = true
			}
		})
		if used || unknown {
			break
		}
	}
	if !used && !unknown && unusedParameterErrorSubtreeMentionsNameFlat(file, scope, paramName) {
		unknown = true
	}
	return used, unknown
}

// unusedParameterErrorSubtreeMentionsNameFlat reports whether any ERROR node
// inside scope contains a descendant whose text equals name. Tree-sitter-kotlin
// mis-parses identifiers that collide with Kotlin soft keywords
// (`annotation`, `field`, `value`, `dynamic`, etc.) when they appear in
// receiver position before `.`, classifying the name under an ERROR subtree
// instead of a simple_identifier — so the body walk over simple_identifier
// nodes never sees the use. Treat any such mention as unknown to suppress the
// false positive.
func unusedParameterErrorSubtreeMentionsNameFlat(file *scanner.File, scope uint32, name string) bool {
	if file == nil || scope == 0 || name == "" {
		return false
	}
	// Posting-list-backed fast path: most Kotlin files have no ERROR nodes,
	// so the per-parameter recursive scope walk below can be skipped entirely.
	errorTypeID, ok := scanner.LookupFlatNodeType("ERROR")
	if !ok || file.FlatTree == nil || len(file.FlatTree.NodesOfType(errorTypeID)) == 0 {
		return false
	}
	mentioned := false
	file.FlatWalkNodes(scope, "ERROR", func(errNode uint32) {
		if mentioned {
			return
		}
		file.FlatWalkAllNodes(errNode, func(child uint32) {
			if mentioned || child == errNode {
				return
			}
			if file.FlatNodeTextEquals(child, name) {
				mentioned = true
			}
		})
	})
	return mentioned
}

func unusedParameterResolveReferenceFlat(file *scanner.File, scope uint32, paramIdx uint32, ref uint32, paramName string, paramIsFunctionType bool) unusedParameterReferenceMatch {
	if unusedParameterReferenceNameFlat(file, ref) != paramName {
		return unusedParameterNoMatch
	}
	if file.FlatType(ref) == "simple_identifier" {
		reference, known := unusedParameterIdentifierIsReferenceFlat(file, ref, paramIsFunctionType)
		if !known {
			return unusedParameterUnknownMatch
		}
		if !reference {
			return unusedParameterNoMatch
		}
	}
	if unusedParameterReferenceShadowedFlat(file, scope, ref, paramName) {
		return unusedParameterMatchesOtherDeclaration
	}
	if unusedParameterSameFileDeclarationMatchFlat(file, scope, paramIdx, ref, paramName) {
		return unusedParameterMatchesParam
	}
	return unusedParameterUnknownMatch
}

func unusedParameterReferenceNameFlat(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "simple_identifier":
		return strings.Trim(file.FlatNodeText(idx), "`")
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

func unusedParameterIdentifierIsReferenceFlat(file *scanner.File, ident uint32, paramIsFunctionType bool) (isReference bool, known bool) {
	parent, ok := file.FlatParent(ident)
	if !ok {
		return false, false
	}
	if file.FlatType(parent) == "navigation_suffix" {
		return paramIsFunctionType && unusedParameterNavigationSuffixIsCallFlat(file, parent), true
	}
	switch file.FlatType(parent) {
	case "parameter", "class_parameter", "variable_declaration",
		"function_declaration", "class_declaration", "object_declaration",
		"type_identifier", "user_type", "nullable_type", "function_type",
		"type_parameter", "type_parameters", "function_value_parameters",
		"lambda_parameters", "value_argument_label", "import_header",
		"package_header":
		return false, true
	case "value_argument":
		if file.FlatNamedChildCount(parent) >= 2 && file.FlatNamedChild(parent, 0) == ident {
			return false, true
		}
	}
	for cur, ok := file.FlatParent(ident); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "user_type", "nullable_type", "function_type", "type_parameter", "type_parameters":
			return false, true
		case "function_body", "lambda_literal", "function_declaration":
			return true, true
		}
	}
	return false, false
}

func unusedParameterNavigationSuffixIsCallFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "call_suffix", "value_arguments":
			return true
		}
	}
	if nav, ok := file.FlatParent(idx); ok && file.FlatType(nav) == "navigation_expression" {
		if call, ok := file.FlatParent(nav); ok && file.FlatType(call) == "call_expression" {
			return true
		}
	}
	return strings.Contains(file.FlatNodeText(idx), "(")
}

func unusedParameterReferenceShadowedFlat(file *scanner.File, body uint32, ref uint32, name string) bool {
	for cur, ok := file.FlatParent(ref); ok && cur != body; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "lambda_literal":
			if unusedParameterLambdaDeclaresNameFlat(file, cur, name) {
				return true
			}
		case "for_statement":
			if unusedParameterForDeclaresNameFlat(file, cur, name) {
				return true
			}
		case "catch_block", "catch_clause":
			if unusedParameterCatchDeclaresNameFlat(file, cur, name) {
				return true
			}
		case "function_declaration", "anonymous_function":
			if unusedParameterFunctionDeclaresNameFlat(file, cur, name) {
				return true
			}
		}
	}
	return unusedParameterPriorLocalDeclarationShadowsFlat(file, body, ref, name)
}

func unusedParameterFunctionDeclaresNameFlat(file *scanner.File, fn uint32, name string) bool {
	if file == nil || fn == 0 || name == "" {
		return false
	}
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		return false
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "parameter" && extractIdentifierFlat(file, child) == name {
			return true
		}
	}
	return false
}

func unusedParameterLambdaDeclaresNameFlat(file *scanner.File, lambda uint32, name string) bool {
	params, ok := file.FlatFindChild(lambda, "lambda_parameters")
	if !ok {
		return name == "it"
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "simple_identifier":
			if file.FlatNodeTextEquals(child, name) {
				return true
			}
		case "variable_declaration":
			if extractIdentifierFlat(file, child) == name {
				return true
			}
		}
	}
	return false
}

// unusedParameterForDeclaresNameFlat reports whether the for-statement's loop
// variable binds name. Only children positioned before the `in` keyword
// count — the iterable expression that follows `in` is also a direct child
// and must not be treated as a declaration.
func unusedParameterForDeclaresNameFlat(file *scanner.File, forStmt uint32, name string) bool {
	for child := file.FlatFirstChild(forStmt); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) && file.FlatType(child) == "in" {
			return false
		}
		switch file.FlatType(child) {
		case "simple_identifier":
			if file.FlatNodeTextEquals(child, name) {
				return true
			}
		case "variable_declaration", "multi_variable_declaration":
			if unusedParameterDeclarationNodeHasNameFlat(file, child, name) {
				return true
			}
		}
	}
	return false
}

func unusedParameterCatchDeclaresNameFlat(file *scanner.File, catchNode uint32, name string) bool {
	for child := file.FlatFirstChild(catchNode); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "parameter" && extractIdentifierFlat(file, child) == name {
			return true
		}
		if file.FlatType(child) == "simple_identifier" && file.FlatNodeTextEquals(child, name) {
			return true
		}
	}
	return false
}

func unusedParameterPriorLocalDeclarationShadowsFlat(file *scanner.File, body uint32, ref uint32, name string) bool {
	refStart := file.FlatStartByte(ref)
	shadowed := false
	file.FlatWalkAllNodes(body, func(candidate uint32) {
		if shadowed || file.FlatStartByte(candidate) >= refStart {
			return
		}
		switch file.FlatType(candidate) {
		case "property_declaration":
			if unusedParameterNodeContainsFlat(file, candidate, ref) {
				return
			}
			if unusedParameterDeclarationNodeHasNameFlat(file, candidate, name) &&
				unusedParameterDeclarationScopeContainsRefFlat(file, body, candidate, ref) {
				shadowed = true
			}
		case "function_declaration":
			if extractIdentifierFlat(file, candidate) == name &&
				unusedParameterDeclarationScopeContainsRefFlat(file, body, candidate, ref) {
				shadowed = true
			}
		}
	})
	return shadowed
}

func unusedParameterDeclarationNodeHasNameFlat(file *scanner.File, node uint32, name string) bool {
	if extractIdentifierFlat(file, node) == name {
		return true
	}
	found := false
	file.FlatWalkAllNodes(node, func(candidate uint32) {
		if found || file.FlatType(candidate) != "variable_declaration" {
			return
		}
		if extractIdentifierFlat(file, candidate) == name {
			found = true
		}
	})
	return found
}

func unusedParameterDeclarationScopeContainsRefFlat(file *scanner.File, stop uint32, decl uint32, ref uint32) bool {
	scope := unusedParameterLexicalScopeFlat(file, stop, decl)
	return scope != 0 && unusedParameterNodeContainsFlat(file, scope, ref)
}

func unusedParameterSameFileDeclarationMatchFlat(file *scanner.File, body uint32, paramIdx uint32, ref uint32, name string) bool {
	return extractIdentifierFlat(file, paramIdx) == name &&
		unusedParameterNodeContainsFlat(file, body, ref) &&
		!unusedParameterReferenceShadowedFlat(file, body, ref, name)
}

func unusedParameterLexicalScopeFlat(file *scanner.File, stop uint32, node uint32) uint32 {
	for cur, ok := file.FlatParent(node); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "statements", "function_body", "control_structure_body",
			"lambda_literal", "catch_block", "catch_clause", "for_statement":
			return cur
		}
		if cur == stop {
			return cur
		}
	}
	return 0
}

func unusedParameterNodeContainsFlat(file *scanner.File, outer uint32, inner uint32) bool {
	return file.FlatStartByte(outer) <= file.FlatStartByte(inner) &&
		file.FlatEndByte(inner) <= file.FlatEndByte(outer)
}

func hasSiblingOverloadFlat(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 || name == "" {
		return false
	}
	parent, ok := file.FlatParent(idx)
	for ok && file.FlatType(parent) != "class_body" && file.FlatType(parent) != "source_file" &&
		file.FlatType(parent) != "class_member_declarations" {
		parent, ok = file.FlatParent(parent)
	}
	if !ok {
		return false
	}
	// Linear sibling walk via FirstChild/NextSib. The previous form used
	// FlatNamedChild(parent, i) in a for-i loop, which is O(k) per call and
	// O(N²) across the iteration. For files with many siblings under one
	// parent (generated code, Dagger modules with lots of @Binds methods)
	// this was a latent quadratic.
	for sib := file.FlatFirstChild(parent); sib != 0; sib = file.FlatNextSib(sib) {
		if !file.FlatIsNamed(sib) || sib == idx {
			continue
		}
		if file.FlatType(sib) != "function_declaration" {
			continue
		}
		if extractIdentifierFlat(file, sib) == name {
			return true
		}
	}
	return false
}

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
		if file.FlatType(rhs) != "simple_identifier" {
			return
		}
		if unusedVariableIdentifierName(file, rhs) != target.name {
			return
		}
		found = true
	})
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

// UnusedPrivateClassRule detects private classes that are never referenced.
type UnusedPrivateClassRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedPrivateClassRule) Confidence() float64 { return api.ConfidenceMedium }

// entryPointAnnotationNames lists annotation names that mark a declaration as
// a framework entry point (called via reflection, preview tooling, test
// runners, etc.). Private declarations with these annotations should NOT be
// flagged as unused.
var entryPointAnnotationNames = map[string]bool{
	"Preview":           true, // androidx.compose.ui.tooling.preview.Preview
	"ComposePreview":    true,
	"PreviewParameter":  true,
	"PreviewLightDark":  true,
	"DarkPreview":       true,
	"LightPreview":      true,
	"NightPreview":      true,
	"DayPreview":        true,
	"Test":              true, // JUnit @Test
	"ParameterizedTest": true,
	"BeforeEach":        true,
	"AfterEach":         true,
	"BeforeAll":         true,
	"AfterAll":          true,
	"Before":            true,
	"After":             true,
	"BeforeClass":       true,
	"AfterClass":        true,
	"ParameterizedRobolectricTestRunner.Parameters": true,
	"Parameters":    true,
	"Provides":      true, // Dagger
	"Binds":         true,
	"BindsInstance": true,
	"Module":        true,
	"JvmStatic":     true,
	"JvmName":       true,
	"JvmField":      true,
	// Reflection/proguard retention markers.
	"Keep":              true, // androidx.annotation.Keep
	"UsedByReflection":  true,
	"UsedByNative":      true,
	"VisibleForTesting": true, // accessed from test module
	"SerializedName":    true, // Gson/Moshi
	"JsonCreator":       true, // Jackson
	"JsonProperty":      true,
}

func flatAnnotationListContains(parentText string, childText string, name string) bool {
	text := childText
	text = strings.TrimPrefix(text, "@")
	if parenIdx := strings.Index(text, "("); parenIdx >= 0 {
		text = text[:parenIdx]
	}
	if colonIdx := strings.Index(text, ":"); colonIdx >= 0 {
		text = text[colonIdx+1:]
	}
	text = strings.TrimSpace(text)
	if dotIdx := strings.LastIndex(text, "."); dotIdx >= 0 {
		text = text[dotIdx+1:]
	}
	return text == name || (parentText == text && text == name)
}

func flatHasAnnotationNamed(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 {
		return false
	}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
		for i := 0; i < file.FlatChildCount(mods); i++ {
			child := file.FlatChild(mods, i)
			t := file.FlatType(child)
			if t != "annotation" && t != "modifier" {
				continue
			}
			if flatAnnotationListContains("", file.FlatNodeText(child), name) {
				return true
			}
		}
	}
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		t := file.FlatType(child)
		if t != "annotation" && t != "modifier" {
			continue
		}
		if flatAnnotationListContains("", file.FlatNodeText(child), name) {
			return true
		}
	}
	return false
}

func flatHasEntryPointAnnotation(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	if flatChildrenHaveEntryPointAnnotation(file, idx) {
		return true
	}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
		if flatChildrenHaveEntryPointAnnotation(file, mods) {
			return true
		}
	}
	return flatLeadingTextHasEntryPointAnnotation(file.FlatNodeText(idx))
}

func flatChildrenHaveEntryPointAnnotation(file *scanner.File, idx uint32) bool {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		t := file.FlatType(child)
		if t != "annotation" && t != "modifier" {
			continue
		}
		if isEntryPointAnnotationName(flatAnnotationSimpleName(file.FlatNodeText(child))) {
			return true
		}
	}
	return false
}

func flatAnnotationSimpleName(text string) string {
	text = strings.TrimPrefix(text, "@")
	if colonIdx := strings.Index(text, ":"); colonIdx >= 0 {
		text = text[colonIdx+1:]
	}
	if parenIdx := strings.Index(text, "("); parenIdx >= 0 {
		text = text[:parenIdx]
	}
	text = strings.TrimSpace(text)
	if dotIdx := strings.LastIndex(text, "."); dotIdx >= 0 {
		text = text[dotIdx+1:]
	}
	return text
}

func flatLeadingTextHasEntryPointAnnotation(text string) bool {
	if funIdx := strings.Index(text, "fun "); funIdx >= 0 {
		text = text[:funIdx]
	}
	for {
		at := strings.Index(text, "@")
		if at < 0 {
			return false
		}
		text = text[at+1:]
		end := 0
		for end < len(text) {
			ch := text[end]
			if (ch >= 'a' && ch <= 'z') ||
				(ch >= 'A' && ch <= 'Z') ||
				(ch >= '0' && ch <= '9') ||
				ch == '_' || ch == '.' || ch == ':' {
				end++
				continue
			}
			break
		}
		if end > 0 && isEntryPointAnnotationName(flatAnnotationSimpleName(text[:end])) {
			return true
		}
		text = text[end:]
	}
}

func isEntryPointAnnotationName(name string) bool {
	if entryPointAnnotationNames[name] {
		return true
	}
	// Compose projects commonly wrap @Preview in local annotations such as
	// @ConstellationPreviews, @PreviewAll, or @PreviewLightDarkDevices. Those
	// functions are discovered by tooling and should not be removed as unused.
	return strings.Contains(name, "Preview")
}

func privateOperatorFunctionHasSymbolUse(file *scanner.File, idx uint32, name string) bool {
	if file == nil || !file.FlatHasModifier(idx, "operator") {
		return false
	}
	symbol := kotlinOperatorSymbol(name)
	if symbol == "" {
		return false
	}
	start, end := int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx))
	if start < 0 || end < start || end > len(file.Content) {
		return false
	}
	before := string(file.Content[:start])
	after := string(file.Content[end:])
	return strings.Contains(before, symbol) || strings.Contains(after, symbol)
}

func kotlinOperatorSymbol(name string) string {
	switch name {
	case "times":
		return "*"
	default:
		return ""
	}
}

func flatHasFrameworkAnnotation(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	mods, _ := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return false
	}
	for i := 0; i < file.FlatChildCount(mods); i++ {
		child := file.FlatChild(mods, i)
		if file.FlatType(child) != "annotation" {
			continue
		}
		raw := file.FlatNodeText(child)
		text := strings.TrimPrefix(raw, "@")
		if colonIdx := strings.Index(text, ":"); colonIdx >= 0 {
			text = text[colonIdx+1:]
		}
		if parenIdx := strings.Index(text, "("); parenIdx >= 0 {
			text = text[:parenIdx]
		}
		text = strings.TrimSpace(text)
		if dotIdx := strings.LastIndex(text, "."); dotIdx >= 0 {
			text = text[dotIdx+1:]
		}
		if frameworkAnnotationNames[text] {
			return true
		}
		if text == "Suppress" || text == "SuppressWarnings" {
			if strings.Contains(raw, `"unused"`) ||
				strings.Contains(raw, `"UNUSED_PARAMETER"`) ||
				strings.Contains(raw, `"UNUSED_VARIABLE"`) ||
				strings.Contains(raw, `"UnusedPrivateProperty"`) ||
				strings.Contains(raw, `"UnusedPrivateMember"`) ||
				strings.Contains(raw, `"UnusedPrivateFunction"`) ||
				strings.Contains(raw, `"UnusedVariable"`) {
				return true
			}
		}
	}
	return false
}

// UnusedPrivateFunctionRule detects private functions that are never called.
type UnusedPrivateFunctionRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames *regexp.Regexp
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedPrivateFunctionRule) Confidence() float64 { return api.ConfidenceMedium }

// UnusedPrivatePropertyRule detects private properties that are never referenced.
type UnusedPrivatePropertyRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames *regexp.Regexp
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedPrivatePropertyRule) Confidence() float64 { return api.ConfidenceMedium }

// UnusedPrivateMemberRule is a combined check for unused private members.
type UnusedPrivateMemberRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames    *regexp.Regexp
	IgnoreAnnotated []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedPrivateMemberRule) Confidence() float64 { return api.ConfidenceMedium }
