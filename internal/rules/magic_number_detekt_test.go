package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// runMagicNumberAsDetekt runs MagicNumber with detekt's default options.
func runMagicNumberAsDetekt(t *testing.T, code string) []scanner.Finding {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID != "MagicNumber" {
			continue
		}
		rule, ok := r.Implementation.(*rules.MagicNumberRule)
		if !ok {
			t.Fatalf("MagicNumber implementation is %T", r.Implementation)
		}
		saved := struct {
			idioms, enums, ranges, prop, local bool
			numbers                            []string
		}{rule.IgnoreIdioms, rule.IgnoreEnums, rule.IgnoreRanges, rule.IgnorePropertyDeclaration, rule.IgnoreLocalVariableDeclaration, rule.IgnoreNumbers}
		rule.IgnoreIdioms = false
		rule.IgnoreEnums = false
		rule.IgnoreRanges = false
		rule.IgnorePropertyDeclaration = false
		rule.IgnoreLocalVariableDeclaration = false
		rule.IgnoreNumbers = []string{"-1", "0", "1", "2"}
		defer func() {
			rule.IgnoreIdioms = saved.idioms
			rule.IgnoreEnums = saved.enums
			rule.IgnoreRanges = saved.ranges
			rule.IgnorePropertyDeclaration = saved.prop
			rule.IgnoreLocalVariableDeclaration = saved.local
			rule.IgnoreNumbers = saved.numbers
		}()
		cols := rules.NewDispatcher([]*api.Rule{r}, nil).Run(parseInline(t, code))
		return cols.Findings()
	}
	t.Fatal("MagicNumber not registered")
	return nil
}

func TestMagicNumberAsDetekt_ReportsAndIgnores(t *testing.T) {
	findings := runMagicNumberAsDetekt(t, `package test
val big = 42
val negativeOne = -1
val negativeTwo = -2
const val LIMIT = 99
fun local() { val q = 10; val h = 0x10; val g = 25_000 }
fun expr() = 77
fun ret(): Int { return 78 }
fun sum(a: Int) = a * 100
fun defaults(x: Int = 5000) = x
enum class E(val v: Int) { A(5) }
val r = 1..10
val unit = 3.dp
class C { companion object { val k = 33 } }
class Hash { override fun hashCode(): Int { return 31 * 7 } }
`)
	want := map[int]bool{2: true, 4: true, 6: true, 9: true, 11: true, 12: true}
	got := map[int]int{}
	for _, f := range findings {
		got[f.Line]++
		if f.Message != "This expression contains a magic number. Consider defining it to a well named constant." {
			t.Errorf("unexpected message %q", f.Message)
		}
	}
	for line := range want {
		if got[line] == 0 {
			t.Errorf("expected a finding on line %d, got %v", line, got)
		}
	}
	for line := range got {
		if !want[line] {
			t.Errorf("unexpected finding on line %d", line)
		}
	}
	if got[6] != 3 {
		t.Errorf("line 6 should report 10, 0x10 and 25_000, got %d", got[6])
	}
}

func TestMagicNumber_IgnoresNumbersByValue(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `package test
fun f() {
    val a = 0x10
    val b = 16.0f
    val c = 5000L
}
`)
	// 16 is in the default krit ignoreNumbers list, whatever its spelling.
	for _, fnd := range findings {
		if fnd.Line == 3 || fnd.Line == 4 {
			t.Errorf("16 should be ignored by value on line %d", fnd.Line)
		}
	}
}
