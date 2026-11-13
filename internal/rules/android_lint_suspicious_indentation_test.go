package rules_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// suspiciousIndentationFindings formats findings as "line:col severity message", ordered by position.
func suspiciousIndentationFindings(findings []scanner.Finding) []string {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Col < findings[j].Col
	})
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, fmt.Sprintf("%d:%d %s %s", f.Line, f.Col, f.Severity, f.Message))
	}
	return out
}

func requireSameFindings(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("findings differ\ngot:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// The expected findings are the ones Android Lint 32.4.1 reports for the same sources.
func TestSuspiciousIndentationKotlinLintParity(t *testing.T) {
	src := `package com.example

fun unit() {}
fun num(): Int = 1

fun nested(x: Int) {
    if (x > 0)
        unit()
        unit()
}

fun nestedLoop(x: Int) {
    for (i in 0..x)
        unit()
        unit()
}

fun nestedWhile(x: Int) {
    while (x > 3)
        unit()
        unit()
}

fun sameLine(x: Int) {
    if (x > 0) unit()
        unit()
}

fun elseBranch(x: Int) {
    if (x > 0)
        unit()
    else
        unit()
        unit()
}

fun elseIfChain(x: Int) {
    if (x > 0) unit()
    else if (x > 1) unit()
    else unit()
        unit()
}

fun braced(x: Int) {
    if (x > 0) {
        unit()
    }
        unit()
}

fun conditionalSameColumn(x: Int) {
    if (x > 0)
    unit()
}

fun loopSameColumn(x: Int) {
    for (i in 0..x)
    unit()
    while (x > 0)
    unit()
}

fun jumpBody(x: Int) {
    if (x > 0) return
        unit()
}

fun jumpThenJump(x: Int) {
    if (x > 0)
    return
    return
}

fun afterDeclaration() {
    val a = 1
        num()
}

fun afterDeclarationUnit() {
    val a = 1
        unit()
}

fun afterDeclarationLiteral() {
    val a = 1
        "text"
}

fun afterDeclarationDeclaration() {
    val a = 1
        val b = 2
}

fun afterAssignment() {
    var a = 1
    a = 2
        num()
}

fun afterAssignmentIncrement() {
    var a = 1
    a = 2
        ++a
}

fun afterBinary(x: Int) {
    x + 1
        unit()
}

fun afterCall() {
    unit()
        unit()
}

fun afterBracedLocal() {
    val f = { }
        unit()
}

fun afterLoopBraced(x: Int) {
    for (i in 0..x) { }
        unit()
}

fun commentedCondition(x: Int) {
    val a = 1
    // if (x > 0)
        unit()
}

fun commentedElsewhere(x: Int) {
    val a = 1
    // note
        num()
}

fun tabs(x: Int) {
    if (x > 0)
        unit()
	unit()
}

fun spaces(x: Int) {
	unit()
        unit()
}

fun inIfBlock(x: Int) {
    if (x > 0) {
        val a = 1
            num()
    }
}

fun inElseBlock(x: Int) {
    if (x > 0) {
    } else {
        val a = 1
            num()
    }
}

fun inForBlock(x: Int) {
    for (i in 0..x) {
        val a = 1
            num()
    }
}

fun inTryBlocks(x: Int) {
    try {
        val a = 1
            num()
    } catch (e: Exception) {
        val b = 1
            num()
    } finally {
        val c = 1
            num()
    }
}

fun inLambda(x: Int) {
    x.let {
        val a = 1
            num()
    }
}

fun inWhenBranch(x: Int) {
    when (x) {
        1 -> {
            val a = 1
                num()
        }
    }
}

class Holder {
    constructor(x: Int) {
        val a = 1
            num()
    }

    init {
        val a = 1
            num()
    }

    val p: Int
        get() {
            val a = 1
                return a
        }
}

fun oneReportPerBlock(x: Int) {
    if (x > 0)
        unit()
        unit()
    if (x > 1)
        unit()
        unit()
}

fun multipleBlocks(x: Int) {
    if (x > 0) {
        if (x > 1)
            unit()
            unit()
    }
    if (x > 2)
        unit()
        unit()
}

fun statementsNotStartingLines(x: Int) {
    unit(); unit()
    if (x > 0) unit(); unit()
}

fun clean(x: Int) {
    val a = 1
    num()
    if (x > 0)
        unit()
    unit()
    for (i in 0..x)
        unit()
    unit()
    val s = x.toString()
        .length
    unit()
    val t = listOf(
        1,
        2,
    )
    unit()
}
`
	want := []string{
		"9:9 error Suspicious indentation: This is indented but is not nested under the previous expression (`if (x > 0)         u`...)",
		"15:9 error Suspicious indentation: This is indented but is not nested under the previous expression (`for (i in 0..x)     `...)",
		"21:9 error Suspicious indentation: This is indented but is not nested under the previous expression (`while (x > 3)       `...)",
		"26:9 error Suspicious indentation: This is indented but is not nested under the previous expression (`if (x > 0) unit`...)",
		"34:9 error Suspicious indentation: This is indented but is not nested under the previous expression (`if (x > 0)         u`...)",
		"41:9 error Suspicious indentation: This is indented but is not nested under the previous expression (`if (x > 0) unit()   `...)",
		"53:5 error Suspicious indentation: This is conditionally executed; expected it to be indented",
		"58:5 error Suspicious indentation: This is conditionally executed; expected it to be indented",
		"60:5 error Suspicious indentation: This is conditionally executed; expected it to be indented",
		"65:9 error Suspicious indentation: This is indented but is not nested under the previous expression (`if (x > 0) return`...)",
		"76:9 error Suspicious indentation: This is indented but is not continuing the previous expression (`val a = 1`...)",
		"81:9 error Suspicious indentation: This is indented but is not continuing the previous expression (`val a = 1`...)",
		"86:9 error Suspicious indentation: This is indented but is not continuing the previous expression (`val a = 1`...)",
		"97:9 error Suspicious indentation: This is indented but is not continuing the previous expression (`a = 2`...)",
		"108:9 error Suspicious indentation: This is indented but is not continuing the previous expression (`x + 1`...)",
		"141:1 warning The indentation string here is different from on the previous line (`\" \"` vs `\\t`)",
		"152:13 error Suspicious indentation: This is indented but is not continuing the previous expression (`val a = 1`...)",
		"160:13 error Suspicious indentation: This is indented but is not continuing the previous expression (`val a = 1`...)",
		"167:13 error Suspicious indentation: This is indented but is not continuing the previous expression (`val a = 1`...)",
		"174:13 error Suspicious indentation: This is indented but is not continuing the previous expression (`val a = 1`...)",
		"177:13 error Suspicious indentation: This is indented but is not continuing the previous expression (`val b = 1`...)",
		"180:13 error Suspicious indentation: This is indented but is not continuing the previous expression (`val c = 1`...)",
		"203:13 error Suspicious indentation: This is indented but is not continuing the previous expression (`val a = 1`...)",
		"208:13 error Suspicious indentation: This is indented but is not continuing the previous expression (`val a = 1`...)",
		"214:17 error Suspicious indentation: This is indented but is not continuing the previous expression (`val a = 1`...)",
		"221:9 error Suspicious indentation: This is indented but is not nested under the previous expression (`if (x > 0)         u`...)",
		"231:13 error Suspicious indentation: This is indented but is not nested under the previous expression (`if (x > 1)          `...)",
		"235:9 error Suspicious indentation: This is indented but is not nested under the previous expression (`if (x > 2)         u`...)",
	}
	requireSameFindings(t, suspiciousIndentationFindings(runRuleByName(t, "SuspiciousIndentation", src)), want)
}

func TestSuspiciousIndentationJavaLintParity(t *testing.T) {
	src := `package com.example;

class T {
    static void unit() {}
    static int num() { return 1; }
    int fld;

    void nested(boolean b) {
        if (b)
            unit();
            unit();
    }

    void nestedLoops(boolean b, int[] xs) {
        for (int i = 0; i < 2; i++)
            unit();
            unit();
    }

    void nestedForEach(int[] xs) {
        for (int x : xs)
            unit();
            unit();
    }

    void nestedWhile(boolean b) {
        while (b)
            unit();
            unit();
    }

    void conditionalSameColumn(boolean b) {
        if (b)
        unit();
    }

    void jumpBody(boolean b) {
        if (b) return;
            unit();
    }

    void elseBranch(boolean b) {
        if (b)
            unit();
        else
            unit();
            unit();
    }

    void braced(boolean b) {
        if (b) {
            unit();
        }
            fld++;
    }

    void doWhileIsNotControl(boolean b) {
        do unit(); while (b);
            fld++;
    }

    void afterAssignment() {
        fld = 1;
            num();
    }

    void afterAssignmentVoid() {
        fld = 1;
            unit();
    }

    void afterAssignmentIncrement() {
        fld = 1;
            fld++;
    }

    void afterAssignmentPrefix() {
        fld = 1;
            ++fld;
    }

    void afterAssignmentNew() {
        fld = 1;
            new Object();
    }

    void afterDeclaration() {
        int a = 1;
            fld++;
    }

    void inIfBlock(boolean b) {
        if (b) {
            fld = 1;
                fld++;
        }
    }

    void inTry() {
        try {
            fld = 1;
                fld++;
        } catch (Exception e) {
            fld = 1;
                fld++;
        }
    }

    void inLambda() {
        Runnable r = () -> {
            fld = 1;
                fld++;
        };
    }

    void inSwitch(int x) {
        switch (x) {
            case 1:
                fld = 1;
                    fld++;
                break;
            default: {
                fld = 1;
                    fld++;
            }
        }
    }

    void inAnonymousClass() {
        Runnable r = new Runnable() {
            public void run() {
                fld = 1;
                    fld++;
            }
        };
    }

    void tabs(boolean b) {
	fld = 1;
        fld++;
    }

    void commentedOut(boolean b) {
        fld = 1;
        // fld = 2
            fld++;
    }

    T() {
        fld = 1;
            fld++;
    }

    static {
        int a = 1;
        a = 2;
            a++;
    }
}
`
	want := []string{
		"11:13 error Suspicious indentation: This is indented but is not nested under the previous expression (`if (b)             u`...)",
		"17:13 error Suspicious indentation: This is indented but is not nested under the previous expression (`for (int i = 0; i < `...)",
		"23:13 error Suspicious indentation: This is indented but is not nested under the previous expression (`for (int x : xs)    `...)",
		"29:13 error Suspicious indentation: This is indented but is not nested under the previous expression (`while (b)           `...)",
		"34:9 error Suspicious indentation: This is conditionally executed; expected it to be indented",
		"39:13 error Suspicious indentation: This is indented but is not nested under the previous expression (`if (b) return`...)",
		"47:13 error Suspicious indentation: This is indented but is not nested under the previous expression (`if (b)             u`...)",
		"64:13 error Suspicious indentation: This is indented but is not continuing the previous expression (`fld = 1`...)",
		"74:13 error Suspicious indentation: This is indented but is not continuing the previous expression (`fld = 1`...)",
		"84:13 error Suspicious indentation: This is indented but is not continuing the previous expression (`fld = 1`...)",
		"95:17 error Suspicious indentation: This is indented but is not continuing the previous expression (`fld = 1`...)",
		"102:17 error Suspicious indentation: This is indented but is not continuing the previous expression (`fld = 1`...)",
		"105:17 error Suspicious indentation: This is indented but is not continuing the previous expression (`fld = 1`...)",
		"112:17 error Suspicious indentation: This is indented but is not continuing the previous expression (`fld = 1`...)",
		"124:21 error Suspicious indentation: This is indented but is not continuing the previous expression (`fld = 1`...)",
		"133:21 error Suspicious indentation: This is indented but is not continuing the previous expression (`fld = 1`...)",
		"140:9 error Suspicious indentation: This is indented but is not continuing the previous expression (`fld = 1`...)",
		"151:13 error Suspicious indentation: This is indented but is not continuing the previous expression (`fld = 1`...)",
		"157:13 error Suspicious indentation: This is indented but is not continuing the previous expression (`a = 2`...)",
	}
	requireSameFindings(t, suspiciousIndentationFindings(runRuleByNameOnJava(t, "SuspiciousIndentation", src)), want)
}
