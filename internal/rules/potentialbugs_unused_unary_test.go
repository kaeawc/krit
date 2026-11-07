package rules_test

import (
	"fmt"
	"sort"
	"testing"
)

// Expectations come from running detekt 1.23.8 with type resolution on the same sources.
func TestUnusedUnaryOperator_MatchesDetekt(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string // line:col message, lines counted from the first line of body
	}{
		{"plain statement", "fun f(x: Int) {\n    +x\n    -x\n}", []string{"2:5 This '+x' is not used", "3:5 This '-x' is not used"}},
		{"after a value", "fun f(x: Int) {\n    val a = x\n    + 3\n}", []string{"3:5 This '+ 3' is not used"}},
		{"after a call", "fun f() {\n    g()\n    -1\n}\nfun g() {}", []string{"3:5 This '-1' is not used"}},
		{"text is the whole binary expression", "fun f(x: Int) {\n    val a = x\n    -x * 2 + 3\n}", []string{"3:5 This '-x * 2 + 3' is not used"}},
		{"two broken lines", "fun f(x: Int) {\n    val a = x\n    - x\n    + x\n}", []string{"3:5 This '- x' is not used", "4:5 This '+ x' is not used"}},
		{"comparison and elvis", "fun f(x: Int) {\n    val a = x\n    -x < 1\n    -x ?: 1\n    -x in 0..3\n}", []string{
			"3:5 This '-x < 1' is not used", "4:5 This '-x ?: 1' is not used", "5:5 This '-x in 0..3' is not used"}},
		{"infix call", "fun f(x: Int) {\n    val a = x\n    -x to 3\n}", []string{"3:5 This '-x to 3' is not used"}},
		{"wrapped assignment", "fun f(x: Int) {\n    var a = 0\n    a =\n        -1\n}", []string{"4:9 This 'a =\n        -1' is not used"}},
		{"comment before the operator", "fun f(x: Int) {\n    val a = x // c\n    /* c */ -1\n}", []string{"3:13 This '-1' is not used"}},
		{"branch of a statement if", "fun f(b: Boolean, x: Int) {\n    if (b) {\n        x\n        -1\n    } else {\n        +2\n    }\n}", []string{
			"4:9 This '-1' is not used", "6:9 This '+2' is not used"}},
		{"branch of a statement when", "fun f(x: Int) {\n    when (x) {\n        1 -> {\n            x\n            -1\n        }\n        else -> Unit\n    }\n}", []string{
			"5:13 This '-1' is not used"}},
		{"loop bodies", "fun f(x: Int) {\n    for (i in 0..x) {\n        -i\n    }\n    while (x > 0) {\n        -x\n        break\n    }\n}", []string{
			"3:9 This '-i' is not used", "6:9 This '-x' is not used"}},
		{"try catch finally as statements", "fun f(x: Int) {\n    try {\n        -x\n    } catch (e: Exception) {\n        -x\n    } finally {\n        -x\n    }\n}", []string{
			"3:9 This '-x' is not used", "5:9 This '-x' is not used", "7:9 This '-x' is not used"}},
		{"finally of a try expression", "fun f(x: Int): Int {\n    return try {\n        x\n    } finally {\n        -3\n    }\n}", []string{"5:9 This '-3' is not used"}},
		{"unit lambdas", "fun f(l: List<Int>) {\n    l.forEach {\n        -it\n    }\n    repeat(3) {\n        -it\n    }\n}", []string{
			"3:9 This '-it' is not used", "6:9 This '-it' is not used"}},
		{"parenthesized statement", "fun f(x: Int) {\n    (\n        -x\n    )\n}", []string{"3:9 This '-x' is not used"}},
		{"member init and accessor blocks", "class K(val x: Int) {\n    init {\n        x\n        -x\n    }\n    val p: Int\n        get() {\n            x\n            -x\n            return x\n        }\n}", []string{
			"4:9 This '-x' is not used", "9:13 This '-x' is not used"}},
		{"indexed augmented assignment", "fun f(a: IntArray) {\n    a[0] +=\n        -1\n}", []string{"3:9 This 'a[0] +=\n        -1' is not used"}},

		{"same line", "fun f(x: Int) {\n    x + 1; -x\n}", nil},
		{"used values", "fun f(x: Int): Int {\n    val a = -x\n    g(-x,\n        +x)\n    return -x\n}\nfun g(a: Int, b: Int) {}", nil},
		{"inside parentheses", "fun f(x: Int) {\n    val a = (x\n        + 3)\n    g(x\n        - 3)\n}\nfun g(a: Int) {}", nil},
		{"trailing operator", "fun f(x: Int) {\n    val a = x +\n        -3\n    val b = x ==\n        -1\n}", nil},
		{"lambda and branch results", "fun f(b: Boolean, x: Int): Int {\n    val a = run {\n        x\n        -1\n    }\n    val c = if (b) {\n        x\n        -1\n    } else {\n        +2\n    }\n    val d = when (x) {\n        1 ->\n            -1\n        else -> {\n            0\n            -2\n        }\n    }\n    return if (b) {\n        x\n        -1\n    } else a\n}", nil},
		{"try expression result", "fun f(x: Int): Int {\n    val r = try {\n        g()\n        -x\n    } catch (e: Exception) {\n        +x\n    }\n    return r\n}\nfun g() {}", nil},
		{"operand of as and is", "fun f(x: Int) {\n    val a = x\n    -x as Int\n    -x is Int\n}", nil},
		{"augmented assignment", "fun f(x: Int) {\n    var a = 0\n    a +=\n        -1\n    a -=\n        -1\n}", nil},
		{"string template", "fun f(x: Int): String {\n    return \"${x\n        - 1}\"\n}", nil},
		{"prefix operators other than plus and minus", "fun f(b: Boolean, x: Int) {\n    var i = x\n    !b\n    ++i\n    --i\n}", nil},
		{"after a keyword that needs a value", "fun f(b: Boolean, x: Int): Int {\n    val a = if (b) 1 else\n        -1\n    return a\n}", nil},
		{"suppressed", "@Suppress(\"UnusedUnaryOperator\")\nfun f(x: Int) {\n    val a = x\n    -x\n}", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, f := range runRuleByName(t, "UnusedUnaryOperator", "package test\n"+tc.body+"\n") {
				got = append(got, fmt.Sprintf("%d:%d %s", f.Line-1, f.Col, f.Message))
			}
			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("findings = %q\nwant      %q", got, want)
			}
		})
	}
}

func TestUnusedUnaryOperator_FixRemovesOnlySideEffectFreeLines(t *testing.T) {
	removable := runRuleByName(t, "UnusedUnaryOperator", "package test\nfun f(x: Int) {\n    -x\n}\n")
	if len(removable) != 1 || removable[0].Fix == nil {
		t.Fatalf("a plain unused operator should carry a fix: %+v", removable)
	}
	for name, src := range map[string]string{
		"call":       "package test\nfun f(x: Int) {\n    val a = x\n    -g()\n}\nfun g() = 1\n",
		"same line":  "package test\nfun f(x: Int) {\n    val a = x\n    -x; g()\n}\nfun g() = 1\n",
		"assignment": "package test\nfun f(x: Int) {\n    var a = 0\n    a =\n        -x\n}\n",
	} {
		got := runRuleByName(t, "UnusedUnaryOperator", src)
		if len(got) != 1 || got[0].Fix != nil {
			t.Errorf("%s: want one finding without a fix, got %+v", name, got)
		}
	}
}
