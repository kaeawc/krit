package rules

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// usesOf returns the simple_identifier nodes named name, in source order.
func usesOf(file *scanner.File, name string) []uint32 {
	var uses []uint32
	file.FlatWalkNodes(0, "simple_identifier", func(n uint32) {
		if file.FlatNodeText(n) == name {
			uses = append(uses, n)
		}
	})
	return uses
}

// A lambda parameter or for-loop variable is the nearest declaration of its name: it hides a local declared before
// its lambda or loop and a typed property, while the loop's own iterable still reads the outer variable.
func TestLintHelpersHonorLambdaAndLoopBinders(t *testing.T) {
	file := parseInlineForInternalTest(t, `package a

class Manager
class Fake

class C(private val m: Manager) {
    fun f(fakes: List<Fake>) {
        val m = Manager()
        fakes.forEach { m -> println(m) }
        for (m in fakes) {
            println(m)
        }
        println(m)
    }
}
`)
	uses := usesOf(file, "m")
	// m's simple identifiers in order: the property, the local, the lambda parameter, the use in the lambda, the loop
	// variable, the use in the loop, the use after the loop.
	if len(uses) != 7 {
		t.Fatalf("found %d uses of m, want 7", len(uses))
	}
	inLambda, inLoop, after := uses[3], uses[5], uses[6]
	for _, c := range []struct {
		name string
		use  uint32
		hide bool
	}{{"lambda", inLambda, true}, {"loop", inLoop, true}, {"after the loop", after, false}} {
		init := lintInitializerOf(file, c.use, "m", true)
		if c.hide && init != 0 {
			t.Errorf("%s: lintInitializerOf resolved the hidden local (%q)", c.name, file.FlatNodeText(init))
		}
		if !c.hide && (init == 0 || !strings.Contains(file.FlatNodeText(init), "Manager()")) {
			t.Errorf("%s: lintInitializerOf = %q, want the local's Manager()", c.name, file.FlatNodeText(init))
		}
		if typ := lintDeclaredTypeText(file, c.use, "m"); c.hide && typ != "" {
			t.Errorf("%s: lintDeclaredTypeText = %q, want none for an untyped binder", c.name, typ)
		}
	}
	if !lintNameIsParameterAt(file, inLoop, "m") || lintNameIsParameterAt(file, after, "m") {
		t.Error("the loop variable binds in the loop body only")
	}
}
