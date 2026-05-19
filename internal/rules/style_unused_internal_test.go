package rules

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func firstFlatNode(file *scanner.File, nodeType string) uint32 {
	var first uint32
	file.FlatWalkNodes(0, nodeType, func(idx uint32) {
		if first == 0 {
			first = idx
		}
	})
	return first
}

// TestUnusedParameterForDeclaresNameFlat_IterableNotADeclaration locks the
// helper that decides whether a for-statement's loop variable binds a given
// name. The iterable expression after `in` is also a child of the
// for_statement, so a naive scan over all children would mistake the iterable
// for the loop variable and silently shadow the outer parameter.
func TestUnusedParameterForDeclaresNameFlat_IterableNotADeclaration(t *testing.T) {
	file := parseInlineForInternalTest(t, `package test
fun process(params: List<String>) {
    for (param in params) {
        println(param)
    }
}
`)
	forStmt := firstFlatNode(file, "for_statement")
	if forStmt == 0 {
		t.Fatal("expected to find a for_statement in fixture")
	}

	if !unusedParameterForDeclaresNameFlat(file, forStmt, "param") {
		t.Errorf("for-statement should declare its loop variable %q", "param")
	}
	if unusedParameterForDeclaresNameFlat(file, forStmt, "params") {
		t.Errorf("for-statement must not be treated as declaring iterable name %q", "params")
	}
	if unusedParameterForDeclaresNameFlat(file, forStmt, "println") {
		t.Errorf("for-statement should not declare unrelated name %q", "println")
	}
}

// TestUnusedParameterForDeclaresNameFlat_DestructuringLoopVar pins the
// multi_variable_declaration branch — for ((k, v) in entries) — so a future
// refactor cannot silently regress destructured-name shadowing while keeping
// the iterable-name fix intact.
func TestUnusedParameterForDeclaresNameFlat_DestructuringLoopVar(t *testing.T) {
	file := parseInlineForInternalTest(t, `package test
fun process(entries: Map<String, String>) {
    for ((k, v) in entries) {
        println("$k=$v")
    }
}
`)
	forStmt := firstFlatNode(file, "for_statement")
	if forStmt == 0 {
		t.Fatal("expected to find a for_statement in fixture")
	}
	if !unusedParameterForDeclaresNameFlat(file, forStmt, "k") {
		t.Errorf("destructured loop var %q should be treated as declared", "k")
	}
	if !unusedParameterForDeclaresNameFlat(file, forStmt, "v") {
		t.Errorf("destructured loop var %q should be treated as declared", "v")
	}
	if unusedParameterForDeclaresNameFlat(file, forStmt, "entries") {
		t.Errorf("iterable %q must not be treated as a loop declaration", "entries")
	}
}

// TestUnusedParameterErrorSubtreeMentionsNameFlat_SoftKeywordReceiver locks
// the fallback used when tree-sitter-kotlin mis-parses a soft-keyword
// identifier (e.g. `annotation`) as a class_modifier under an ERROR subtree.
// Without the fallback the body walk over simple_identifier nodes never sees
// the usage and the parameter is reported as unused.
func TestUnusedParameterErrorSubtreeMentionsNameFlat_SoftKeywordReceiver(t *testing.T) {
	file := parseInlineForInternalTest(t, `package test
class KSAnnotation { fun process() {} }
fun foo(annotation: KSAnnotation) {
    annotation.process()
}
`)
	// Pick the function_body of foo() — the one that contains the
	// `annotation.process()` call; the empty stub body of process() must be
	// skipped or the ERROR subtree won't be in scope.
	var body uint32
	file.FlatWalkNodes(0, "function_body", func(idx uint32) {
		if body == 0 && strings.Contains(file.FlatNodeText(idx), "annotation.process") {
			body = idx
		}
	})
	if body == 0 {
		t.Fatal("expected to find foo()'s function_body in fixture")
	}

	if !unusedParameterErrorSubtreeMentionsNameFlat(file, body, "annotation") {
		t.Errorf("ERROR fallback should mention soft-keyword param name %q", "annotation")
	}
	if unusedParameterErrorSubtreeMentionsNameFlat(file, body, "missing") {
		t.Errorf("ERROR fallback must not invent a mention of unrelated name %q", "missing")
	}
	if unusedParameterErrorSubtreeMentionsNameFlat(file, body, "") {
		t.Errorf("empty name must never be reported as mentioned")
	}
}

// TestUnusedParameterErrorSubtreeMentionsNameFlat_NoErrorReturnsFalse pins
// the posting-list fast path: well-parsed bodies must skip the ERROR walk.
func TestUnusedParameterErrorSubtreeMentionsNameFlat_NoErrorReturnsFalse(t *testing.T) {
	file := parseInlineForInternalTest(t, `package test
fun foo(name: String) {
    println(name)
}
`)
	body := firstFlatNode(file, "function_body")
	if body == 0 {
		t.Fatal("expected function_body")
	}
	if unusedParameterErrorSubtreeMentionsNameFlat(file, body, "name") {
		t.Errorf("well-parsed body must not produce ERROR-fallback mentions")
	}
}
