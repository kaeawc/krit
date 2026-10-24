package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

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

// parameterHasUnusedSuppression reports whether the parameter at paramIdx
// carries a `@Suppress("unused")` / `@Suppress("UNUSED_PARAMETER")` annotation.
//
// In the Kotlin grammar a parameter's annotations parse as a
// `parameter_modifiers` node that precedes the `parameter` node inside
// `function_value_parameters`, so the annotation is a *sibling* of the
// parameter rather than part of its node text. Inspecting only the parameter
// node text therefore misses the suppression and produces a false positive.
// Check both the parameter node and its preceding `parameter_modifiers`
// sibling.
func parameterHasUnusedSuppression(file *scanner.File, paramIdx uint32) bool {
	if file == nil || paramIdx == 0 {
		return false
	}
	if parameterTextSuppressesUnused(file.FlatNodeText(paramIdx)) {
		return true
	}
	if prev, ok := file.FlatPrevSibling(paramIdx); ok && file.FlatType(prev) == "parameter_modifiers" {
		if parameterTextSuppressesUnused(file.FlatNodeText(prev)) {
			return true
		}
	}
	return false
}

func parameterTextSuppressesUnused(text string) bool {
	return strings.Contains(text, "@Suppress") &&
		(strings.Contains(text, `"unused"`) || strings.Contains(text, `"UNUSED_PARAMETER"`))
}

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
	if !used && !unknown {
		used = unusedParameterMisparsedReferenceFlat(file, scope, paramIdx, paramName)
	}
	if !used && !unknown && unusedParameterErrorSubtreeMentionsNameFlat(file, scope, paramName) {
		unknown = true
	}
	return used, unknown
}

// unusedParameterMisparsedReferenceFlat finds parameter reads that
// tree-sitter-kotlin does not expose as simple identifiers:
//
//   - the receiver of a bound callable reference or class literal
//     (`buffer::execute`, `screen::class`) parses as a type_identifier;
//   - `!isReady` / `!inProgress` in a when condition lexes as the `!is` /
//     `!in` operator glued to the rest of the name (`!is Ready`), producing
//     a type_test or range_test instead of a negated identifier.
func unusedParameterMisparsedReferenceFlat(file *scanner.File, scope uint32, paramIdx uint32, name string) bool {
	found := false
	check := func(ref uint32) {
		if !found && unusedParameterSameFileDeclarationMatchFlat(file, scope, paramIdx, ref, name) {
			found = true
		}
	}
	file.FlatWalkNodes(scope, "callable_reference", func(ref uint32) {
		receiver := file.FlatFirstChild(ref)
		if receiver == 0 || file.FlatType(receiver) != "type_identifier" ||
			strings.Trim(file.FlatNodeText(receiver), "`") != name {
			return
		}
		if next := file.FlatNextSib(receiver); next == 0 || file.FlatType(next) != "::" {
			return
		}
		check(receiver)
	})
	for _, testType := range []string{"type_test", "range_test"} {
		if found {
			break
		}
		file.FlatWalkNodes(scope, testType, func(test uint32) {
			if misLexed, ok := misLexedNegatedIdentifierFlat(file, test); ok && misLexed == name {
				check(test)
			}
		})
	}
	return found
}

// misLexedNegatedIdentifierFlat recovers the identifier from a type_test or
// range_test whose `!is` / `!in` operator token is immediately followed, with
// no whitespace, by the operand: tree-sitter-kotlin lexes `!isReady` as
// `!is Ready` and `!inProgress` as `!in Progress`, while Kotlin reads them as
// the negated identifiers `isReady` and `inProgress`.
func misLexedNegatedIdentifierFlat(file *scanner.File, test uint32) (string, bool) {
	op := file.FlatFirstChild(test)
	if op == 0 {
		return "", false
	}
	prefix := ""
	switch file.FlatType(op) {
	case "!is":
		prefix = "is"
	case "!in":
		prefix = "in"
	default:
		return "", false
	}
	operand := file.FlatNextSib(op)
	if operand == 0 || file.FlatStartByte(operand) != file.FlatEndByte(op) {
		return "", false
	}
	leaf := operand
	for file.FlatType(leaf) != "type_identifier" && file.FlatType(leaf) != "simple_identifier" {
		leaf = file.FlatFirstChild(leaf)
		if leaf == 0 {
			return "", false
		}
	}
	if file.FlatStartByte(leaf) != file.FlatEndByte(op) {
		return "", false
	}
	return prefix + file.FlatNodeText(leaf), true
}

// unusedParameterInMisparsedDeclarationFlat reports whether a function's
// enclosing declaration was lost to tree-sitter-kotlin error recovery, so its
// owner (and with it whether the function is an overridable interface
// member) cannot be determined from the tree. The grammar cannot parse
// `fun interface`: its members land in a top-level lambda or expression, or
// become top-level function siblings of the mangled header, and error
// recovery can swallow the declarations that follow as well.
func unusedParameterInMisparsedDeclarationFlat(file *scanner.File, fn uint32) bool {
	if file == nil || fn == 0 {
		return false
	}
	if flatIsFunInterfaceHeader(file, fn) {
		return true
	}
	top := fn
	for {
		p, ok := file.FlatParent(top)
		if !ok {
			return false
		}
		if file.FlatType(p) == "source_file" {
			break
		}
		top = p
	}
	switch file.FlatType(top) {
	case "function_declaration", "class_declaration", "object_declaration",
		"property_declaration", "type_alias":
	default:
		// A Kotlin source file holds only declarations at the top level;
		// scripts may hold statements, where a lambda can declare a local
		// function legitimately.
		return !strings.HasSuffix(file.Path, ".kts")
	}
	if top != fn {
		return false
	}
	// A top-level function whose earlier siblings reach a `fun interface`
	// header before the interface's closing brace is a member of it.
	for sib, ok := file.FlatPrevSibling(fn); ok && sib != 0; sib, ok = file.FlatPrevSibling(sib) {
		switch file.FlatType(sib) {
		case "}", "lambda_literal":
			// A closing brace ends the interface body; a top-level lambda
			// already holds the body of an earlier header.
			return false
		case "ERROR":
			if flatIsFunInterfaceHeader(file, sib) {
				return true
			}
			if file.FlatHasChildOfType(sib, "}") {
				return false
			}
		case "function_declaration":
			if flatIsFunInterfaceHeader(file, sib) {
				return true
			}
		}
	}
	return false
}

// flatIsFunInterfaceHeader reports whether node is the error-recovered
// header of a `fun interface` declaration: a `fun` keyword immediately
// followed by `interface` parsed as a type or identifier.
func flatIsFunInterfaceHeader(file *scanner.File, node uint32) bool {
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "fun" {
			continue
		}
		next := file.FlatNextSib(child)
		if next == 0 {
			return false
		}
		switch file.FlatType(next) {
		case "user_type", "type_identifier", "simple_identifier":
			return file.FlatNodeTextEquals(next, "interface")
		}
		return false
	}
	return false
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
			// Only a genuine *local* function shadows an outer parameter. A
			// member function inside a nested class/object literal (e.g. an
			// `override fun onAnimationStart()` in an `object : Listener { ... }`)
			// is a member declaration, not a local; a bare call to a captured
			// parameter inside such a member body still refers to the parameter.
			// Treating the member's name as a shadow produced a false positive
			// whenever the parameter and the overridden member share a name.
			if !unusedParameterFunctionIsLocalFlat(file, candidate) {
				return
			}
			if extractIdentifierFlat(file, candidate) == name &&
				unusedParameterDeclarationScopeContainsRefFlat(file, body, candidate, ref) {
				shadowed = true
			}
		}
	})
	return shadowed
}

// unusedParameterFunctionIsLocalFlat reports whether a function_declaration is
// a true local function (declared inside a statement block / function body)
// rather than a member of a class, object, or interface body. Only local
// functions introduce a name that can shadow an enclosing parameter.
func unusedParameterFunctionIsLocalFlat(file *scanner.File, fn uint32) bool {
	parent, ok := file.FlatParent(fn)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "statements", "function_body", "control_structure_body":
		return true
	default:
		return false
	}
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
