package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnnecessaryApply follows detekt 1.23.8. It reports `receiver.apply { stmt }` when
//   - the lambda has exactly one statement: an assignment, `this`, a reference, or a
//     dot-qualified expression (a safe call chain does not count);
//   - exactly one name reference reads the apply receiver implicitly (an unqualified
//     member, or `this`), counted over the whole statement or, for an assignment, only
//     its target;
//   - the apply value is not used: the call is a statement of a block, and the block
//     does not produce a value that the surrounding code reads.
//
// detekt resolves the implicit receivers with type information. This syntactic fallback
// has no types, so it reports only when `this` is the one receiver reference and every
// other name is provably not a member of the receiver. The FIR twin reports the rest.

const (
	unnecessaryApplyOmitMessage = "apply expression can be omitted"
	unnecessaryApplyLetMessage  = "apply can be replaced with let or an if"
)

// unnecessaryApplyShadow matches a user-declared or imported `apply`, which is not kotlin.apply.
var unnecessaryApplyShadow = regexp.MustCompile(`\bfun\s+(?:<[^>]*>\s*)?(?:[\w.<>?, ]+\.)?apply\s*[<(]`)

// unnecessaryApplyImport matches an import of a function named `apply`, with its package and any alias.
var unnecessaryApplyImport = regexp.MustCompile(`(?m)^\s*import\s+([\w.]+)\.apply\b(\s+as\s+\w+)?`)

// unnecessaryApplyShadowed reports whether `apply` in file may be something other than kotlin.apply.
func unnecessaryApplyShadowed(content []byte) bool {
	if unnecessaryApplyShadow.Match(content) {
		return true
	}
	for _, m := range unnecessaryApplyImport.FindAllSubmatch(content, -1) {
		if string(m[1]) != "kotlin" || len(m[2]) > 0 {
			return true
		}
	}
	return false
}

// unnecessaryApplyStdlibCallees are top-level functions a receiver is not expected to shadow.
var unnecessaryApplyStdlibCallees = map[string]bool{
	"println": true, "print": true, "require": true, "check": true, "TODO": true,
	"listOf": true, "mapOf": true, "setOf": true, "arrayOf": true,
	"emptyList": true, "emptyMap": true, "emptySet": true,
	"mutableListOf": true, "mutableMapOf": true, "mutableSetOf": true,
	"hashMapOf": true, "linkedMapOf": true, "hashSetOf": true, "linkedSetOf": true,
}

func (r *UnnecessaryApplyRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if flatCallExpressionName(file, idx) != "apply" {
		return
	}
	nav, _ := flatCallExpressionParts(file, idx)
	suffix, _ := file.FlatFindChild(idx, "call_suffix")
	if nav == 0 || suffix == 0 {
		return
	}
	if _, hasArgs := file.FlatFindChild(suffix, "value_arguments"); hasArgs {
		return
	}
	navSuffix := flatLastChildOfType(file, nav, "navigation_suffix")
	if navSuffix == 0 {
		return
	}
	applyName, _ := file.FlatFindChild(navSuffix, "simple_identifier")
	if applyName == 0 {
		return
	}
	lambda, params, stmts := flatTrailingLambdaParts(file, suffix)
	if lambda == 0 || params != 0 || stmts == 0 {
		return
	}
	stmt := applySingleStatement(file, stmts)
	if stmt == 0 || !applyStatementShape(file, stmt) {
		return
	}
	if !applyValueUnused(file, idx) {
		return
	}
	if unnecessaryApplyShadowed(file.Content) {
		return
	}
	if !applySingleExplicitThisReference(file, suffix, lambda, applyScanRoot(file, stmt)) {
		return
	}
	msg := unnecessaryApplyOmitMessage
	if strings.HasPrefix(file.FlatNodeText(navSuffix), "?.") {
		msg = unnecessaryApplyLetMessage
	}
	ctx.Emit(r.Finding(file, file.FlatRow(applyName)+1, file.FlatCol(applyName)+1, msg))
}

// applySignificantChildren returns the named children of stmts that are not comments.
func applySignificantChildren(file *scanner.File, stmts uint32) []uint32 {
	var out []uint32
	for c := file.FlatFirstChild(stmts); c != 0; c = file.FlatNextSib(c) {
		if !file.FlatIsNamed(c) {
			continue
		}
		if t := file.FlatType(c); t == "line_comment" || t == "multiline_comment" {
			continue
		}
		out = append(out, c)
	}
	return out
}

func applySingleStatement(file *scanner.File, stmts uint32) uint32 {
	children := applySignificantChildren(file, stmts)
	if len(children) != 1 {
		return 0
	}
	return children[0]
}

// applyScanRoot is the part of stmt whose name references detekt counts: the target of an assignment, else the statement.
func applyScanRoot(file *scanner.File, stmt uint32) uint32 {
	if file.FlatType(stmt) != "assignment" {
		return stmt
	}
	if children := applySignificantChildren(file, stmt); len(children) > 0 {
		return children[0]
	}
	return stmt
}

// applyStatementShape reports whether stmt is one of the statement kinds detekt accepts.
func applyStatementShape(file *scanner.File, stmt uint32) bool {
	switch file.FlatType(stmt) {
	case "assignment":
		return file.FlatNamedChildCount(stmt) > 1
	case "this_expression", "simple_identifier":
		return true
	case "call_expression", "navigation_expression", "indexing_expression":
		return !applyEndsInSafeAccess(file, stmt)
	}
	return false
}

// applyEndsInSafeAccess reports whether the outermost qualifier of a chain is `?.` or `::`, so the chain is not dot-qualified.
func applyEndsInSafeAccess(file *scanner.File, node uint32) bool {
	switch file.FlatType(node) {
	case "navigation_expression":
		last := flatLastChildOfType(file, node, "navigation_suffix")
		if last == 0 {
			return false
		}
		text := file.FlatNodeText(last)
		return strings.HasPrefix(text, "?.") || strings.HasPrefix(text, "::")
	case "call_expression", "indexing_expression":
		first := file.FlatFirstChild(node)
		if first != 0 {
			switch file.FlatType(first) {
			case "navigation_expression", "call_expression", "indexing_expression":
				return applyEndsInSafeAccess(file, first)
			}
		}
	}
	return false
}

// applyValueUnused reports whether the value of the apply call is certainly not read.
func applyValueUnused(file *scanner.File, call uint32) bool {
	return applyExpressionUnused(file, call)
}

// applyExpressionUnused reports whether the value of node is certainly dropped.
// It answers false whenever the surrounding code might read the value.
func applyExpressionUnused(file *scanner.File, node uint32) bool {
	parent, ok := file.FlatParent(node)
	if !ok || file.FlatType(parent) != "statements" {
		return false
	}
	if !applyIsLastStatement(file, parent, node) {
		return true
	}
	owner, ok := file.FlatParent(parent)
	if !ok {
		return false
	}
	switch file.FlatType(owner) {
	case "function_body":
		grand, _ := file.FlatParent(owner)
		switch file.FlatType(grand) {
		case "function_declaration", "getter", "setter", "anonymous_function":
			return true
		}
	case "anonymous_initializer", "secondary_constructor", "finally_block":
		return true
	case "catch_block", "try_expression":
		try := owner
		if file.FlatType(owner) == "catch_block" {
			try, _ = file.FlatParent(owner)
		}
		return applyExpressionUnused(file, try)
	case "control_structure_body":
		return applyBranchUnused(file, owner)
	}
	return false
}

// applyBranchUnused handles a braced branch of an if, a when or a loop.
func applyBranchUnused(file *scanner.File, body uint32) bool {
	owner, ok := file.FlatParent(body)
	if !ok {
		return false
	}
	switch file.FlatType(owner) {
	case "for_statement", "while_statement", "do_while_statement":
		return true
	case "if_expression":
		branches := 0
		for c := file.FlatFirstChild(owner); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "control_structure_body" {
				branches++
			}
		}
		if branches < 2 {
			return true
		}
		return applyExpressionUnused(file, owner)
	case "when_entry":
		when, _ := file.FlatParent(owner)
		for c := file.FlatFirstChild(when); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "when_entry" && !applyHasChild(file, c, "when_condition") {
				return applyExpressionUnused(file, when)
			}
		}
	}
	return false
}

func applyHasChild(file *scanner.File, node uint32, childType string) bool {
	_, ok := file.FlatFindChild(node, childType)
	return ok
}

func applyIsLastStatement(file *scanner.File, stmts, node uint32) bool {
	children := applySignificantChildren(file, stmts)
	return len(children) > 0 && children[len(children)-1] == node
}

// applySingleExplicitThisReference reports whether `this` is the only receiver reference.
func applySingleExplicitThisReference(file *scanner.File, suffix, lambda, stmt uint32) bool {
	label := ""
	if ann, ok := file.FlatFindChild(suffix, "annotated_lambda"); ok {
		if l, ok := file.FlatFindChild(ann, "label"); ok {
			label = strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(l)), "@")
		}
	}
	certain, unknown, unsafe := 0, 0, false
	file.FlatWalkAllNodes(stmt, func(n uint32) {
		if unsafe {
			return
		}
		switch file.FlatType(n) {
		case "this_expression":
			if parent, ok := file.FlatParent(n); ok && file.FlatType(parent) == "call_expression" && file.FlatFirstChild(parent) == n {
				// `this()` calls invoke; detekt counts no receiver reference there.
				return
			}
			target := ""
			if lt, ok := file.FlatFindChild(n, "type_identifier"); ok {
				target = file.FlatNodeText(lt)
			}
			if target != "" && target != "apply" && target != label {
				return
			}
			if applyInsideNestedScope(file, n, stmt) {
				unsafe = true
				return
			}
			certain++
		case "simple_identifier", "interpolated_identifier":
			parent, ok := file.FlatParent(n)
			if !ok || !applyReferenceParent(file, parent, n) {
				return
			}
			name := file.FlatNodeText(n)
			if name == "it" {
				return
			}
			if file.FlatType(parent) == "call_expression" && unnecessaryApplyStdlibCallees[name] {
				return
			}
			if applyScopeDeclares(file, stmt, name) || applyEnclosingScopeDeclares(file, lambda, name) {
				return
			}
			unknown++
		}
	})
	return !unsafe && certain == 1 && unknown == 0
}

// applyInsideNestedScope reports whether a lambda, function, class or object sits between node and stmt.
func applyInsideNestedScope(file *scanner.File, node, stmt uint32) bool {
	if node == stmt {
		return false
	}
	for p, ok := file.FlatParent(node); ok && p != stmt; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal", "anonymous_function", "function_declaration", "class_declaration",
			"object_declaration", "object_literal", "class_body":
			return true
		}
	}
	return false
}

// applyReferenceParent reports whether an identifier under parent can name a variable or function.
func applyReferenceParent(file *scanner.File, parent, ident uint32) bool {
	switch file.FlatType(parent) {
	case "variable_declaration", "parameter", "class_parameter", "function_declaration", "navigation_suffix",
		"lambda_parameters", "label", "user_type", "type_identifier", "import_header", "identifier", "package_header",
		"class_declaration", "object_declaration", "type_alias", "catch_block", "annotation", "user_type_parameter":
		return false
	case "value_argument":
		if next := file.FlatNextSib(ident); next != 0 && file.FlatType(next) == "=" {
			return false
		}
	}
	return true
}
