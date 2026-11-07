package rules_test

import (
	"sort"
	"testing"
)

func unnecessaryApplyLines(t *testing.T, src string) []int {
	t.Helper()
	var lines []int
	for _, f := range runRuleByName(t, "UnnecessaryApply", src) {
		lines = append(lines, f.Line)
	}
	sort.Ints(lines)
	return lines
}

func expectUnnecessaryApplyLines(t *testing.T, src string, want ...int) {
	t.Helper()
	got := unnecessaryApplyLines(t, src)
	if len(got) != len(want) {
		t.Fatalf("lines = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lines = %v, want %v", got, want)
		}
	}
}

func TestUnnecessaryApply_ExplicitThisSingleStatementIsReported(t *testing.T) {
	expectUnnecessaryApplyLines(t, `
package test

class Box { var size = 0; fun grow() {}; val items = mutableListOf<Int>() }

fun reported(box: Box, other: Box, amount: Int) {
  box.apply { this.grow() }
  box.apply { this.size = amount }
  box.apply { this.items.add(amount) }
  box.apply { this.size += 1 }
  box.apply { this[0] = 1 }
  box.apply { this.also { it.grow() } }
  box.apply { (this).grow() }
  box.apply { this.size = this.size + other.size }
  box.apply { this.size = size }
}
`, 7, 8, 9, 10, 11, 12, 13, 14, 15)
}

func TestUnnecessaryApply_MessageAndAnchor(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryApply", `
package test

class Box { fun grow() {} }

fun reported(box: Box?) {
  box
    ?.apply { this.grow() }
  Box()
    .apply { this.grow() }
}
`)
	if len(findings) != 2 {
		t.Fatalf("findings = %v", findings)
	}
	byLine := map[int]string{}
	for _, f := range findings {
		byLine[f.Line] = f.Message
	}
	if byLine[8] != "apply can be replaced with let or an if" {
		t.Errorf("safe call message = %q", byLine[8])
	}
	if byLine[10] != "apply expression can be omitted" {
		t.Errorf("message = %q", byLine[10])
	}
}

func TestUnnecessaryApply_NotReportedWhenValueIsUsed(t *testing.T) {
	expectUnnecessaryApplyLines(t, `
package test

class Box { fun grow() {} }

fun id(box: Box): Box = box.apply { this.grow() }
fun chained(box: Box) { box.apply { this.grow() }.grow() }
fun assigned(box: Box): Box {
  val x = box.apply { this.grow() }
  return box.apply { this.grow() }
}
fun lambdaResult(box: Box) = run { box.apply { this.grow() } }
fun argument(box: Box) = listOf(box.apply { this.grow() })
fun unbracedIf(box: Box, flag: Boolean) { if (flag) box.apply { this.grow() } }
`)
}

func TestUnnecessaryApply_StatementPositions(t *testing.T) {
	expectUnnecessaryApplyLines(t, `
package test

class Box { fun grow() {} }

fun notLast(box: Box): Int {
  box.apply { this.grow() }
  return 1
}
fun lastInUnitFunction(box: Box) {
  box.apply { this.grow() }
}
fun branches(box: Box, flag: Boolean) {
  if (flag) { box.apply { this.grow() } }
  for (i in 0..1) { box.apply { this.grow() } }
}
class WithInit { init { Box().apply { this.grow() } } }
`, 7, 11, 14, 15, 17)
}

func TestUnnecessaryApply_NotReportedWhenReceiverUseIsAmbiguous(t *testing.T) {
	expectUnnecessaryApplyLines(t, `
package test

class Box { var size = 0; fun grow() {} }
fun topLevel() {}

fun quiet(box: Box, other: Box, amount: Int) {
  box.apply { grow() }
  box.apply { size = 1 }
  box.apply { topLevel() }
  box.apply { other.grow() }
  box.apply { this.grow(); this.grow() }
  box.apply { }
  box.apply { this.size + 1 }
  box.apply { this?.grow() }
  box.apply { this::grow }
  box.apply { this() }
  box.apply { this@quiet.grow() }
  box.apply { run { this.grow() } }
  apply { this.grow() }
  box.apply({ this.grow() })
}
`)
}

func TestUnnecessaryApply_LocalNamesDoNotCountAsReceiverReferences(t *testing.T) {
	expectUnnecessaryApplyLines(t, `
package test

class Box { var size = 0; fun add(x: Int) {} }

fun reported(box: Box, amount: Int) {
  val local = 2
  box.apply { this.add(amount) }
  box.apply { this.add(local) }
  box.apply { this.add(listOf(amount).size) }
}
`, 8, 9, 10)
}

func TestUnnecessaryApply_UserDefinedApplyIsNotStdlib(t *testing.T) {
	expectUnnecessaryApplyLines(t, `
package test

class Box { fun grow() {} }
fun <T> T.apply(block: T.() -> Unit): T = this

fun custom(box: Box) {
  box.apply { this.grow() }
}
`)
}
