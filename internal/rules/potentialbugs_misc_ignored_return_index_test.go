package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/filefacts"
)

func TestIgnoredReturnValueHelperResultUsedElsewhereIndexedMatchesWalk(t *testing.T) {
	prev := fileFactsCache()
	setSharedFileFacts(filefacts.NewCache())
	t.Cleanup(func() { setSharedFileFacts(prev) })

	file := parseInlineKt(t, `
fun helper(): Int = 1
fun other(): Int = 2
fun g() {
    helper()
    val x = helper()
    other()
    other()
    println(other())
    obj.helper()
}
`)
	walk := func(idx uint32, name string) bool {
		used := false
		file.FlatWalkNodes(0, "call_expression", func(call uint32) {
			if used || call == idx || flatCallExpressionName(file, call) != name {
				return
			}
			used = flatIsUsedAsExpression(file, call)
		})
		return used
	}
	n := 0
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		name := flatCallExpressionName(file, idx)
		if name == "" || !ignoredReturnValueFileDeclaresFunction(file, name) {
			return
		}
		n++
		if got, want := ignoredReturnValueHelperResultUsedElsewhere(file, idx, name), walk(idx, name); got != want {
			t.Errorf("%s at %d: got %v, want %v", name, file.FlatRow(idx)+1, got, want)
		}
	})
	if n < 5 {
		t.Fatalf("only %d helper calls checked", n)
	}
}
