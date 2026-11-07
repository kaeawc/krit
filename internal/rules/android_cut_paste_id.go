package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// CutPasteIDRule ports Android Lint's CutPasteId (CutPasteDetector): within one
// method, the same R.id looked up a second time with findViewById or
// requireViewById on the same receiver, into a different variable. That is the
// trace of a pasted lookup whose id was not updated. A lookup into the same
// variable, on another receiver, or in the other branch of an if, when or
// switch is not reported.
//
// Lint decides reachability on UAST; this port approximates it with source order
// plus the exclusive branches of if, when and switch, and ignores returns
// between the two lookups. The layout rule that used this name is
// IdTypePrefixMismatchResourceRule.
type CutPasteIDRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: reachability is approximated.
func (r *CutPasteIDRule) Confidence() float64 { return api.ConfidenceMedium }

// cutPasteIDArgRe matches the id argument: a qualified name whose second to last part is `id`, such as R.id.name.
var cutPasteIDArgRe = regexp.MustCompile(`^(?:\w+\.)*id\.(\w+)$`)

type cutPasteLookup struct {
	node     uint32 // the call
	nameNode uint32 // the findViewById identifier
	lhs      string
	operand  string
	id       string
	argText  string
	hasVar   bool
	assigned bool // the variable is assigned after its declaration, not declared by this lookup
}

func (r *CutPasteIDRule) check(ctx *api.Context) {
	file, fn := ctx.File, ctx.Idx
	var lookups []cutPasteLookup
	var walk func(n uint32)
	walk = func(n uint32) {
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "class_body", "object_literal", "anonymous_initializer":
				continue // members and initializers of nested classes are separate methods
			case "call_expression", "method_invocation":
				if lookup, ok := cutPasteLookupAt(file, child); ok {
					// A Kotlin assignment to a property goes through a setter, which Lint cannot resolve to a variable.
					if file.Language == scanner.LangKotlin && lookup.assigned && !cutPasteKotlinLocal(file, fn, lookup.lhs) {
						lookup.hasVar = false
					}
					lookups = append(lookups, lookup)
				}
			}
			walk(child)
		}
	}
	walk(fn)

	seen := map[string]cutPasteLookup{}
	for _, cur := range lookups {
		if prev, ok := seen[cur.id]; ok {
			if prev.lhs != cur.lhs && prev.operand == cur.operand && cutPasteReachable(file, prev.node, cur.node) && prev.hasVar && cur.hasVar {
				f := r.Finding(file, file.FlatRow(cur.nameNode)+1, file.FlatCol(cur.nameNode)+1,
					fmt.Sprintf("The id `%s` has already been looked up in this method; possible cut & paste error?", cur.argText))
				ctx.Emit(f)
			}
		}
		seen[cur.id] = cur
	}
}

// cutPasteLookupAt describes a findViewById or requireViewById call on an R.id constant that assigns a variable.
func cutPasteLookupAt(file *scanner.File, call uint32) (cutPasteLookup, bool) {
	var lookup cutPasteLookup
	var argText string
	switch file.FlatType(call) {
	case "call_expression":
		name := flatCallExpressionName(file, call)
		if name != "findViewById" && name != "requireViewById" {
			return lookup, false
		}
		navExpr, args := flatCallExpressionParts(file, call)
		arg := flatPositionalValueArgument(file, args, 0)
		if arg == 0 {
			return lookup, false
		}
		expr := flatValueArgumentExpression(file, arg)
		if expr == 0 {
			return lookup, false
		}
		argText = strings.TrimSpace(file.FlatNodeText(expr))
		lookup.nameNode = call
		if navExpr != 0 {
			text := file.FlatNodeText(navExpr)
			if dot := strings.LastIndex(text, "."); dot >= 0 {
				lookup.operand = strings.TrimSuffix(strings.TrimSpace(text[:dot]), "?")
			}
			lookup.nameNode = flatLastIdentifierNode(file, navExpr)
		} else if id := flatFirstChildOfType(file, call, "simple_identifier"); id != 0 {
			lookup.nameNode = id
		}
	case "method_invocation":
		if name := javaMethodInvocationName(file, call); name != "findViewById" && name != "requireViewById" {
			return lookup, false
		}
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok {
			return lookup, false
		}
		var first uint32
		for c := file.FlatFirstChild(args); c != 0; c = file.FlatNextSib(c) {
			if file.FlatIsNamed(c) {
				first = c
				break
			}
		}
		if first == 0 {
			return lookup, false
		}
		argText = strings.TrimSpace(file.FlatNodeText(first))
		lookup.nameNode = call
		for c := file.FlatFirstChild(call); c != 0 && c != args; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "identifier" {
				lookup.nameNode = c
			}
		}
		if start, nameStart := int(file.FlatStartByte(call)), int(file.FlatStartByte(lookup.nameNode)); nameStart > start {
			before := strings.TrimSpace(file.FlatNodeText(call)[:nameStart-start])
			lookup.operand = strings.TrimSpace(strings.TrimSuffix(before, "."))
		}
	default:
		return lookup, false
	}
	m := cutPasteIDArgRe.FindStringSubmatch(strings.Join(strings.Fields(argText), ""))
	if m == nil {
		return lookup, false
	}
	lookup.node, lookup.id, lookup.argText = call, m[1], argText
	if parent, ok := file.FlatParent(call); ok && cutPasteIsPlainBinary(file.FlatType(parent)) {
		return lookup, false
	}
	lookup.lhs, lookup.hasVar, lookup.assigned = cutPasteAssignedVariable(file, call)
	if lookup.lhs == "" {
		return lookup, false
	}
	return lookup, true
}

// cutPasteIsPlainBinary reports a binary operator node that is not an assignment.
func cutPasteIsPlainBinary(nodeType string) bool {
	switch nodeType {
	case "equality_expression", "comparison_expression", "conjunction_expression", "disjunction_expression",
		"additive_expression", "multiplicative_expression", "binary_expression":
		return true
	}
	return false
}

// cutPasteAssignedVariable climbs from call to the variable declaration or assignment that receives its result, and
// returns the variable's text. hasVar is false for an array element, which Lint cannot resolve to a variable.
// assigned is true when the call assigns an existing variable instead of declaring one.
func cutPasteAssignedVariable(file *scanner.File, call uint32) (lhs string, hasVar, assigned bool) {
	cur := call
	for {
		parent, ok := file.FlatParent(cur)
		if !ok {
			return "", false, false
		}
		switch file.FlatType(parent) {
		case "statements", "block", "function_body", "class_body", "control_structure_body", "source_file", "program":
			return "", false, false
		case "property_declaration":
			if decl, ok := file.FlatFindChild(parent, "variable_declaration"); ok {
				return strings.TrimSpace(file.FlatNodeText(decl)), true, false
			}
			return "", false, false
		case "variable_declarator":
			if id, ok := file.FlatFindChild(parent, "identifier"); ok {
				return file.FlatNodeText(id), true, false
			}
			return "", false, false
		case "assignment":
			left := file.FlatNamedChild(parent, 0)
			if left != 0 && left != cur {
				if flatFirstChildOfType(file, left, "indexing_suffix") != 0 {
					return strings.TrimSpace(file.FlatNodeText(flatFirstChildOfType(file, left, "simple_identifier"))), false, true
				}
				return strings.TrimSpace(file.FlatNodeText(left)), true, true
			}
			return "", false, false
		case "assignment_expression":
			left := file.FlatNamedChild(parent, 0)
			if left != 0 && left != cur {
				if file.FlatType(left) == "array_access" {
					return strings.TrimSpace(file.FlatNodeText(file.FlatNamedChild(left, 0))), false, true
				}
				return strings.TrimSpace(file.FlatNodeText(left)), true, true
			}
			return "", false, false
		}
		cur = parent
	}
}

// cutPasteReachable reports whether the second lookup can run after the first: they are not in exclusive branches
// of the same if, when or switch.
func cutPasteReachable(file *scanner.File, first, second uint32) bool {
	chain := func(n uint32) []uint32 {
		var out []uint32
		for cur := n; ; {
			out = append(out, cur)
			parent, ok := file.FlatParent(cur)
			if !ok {
				return out
			}
			cur = parent
		}
	}
	a, b := chain(first), chain(second)
	// Walk from the roots while the chains agree to find the lowest common ancestor.
	i, j := len(a)-1, len(b)-1
	for i > 0 && j > 0 && a[i-1] == b[j-1] {
		i--
		j--
	}
	if i == 0 || j == 0 {
		return true // one lookup contains the other
	}
	lca := a[i]
	armA, armB := a[i-1], b[j-1]
	switch file.FlatType(lca) {
	case "if_expression", "if_statement":
		return !cutPasteIsBranch(file, armA) || !cutPasteIsBranch(file, armB) || armA == armB
	case "when_expression", "switch_expression", "switch_block", "switch_statement":
		return !cutPasteIsCase(file, armA) || !cutPasteIsCase(file, armB) || armA == armB
	}
	return true
}

// cutPasteIsBranch reports whether an if's child is a branch (not the condition).
func cutPasteIsBranch(file *scanner.File, n uint32) bool {
	switch file.FlatType(n) {
	case "control_structure_body", "block", "expression_statement", "return_statement":
		return true
	}
	return false
}

func cutPasteIsCase(file *scanner.File, n uint32) bool {
	switch file.FlatType(n) {
	case "when_entry", "switch_block_statement_group", "switch_rule":
		return true
	}
	return false
}

// flatFirstChildOfType returns the first direct child of the given type, or 0.
func flatFirstChildOfType(file *scanner.File, parent uint32, nodeType string) uint32 {
	for c := file.FlatFirstChild(parent); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == nodeType {
			return c
		}
	}
	return 0
}

// flatLastIdentifierNode returns the last simple_identifier of a navigation expression, or the expression.
func flatLastIdentifierNode(file *scanner.File, nav uint32) uint32 {
	last := nav
	file.FlatWalkAllNodes(nav, func(n uint32) {
		if file.FlatType(n) == "simple_identifier" {
			last = n
		}
	})
	return last
}

// cutPasteKotlinLocal reports whether fn declares a local variable or parameter called name.
func cutPasteKotlinLocal(file *scanner.File, fn uint32, name string) bool {
	found := false
	file.FlatWalkAllNodes(fn, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "variable_declaration", "parameter", "value_parameter":
			if id := flatFirstChildOfType(file, n, "simple_identifier"); id != 0 && file.FlatNodeText(id) == name {
				found = true
			}
		}
	})
	return found
}
