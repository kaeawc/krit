package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// UnreachableCatchBlockRule detects catch shadowed by general catch above.
// ---------------------------------------------------------------------------
type UnreachableCatchBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — catch-block
// reachability depends on the thrown-type hierarchy from the resolver;
// heuristic fallback uses name containment. Classified per roadmap/17.
func (r *UnreachableCatchBlockRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnreachableCatchBlockRule) checkFlatNode(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	type catchEntry struct {
		typeName string
		line     int
	}
	var catches []catchEntry
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) != "catch_block" {
			continue
		}
		typeName := extractCatchTypeFlat(file, child)
		if typeName == "" {
			continue
		}
		catches = append(catches, catchEntry{
			typeName: typeName,
			line:     file.FlatRow(child) + 1,
		})
	}
	for i := 0; i < len(catches); i++ {
		for j := i + 1; j < len(catches); j++ {
			parentType := catches[i].typeName
			childType := catches[j].typeName
			childLine := catches[j].line
			if parentType == childType {
				ctx.Emit(r.Finding(file, childLine, 1,
					fmt.Sprintf("Duplicate catch block for '%s'.", childType)))
				continue
			}
			if ctx.Resolver != nil {
				if ctx.Resolver.IsExceptionSubtype(childType, parentType) {
					ctx.Emit(r.Finding(file, childLine, 1,
						fmt.Sprintf("Catch block for '%s' is unreachable because '%s' is caught above.", childType, parentType)))
				}
			} else {
				if typeinfer.IsSubtypeOfException(childType, parentType) {
					ctx.Emit(r.Finding(file, childLine, 1,
						fmt.Sprintf("Catch block for '%s' is unreachable because '%s' is caught above.", childType, parentType)))
				}
			}
		}
	}
}

func extractCatchTypeFlat(file *scanner.File, catchNode uint32) string {
	for i := 0; i < file.FlatChildCount(catchNode); i++ {
		child := file.FlatChild(catchNode, i)
		if file.FlatType(child) == "user_type" {
			text := strings.TrimSpace(file.FlatNodeText(child))
			if idx := strings.Index(text, "<"); idx >= 0 {
				text = text[:idx]
			}
			if text != "" {
				return text
			}
		}
	}
	return ""
}

// nothingReturningFuncs lists bare function names that are known to return Nothing.
//
// The heuristic-only callers (blockTerminatesFlat) require this to be a
// conservative allow-list — any name added here is treated as Nothing-returning
// even without resolver evidence. Names that overlap with common user-defined
// helpers (like `fail`) are intentionally omitted; they get picked up via the
// resolver-backed path when the resolver can prove a Nothing return type.
var nothingReturningFuncs = map[string]bool{
	"TODO":        true,
	"error":       true,
	"exitProcess": true,
}

// nothingReturningQualifierPrefixes are the leading navigation segments
// accepted as qualifiers for a Nothing-returning stdlib call. `kotlin.error`
// or `kotlin.system.exitProcess` qualify; arbitrary user-defined receivers
// like `myObject.error` do not.
var nothingReturningQualifierPrefixes = [][]string{
	{"kotlin"},
	{"kotlin", "system"},
	{"kotlin", "test"},
}

func isFlatCommentNode(file *scanner.File, idx uint32) bool {
	return file != nil && (file.FlatType(idx) == "line_comment" || file.FlatType(idx) == "multiline_comment")
}

func isNothingCallFlat(file *scanner.File, idx uint32) bool {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return false
	}
	name := flatCallExpressionName(file, idx)
	if !nothingReturningFuncs[name] {
		return false
	}
	nav, _ := flatCallExpressionParts(file, idx)
	if nav == 0 {
		return true
	}
	segments := flatNavigationChainIdentifiers(file, nav)
	if len(segments) == 0 || segments[len(segments)-1] != name {
		return false
	}
	prefix := segments[:len(segments)-1]
	for _, accepted := range nothingReturningQualifierPrefixes {
		if stringSliceEqual(prefix, accepted) {
			return true
		}
	}
	return false
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// isNothingReturningCallFlat returns true for any call_expression whose
// callee provably returns Nothing. It first checks the static heuristic
// table (isNothingCallFlat) for stdlib globals, then asks the resolver
// for the call's inferred type when the call has no receiver. The
// resolver-backed path catches workspace functions declared with
// `: Nothing` return type.
//
// The resolver fallback is restricted to bare calls because the
// resolver's call-type inference falls back to top-level stdlib lookup
// when a navigation receiver doesn't yield a matching method, which can
// produce a misleading Nothing for harmless calls like `logger.error()`.
// Limiting to bare calls avoids that false positive while still catching
// the high-value case (workspace `fun X(): Nothing`).
//
// Returns false when the resolver is nil or the inferred type is unknown.
func isNothingReturningCallFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	if isNothingCallFlat(file, idx) {
		return true
	}
	if resolver == nil || file == nil || file.FlatType(idx) != "call_expression" {
		return false
	}
	if nav, _ := flatCallExpressionParts(file, idx); nav != 0 {
		// Has a receiver; the resolver's fallback to top-level stdlib
		// lookup can produce a wrong Nothing here. Stay conservative.
		return false
	}
	t := resolver.ResolveFlatNode(idx, file)
	if t == nil {
		return false
	}
	return t.Name == "Nothing"
}

// ifAllBranchesTerminateFlat checks if an if_expression has both then and else branches
// and each branch ends with a return/throw/Nothing-call.
func ifAllBranchesTerminateFlat(file *scanner.File, idx uint32) bool {
	var thenBlock, elseBlock uint32
	foundElse := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatNodeTextEquals(child, "else") {
			foundElse = true
			continue
		}
		if file.FlatType(child) == "control_structure_body" {
			if !foundElse {
				thenBlock = child
			} else {
				elseBlock = child
			}
		}
		// else branch could be another if_expression (else if)
		if foundElse && file.FlatType(child) == "if_expression" {
			if ifAllBranchesTerminateFlat(file, child) {
				elseBlock = child // mark as terminating
			} else {
				return false
			}
		}
	}
	if thenBlock == 0 || !foundElse {
		return false
	}
	if elseBlock == 0 && !foundElse {
		return false
	}
	// Check then block terminates
	if !blockTerminatesFlat(file, thenBlock) {
		return false
	}
	// If else branch was an if_expression that already passed, it terminates
	if elseBlock != 0 && file.FlatType(elseBlock) == "if_expression" {
		return true // already checked recursively above
	}
	if elseBlock == 0 {
		return false
	}
	return blockTerminatesFlat(file, elseBlock)
}

// blockTerminatesFlat checks if the last statement in a block is a jump or Nothing-call.
func blockTerminatesFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	// Unwrap single control_structure_body → statements
	stmts := idx
	if file.FlatType(idx) == "control_structure_body" {
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			if file.FlatType(child) == "statements" {
				stmts = child
				break
			}
		}
	}
	// Find the last non-comment child
	var last uint32
	for i := file.FlatChildCount(stmts) - 1; i >= 0; i-- {
		child := file.FlatChild(stmts, i)
		if file.FlatType(child) != "line_comment" && file.FlatType(child) != "multiline_comment" &&
			file.FlatType(child) != "{" && file.FlatType(child) != "}" {
			last = child
			break
		}
	}
	if last == 0 {
		return false
	}
	if file.FlatType(last) == "jump_expression" {
		text := file.FlatNodeText(last)
		return strings.HasPrefix(text, "return") || strings.HasPrefix(text, "throw")
	}
	if isNothingCallFlat(file, last) {
		return true
	}
	// Recursively check nested if/when
	if file.FlatType(last) == "if_expression" {
		return ifAllBranchesTerminateFlat(file, last)
	}
	return false
}

// whenIsExhaustiveAndTerminates checks if a when expression has an else branch
// (or covers all sealed/enum variants) and all branches terminate.
func whenIsExhaustiveAndTerminatesFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	hasElse := false
	allTerminate := true
	branchCount := 0

	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) != "when_entry" {
			continue
		}
		branchCount++

		// Check for else branch
		entryText := file.FlatNodeText(child)
		if strings.HasPrefix(strings.TrimSpace(entryText), "else") {
			hasElse = true
		}

		// Check if the body of this entry terminates
		body, _ := file.FlatFindChild(child, "control_structure_body")
		if body == 0 {
			// Could be a single expression entry — check the last child
			var lastChild uint32
			for j := file.FlatChildCount(child) - 1; j >= 0; j-- {
				c := file.FlatChild(child, j)
				if !isFlatCommentNode(file, c) {
					lastChild = c
					break
				}
			}
			if lastChild != 0 && (file.FlatType(lastChild) == "jump_expression" || isNothingCallFlat(file, lastChild)) {
				continue
			}
			allTerminate = false
			continue
		}
		if !blockTerminatesFlat(file, body) {
			allTerminate = false
		}
	}

	if branchCount == 0 {
		return false
	}

	// If there's an else branch and all branches terminate, code after is unreachable
	if hasElse && allTerminate {
		return true
	}

	// Check if when subject is a sealed class/enum and all variants are covered
	if !hasElse && allTerminate && resolver != nil {
		subjectName := whenSubjectTypeNameFlat(file, idx, resolver)
		if subjectName != "" {
			variants := resolver.SealedVariants(subjectName)
			if len(variants) > 0 && branchCount >= len(variants) {
				return true
			}
			entries := resolver.EnumEntries(subjectName)
			if len(entries) > 0 && branchCount >= len(entries) {
				return true
			}
		}
	}

	return false
}

// whenSubjectTypeName extracts the type name of the when subject expression.
func whenSubjectTypeNameFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) string {
	// when(expr) — look for the subject expression
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "when_subject" {
			// The actual expression is inside when_subject
			for j := 0; j < file.FlatChildCount(child); j++ {
				inner := file.FlatChild(child, j)
				if file.FlatType(inner) == "simple_identifier" {
					resolved := resolver.ResolveFlatNode(inner, file)
					if resolved != nil && resolved.Name != "" {
						return resolved.Name
					}
				}
			}
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// MissingReturnRule — flags block-bodied functions with a non-Unit/non-Nothing
// declared return type whose body cannot guarantee a returned value on every
// path. Mimics kotlinc's NO_RETURN_IN_FUNCTION_WITH_BLOCK_BODY diagnostic
// using AST + the termination helpers above.
//
// Rule scope is intentionally narrow:
//   - block-bodied functions only (`{ ... }`); expression bodies (`= expr`)
//     always return their expression value
//   - functions with an explicit declared return type only; the implicit
//     Unit case is silent
//   - return type "Unit"/"Nothing" (and the kotlin.X qualified forms) is
//     skipped because Unit doesn't need a return and Nothing-returning
//     functions can't return normally
//   - abstract / interface / expect / external functions have no body and
//     are skipped
//
// ---------------------------------------------------------------------------
type MissingReturnRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MissingReturnRule) Confidence() float64 { return api.ConfidenceHigh }

func missingReturnDeclaredType(file *scanner.File, fn uint32) (typeIdx uint32, body uint32) {
	foundParams := false
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_value_parameters":
			foundParams = true
		case "function_body":
			body = child
		case "user_type", "nullable_type", "type_identifier":
			if foundParams && typeIdx == 0 {
				typeIdx = child
			}
		}
	}
	return typeIdx, body
}

func missingReturnTypeIsUnitOrNothing(text string) bool {
	switch strings.TrimSpace(text) {
	case "Unit", "Nothing", "kotlin.Unit", "kotlin.Nothing":
		return true
	}
	return false
}

func missingReturnIsExpressionBody(file *scanner.File, body uint32) bool {
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			return true
		}
		if file.FlatIsNamed(child) {
			// Reached a named child (e.g. statements) before any `=`. It's a
			// block body.
			return false
		}
	}
	return false
}

// missingReturnFunctionTerminates returns true when the block body is
// guaranteed to terminate on every path with a return, throw,
// Nothing-returning call, exhaustive-if, or exhaustive-when.
func missingReturnFunctionTerminates(file *scanner.File, body uint32, resolver typeinfer.TypeResolver) bool {
	if file == nil || body == 0 {
		return false
	}
	stmts, _ := file.FlatFindChild(body, "statements")
	if stmts == 0 {
		return false
	}
	var last uint32
	for i := file.FlatChildCount(stmts) - 1; i >= 0; i-- {
		c := file.FlatChild(stmts, i)
		t := file.FlatType(c)
		if t == "line_comment" || t == "multiline_comment" || t == "{" || t == "}" {
			continue
		}
		last = c
		break
	}
	if last == 0 {
		return false
	}
	switch file.FlatType(last) {
	case "jump_expression":
		text := file.FlatNodeText(last)
		return strings.HasPrefix(text, "return") || strings.HasPrefix(text, "throw")
	case "if_expression":
		return ifAllBranchesTerminateFlat(file, last)
	case "when_expression":
		return whenIsExhaustiveAndTerminatesFlat(file, last, resolver)
	}
	return isNothingReturningCallFlat(file, last, resolver)
}

func (r *MissingReturnRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if file.FlatType(idx) != "function_declaration" {
		return
	}
	if file.FlatHasModifier(idx, "abstract") || file.FlatHasModifier(idx, "expect") || file.FlatHasModifier(idx, "external") {
		return
	}
	retType, body := missingReturnDeclaredType(file, idx)
	if body == 0 || retType == 0 {
		return
	}
	if missingReturnTypeIsUnitOrNothing(file.FlatNodeText(retType)) {
		return
	}
	if missingReturnIsExpressionBody(file, body) {
		return
	}
	if missingReturnFunctionTerminates(file, body, ctx.Resolver) {
		return
	}
	name := flatDeclarationNameLocal(file, idx)
	typeText := strings.TrimSpace(file.FlatNodeText(retType))
	msg := fmt.Sprintf("Function '%s' declares return type '%s' but its body may not return on every path. Add a `return` or terminate every branch.", name, typeText)
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
}

// flatDeclarationNameLocal returns the function declaration's
// simple_identifier name. Local helper to avoid pulling in the
// resolver package's flatDeclarationName.
func flatDeclarationNameLocal(file *scanner.File, fn uint32) string {
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" {
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return "<anonymous>"
}
