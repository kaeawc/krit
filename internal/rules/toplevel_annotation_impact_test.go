package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
)

func TestTopLevelAnnotationSuppressDispatcher(t *testing.T) {
	if !rules.IsDefaultActive("MagicNumber") {
		t.Fatal("MagicNumber must be default-active for this impact test")
	}
	findings := runRuleByName(t, "MagicNumber", `package p
@Suppress("MagicNumber")
fun f() { val x = 42 }
fun g() {}
`)
	if len(findings) != 0 {
		t.Fatalf("top-level @Suppress must suppress MagicNumber in f: %+v", findings)
	}
}

func TestTopLevelAnnotationSuppressWarningsDispatcher(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `package p
@SuppressWarnings("MagicNumber")
fun f() { val x = 42 }
fun g() {}
`)
	if len(findings) != 0 {
		t.Fatalf("top-level @SuppressWarnings must suppress MagicNumber in f: %+v", findings)
	}
}

func TestTopLevelAnnotationKritIgnoreCommentDispatcher(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `package p
fun f() {
    val x = 42 // krit:ignore[MagicNumber]
}
fun g() {}
`)
	if len(findings) != 0 {
		t.Fatalf("line comment must suppress MagicNumber: %+v", findings)
	}
}

func TestTopLevelAnnotationDetektCommentDispatcher(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `package p
fun f() {
    val x = 42 // detekt:MagicNumber
}
fun g() {}
`)
	if len(findings) != 1 {
		t.Fatalf("detekt comment syntax is unsupported; want one MagicNumber finding, got %+v", findings)
	}
}
