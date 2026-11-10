package rules_test

import (
	"fmt"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// runUselessCallWithSmartCastOracle gives every `s` identifier the non-null String type the compiler reports
// after a smart cast.
func runUselessCallWithSmartCastOracle(t *testing.T, code string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{}
	file.FlatWalkNodes(0, "simple_identifier", func(idx uint32) {
		if file.FlatNodeText(idx) == "s" {
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.Expressions[file.Path][key] = &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypeClass, Resolved: true}
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == "UselessCallOnNotNull" {
			cols := rules.NewDispatcher([]*api.Rule{r}, composite).Run(file)
			return cols.Findings()
		}
	}
	t.Fatal("rule not registered")
	return nil
}

// Detekt reads the declared type, so a name declared nullable is not flagged after a smart cast.
func TestUselessCallOnNotNull_SmartCastOfNullableDeclarationIsSilent(t *testing.T) {
	got := runUselessCallWithSmartCastOracle(t, `package test
fun a(s: String?) {
    if (s != null) {
        println(s.isNullOrEmpty())
        println(s.orEmpty())
    }
}`)
	if len(got) != 0 {
		t.Fatalf("smart-cast nullable declaration must stay silent, got %v", got)
	}
}

func TestUselessCallOnNotNull_SameNameNonNullInOtherFunctionIsReported(t *testing.T) {
	got := runUselessCallWithSmartCastOracle(t, `package test
fun a(s: String?) {
    println(s.isNullOrEmpty())
}
fun b(s: String) {
    println(s.isNullOrEmpty())
}`)
	if len(got) != 1 || got[0].Line != 6 {
		t.Fatalf("expected one finding on line 6, got %v", got)
	}
}
