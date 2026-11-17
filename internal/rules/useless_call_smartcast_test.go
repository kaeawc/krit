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

// byTextOracle answers expression types by node text, so a navigation and its owner, which start at the same
// position, get different types as with the real oracle's byte ranges.
type byTextOracle struct {
	*oracle.FakeOracle
	types map[string]*typeinfer.ResolvedType
}

func (o byTextOracle) LookupExpressionFlat(file *scanner.File, idx uint32) *typeinfer.ResolvedType {
	return o.types[file.FlatNodeText(idx)]
}

// The oracle records `p.subtitle` as non-null after the contract smart cast; detekt reads the declared type.
func TestUselessCallOnNotNull_SmartCastMemberDeclaredNullableIsSilent(t *testing.T) {
	file := parseInline(t, `package test
class Po(val subtitle: CharSequence?, val title: CharSequence)
fun f(p: Po) {
    if (p.subtitle.isNullOrBlank()) return
    println(p.subtitle.isNullOrBlank())
    println(p.title.isNullOrBlank())
}`)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.AnalyzedFiles[file.Path] = true
	nullableCs := &typeinfer.ResolvedType{Name: "CharSequence", FQN: "kotlin.CharSequence", Kind: typeinfer.TypeClass, Nullable: true, Resolved: true}
	cs := &typeinfer.ResolvedType{Name: "CharSequence", FQN: "kotlin.CharSequence", Kind: typeinfer.TypeClass, Resolved: true}
	fake.Classes["test.Po"] = &typeinfer.ClassInfo{Name: "Po", FQN: "test.Po", Kind: "class", Members: []typeinfer.MemberInfo{
		{Name: "subtitle", Kind: "property", Type: nullableCs},
		{Name: "title", Kind: "property", Type: cs},
	}}
	o := byTextOracle{FakeOracle: fake, types: map[string]*typeinfer.ResolvedType{
		"p":          {Name: "Po", FQN: "test.Po", Kind: typeinfer.TypeClass, Resolved: true},
		"p.subtitle": cs,
		"p.title":    cs,
	}}
	composite := oracle.NewCompositeResolver(o, resolver)
	for _, r := range api.Registry {
		if r.ID == "UselessCallOnNotNull" {
			cols := rules.NewDispatcher([]*api.Rule{r}, composite).Run(file)
			got := cols.Findings()
			if len(got) != 1 || got[0].Line != 6 {
				t.Fatalf("only the property declared non-null is useless, got %v", got)
			}
			return
		}
	}
	t.Fatal("rule not registered")
}
