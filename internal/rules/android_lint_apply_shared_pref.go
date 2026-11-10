package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const sharedPreferencesClass = "android.content.SharedPreferences"

// editorMutatorNames are the SharedPreferences.Editor methods that return the editor for chaining.
var editorMutatorNames = map[string]bool{
	"putString": true, "putInt": true, "putLong": true, "putFloat": true, "putBoolean": true,
	"putStringSet": true, "remove": true, "clear": true,
}

// ApplySharedPrefRule mirrors Lint's ApplySharedPref: `commit()` on an editor from SharedPreferences.edit() whose
// result is not used, where `apply()` would not block.
type ApplySharedPrefRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the editor origin is traced within one function.
func (r *ApplySharedPrefRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ApplySharedPrefRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "commit" || len(call.Args) != 0 || call.ReceiverNode == 0 || flatCallTrailingLambda(file, ctx.Idx) != 0 {
		return
	}
	edit, root := editorOrigin(file, call.ReceiverNode, call.Idx, 0)
	if edit == 0 || !editCallIsSharedPrefs(ctx, edit) {
		return
	}
	if root != "" && editorFinalizedEarlier(file, call.Idx, edit, root) {
		return
	}
	if commitResultUsed(file, ctx.Idx) {
		return
	}
	anchor := commitNameNode(file, ctx.Idx)
	ctx.EmitAt(file.FlatRow(anchor)+1, file.FlatCol(anchor)+1,
		"Consider using `apply()` instead; `commit` writes its data to persistent storage immediately, whereas `apply` will handle it in the background")
}

// editorOrigin traces an editor expression back to the `edit()` call that created it through chained Editor
// methods and function-local variables. root names the variable the trace went through, or "" for a direct chain.
func editorOrigin(file *scanner.File, node, scope uint32, depth int) (edit uint32, root string) {
	if node == 0 || depth > 4 {
		return 0, ""
	}
	switch file.FlatType(node) {
	case "parenthesized_expression":
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				return editorOrigin(file, child, scope, depth)
			}
		}
	case "call_expression", "method_invocation":
		inner, ok := lintCallAt(file, node)
		if !ok || inner.ReceiverNode == 0 {
			return 0, ""
		}
		switch {
		case inner.Name == "edit" && len(inner.Args) == 0 && flatCallTrailingLambda(file, node) == 0:
			return node, ""
		case editorMutatorNames[inner.Name]:
			return editorOrigin(file, inner.ReceiverNode, scope, depth+1)
		}
	case "simple_identifier", "identifier":
		name := file.FlatNodeText(node)
		init := lintInitializerOf(file, scope, name, true)
		if init == 0 || !sameCallable(file, init, scope) {
			return 0, ""
		}
		edit, inner := editorOrigin(file, init, init, depth+1)
		if inner == "" {
			inner = name
		}
		return edit, inner
	}
	return 0, ""
}

func sameCallable(file *scanner.File, a, b uint32) bool {
	fa, okA := flatEnclosingCallable(file, a)
	fb, okB := flatEnclosingCallable(file, b)
	return okA && okB && fa == fb
}

// editorFinalizedEarlier reports whether an `apply()` or `commit()` on the same editor variable comes first in the
// function, which Lint treats as the finalizing call instead.
func editorFinalizedEarlier(file *scanner.File, commit, edit uint32, root string) bool {
	fn, ok := flatEnclosingCallable(file, commit)
	if !ok {
		return false
	}
	found := false
	for _, kind := range lintCallNodeTypes {
		file.FlatWalkNodes(fn, kind, func(idx uint32) {
			c, isCall := lintCallAt(file, idx)
			if found || !isCall || idx == commit || file.FlatStartByte(idx) >= file.FlatStartByte(commit) ||
				(c.Name != "commit" && c.Name != "apply") || len(c.Args) != 0 || c.ReceiverNode == 0 {
				return
			}
			origin, name := editorOrigin(file, c.ReceiverNode, idx, 0)
			found = origin == edit && name == root
		})
	}
	return found
}

// editCallIsSharedPrefs confirms the `edit()` call is SharedPreferences.edit using the oracle, javac facts, or the
// receiver's declared type.
func editCallIsSharedPrefs(ctx *api.Context, edit uint32) bool {
	file := ctx.File
	if file.Language == scanner.LangJava {
		if fact, ok := javaSemanticCallFact(ctx, edit); ok && fact.ReceiverType != "" {
			return javaProfileTypeMatches(ctx, fact.ReceiverType, sharedPreferencesClass)
		}
		call, ok := lintCallAt(file, edit)
		if ok {
			if typ := lintReceiverTypeText(file, call); typ != "" {
				return lintTypeRefMatches(file, typ, sharedPreferencesClass)
			}
		}
		return sourceImportsOrMentions(file, sharedPreferencesClass)
	}
	if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok && cr.Oracle() != nil {
		return commitPrefEditsOracleConfirmed(cr.Oracle(), file, edit)
	}
	call, ok := lintCallAt(file, edit)
	if !ok {
		return false
	}
	if inner, isCall := lintCallAt(file, call.ReceiverNode); isCall && strings.HasSuffix(inner.Name, "SharedPreferences") {
		return true
	}
	return lintReceiverIsType(ctx, call, sharedPreferencesClass)
}

// commitResultUsed mirrors Lint's suggestApplyIfApplicable: the result counts as used when the call is an argument,
// an initializer, an operand, returned, or the condition of a conditional. A call chained further (`commit().let {}`)
// is judged by the enclosing chain.
func commitResultUsed(file *scanner.File, idx uint32) bool {
	node := idx
	for {
		parent, ok := file.FlatParent(node)
		if !ok {
			return false
		}
		next := chainParent(file, parent, node)
		if next == 0 {
			return nodeUsedByParent(file, parent, node)
		}
		node = next
	}
}

// chainParent returns the enclosing node when node is only the receiver of a further call or member access, or is
// parenthesized. It returns 0 otherwise.
func chainParent(file *scanner.File, parent, node uint32) uint32 {
	switch file.FlatType(parent) {
	case "parenthesized_expression":
		return parent
	case "navigation_expression":
		if file.FlatNamedChild(parent, 0) != node {
			return 0
		}
		if gp, ok := file.FlatParent(parent); ok && file.FlatType(gp) == "call_expression" && file.FlatNamedChild(gp, 0) == parent {
			return gp
		}
		return parent
	case "method_invocation":
		if c, ok := lintCallAt(file, parent); ok && c.ReceiverNode == node {
			return parent
		}
	}
	return 0
}

func nodeUsedByParent(file *scanner.File, parent, node uint32) bool {
	switch file.FlatType(parent) {
	case "source_file", "control_structure_body", "expression_statement", "when_entry", "block", "class_body":
		return false
	case "statements":
		gp, ok := file.FlatParent(parent)
		return ok && file.FlatType(gp) == "lambda_literal" && flatLastCodeNamedChild(file, parent) == node
	case "ternary_expression":
		return file.FlatNamedChild(parent, 0) == node
	}
	return true
}

// commitNameNode returns the identifier of the `commit` call, which is where Lint reports.
func commitNameNode(file *scanner.File, idx uint32) uint32 {
	if file.FlatType(idx) != "method_invocation" {
		return calleeStartAnchor(file, idx)
	}
	name := idx
	for child := file.FlatFirstChild(idx); child != 0 && file.FlatType(child) != "argument_list"; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "identifier" {
			name = child
		}
	}
	return name
}
