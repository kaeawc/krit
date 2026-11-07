package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// spanText returns the source text a finding spans, failing when it has no span.
func spanText(t *testing.T, code string, f scanner.Finding) string {
	t.Helper()
	if f.EndByte <= f.StartByte || f.EndByte > len(code) {
		t.Fatalf("finding has no usable span: %+v", f)
	}
	return code[f.StartByte:f.EndByte]
}

func TestRecyclerAdapterWithoutDiffUtil_AnchorsOnNotifyCall(t *testing.T) {
	for _, modifier := range []string{"", "private "} {
		code := "package test\nimport androidx.recyclerview.widget.RecyclerView\n" + modifier +
			"class RowAdapter : RecyclerView.Adapter<RecyclerView.ViewHolder>() {\n    fun refresh() { notifyDataSetChanged() }\n}\n"
		findings := runRuleByName(t, "RecyclerAdapterWithoutDiffUtil", code)
		if len(findings) != 1 {
			t.Fatalf("modifier %q: expected 1 finding, got %d", modifier, len(findings))
		}
		if got := spanText(t, code, findings[0]); got != "notifyDataSetChanged" {
			t.Errorf("modifier %q: span %q, want the notify call", modifier, got)
		}
	}
}

func TestTooGenericExceptionCaught_SpansCaughtParameter(t *testing.T) {
	code := "package test\nfun f() {\n    try {\n        work()\n    } catch (e: Exception) {\n        cleanup()\n    }\n}\nfun work() {}\nfun cleanup() {}\n"
	findings := runRuleByName(t, "TooGenericExceptionCaught", code)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if got := spanText(t, code, findings[0]); got != "e" {
		t.Errorf("span %q, want the caught parameter name that detekt reports at", got)
	}
}

func TestUnusedVariableAndPrivateProperty_AnchorOnIdentifier(t *testing.T) {
	code := "package test\nclass Box {\n    private val stale = 1\n    fun f() {\n        val leftover = compute()\n    }\n    fun compute() = 2\n}\n"
	for rule, want := range map[string]string{"UnusedVariable": "leftover", "UnusedPrivateProperty": "stale"} {
		findings := runRuleByName(t, rule, code)
		if len(findings) != 1 {
			t.Fatalf("%s: expected 1 finding, got %d", rule, len(findings))
		}
		if got := spanText(t, code, findings[0]); got != want {
			t.Errorf("%s: span %q, want %q", rule, got, want)
		}
	}
}

func TestCursorLoopWithColumnIndexInLoop_MessageNamesReceiverAndCallee(t *testing.T) {
	code := "package test\nfun f(peopleCursor: android.database.Cursor) {\n    while (peopleCursor.moveToNext()) {\n        peopleCursor.getColumnIndexOrThrow(\"a\")\n    }\n}\n"
	findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.HasPrefix(findings[0].Message, "peopleCursor.getColumnIndexOrThrow() inside while loop") {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestUnreachableCatchBlock_SpansCatchKeyword(t *testing.T) {
	code := "package test\nfun f() {\n    try {\n        work()\n    } catch (e: Exception) {\n        handle(e)\n    } catch (e: Exception) {\n        handle(e)\n    }\n}\nfun work() {}\nfun handle(e: Exception) {}\n"
	findings := runRuleByName(t, "UnreachableCatchBlock", code)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if got := spanText(t, code, findings[0]); got != "catch" {
		t.Errorf("span %q, want the catch keyword", got)
	}
}

func TestAssertNullableWithNotNullAssertion_RequiresDeclaredNullableOperand(t *testing.T) {
	file := parseInline(t, `
package test

import com.google.common.truth.Truth.assertThat

fun testItems(maybe: Item?, other: String?, field: EditField) {
    assertThat(maybe!!.name).isEqualTo("x")
    assertThat(other!!.length).isEqualTo(1)
    assertThat(field.text!!.getSpans().size).isEqualTo(1)
}
`)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	var findings []scanner.Finding
	for _, r := range api.Registry {
		if r.ID == "AssertNullableWithNotNullAssertion" {
			cols := rules.NewDispatcher([]*api.Rule{r}, resolver).Run(file)
			findings = cols.Findings()
		}
	}
	lines := map[int]bool{}
	for _, f := range findings {
		lines[f.Line] = true
	}
	// Lines 7 and 8 assert a declared-nullable operand; line 9 is a platform value of unknown nullability.
	if len(findings) != 2 || !lines[7] || !lines[8] {
		t.Fatalf("expected findings on the declared-nullable operands only, got %+v", findings)
	}
}
