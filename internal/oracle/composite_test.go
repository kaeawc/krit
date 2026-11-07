package oracle

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// stubExprOracle implements Lookup plus flatExpressionLookup so we can
// exercise the new path in CompositeResolver.ResolveByNameFlat without
// needing a parsed scanner.File for byte-position keying.
type stubExprOracle struct {
	exprResult  *typeinfer.ResolvedType
	classResult *typeinfer.ClassInfo
}

func (s *stubExprOracle) LookupClass(_ string) *typeinfer.ClassInfo { return s.classResult }
func (s *stubExprOracle) LookupSealedVariants(_ string) []string    { return nil }
func (s *stubExprOracle) LookupEnumEntries(_ string) []string       { return nil }
func (s *stubExprOracle) IsSubtype(_, _ string) bool                { return false }
func (s *stubExprOracle) Dependencies() map[string]*Class           { return nil }
func (s *stubExprOracle) LookupFunction(_ string) *typeinfer.ResolvedType {
	return nil
}
func (s *stubExprOracle) LookupExpression(_ string, _, _ int) *typeinfer.ResolvedType {
	return s.exprResult
}
func (s *stubExprOracle) LookupExpressionFlat(_ *scanner.File, _ uint32) *typeinfer.ResolvedType {
	return s.exprResult
}
func (s *stubExprOracle) LookupAnnotations(_ string) []string        { return nil }
func (s *stubExprOracle) LookupCallTarget(_ string, _, _ int) string { return "" }
func (s *stubExprOracle) LookupCallTargetSuspend(_ string, _, _ int) (bool, bool) {
	return false, false
}
func (s *stubExprOracle) LookupCallTargetAnnotations(_ string, _, _ int) []string { return nil }
func (s *stubExprOracle) LookupDiagnostics(_ string) []Diagnostic                 { return nil }

var _ Lookup = (*stubExprOracle)(nil)

func TestCompositeResolver_ResolveByNameFlat_SourceWinsOverOracleExpression(t *testing.T) {
	srcType := &typeinfer.ResolvedType{Name: "Int", FQN: "kotlin.Int", Kind: typeinfer.TypeClass, Resolved: true}
	fallback := &fakeTypeResolver{nameResult: srcType}

	stub := &stubExprOracle{
		exprResult: &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypeClass},
	}

	c := NewCompositeResolver(stub, fallback)
	got := c.ResolveByNameFlat("x", 0, nil)
	if got != srcType {
		t.Fatalf("expected source-resolved type to win; got %#v", got)
	}
}

func TestCompositeResolver_ResolveByNameFlat_UnresolvedSourceFallsThroughToOracle(t *testing.T) {
	srcType := &typeinfer.ResolvedType{Name: "Unknown", FQN: "Unknown", Kind: typeinfer.TypeClass}
	fallback := &fakeTypeResolver{nameResult: srcType}

	exprType := &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypeClass}
	stub := &stubExprOracle{
		exprResult: exprType,
	}

	c := NewCompositeResolver(stub, fallback)
	got := c.ResolveByNameFlat("x", 0, nil)
	if got != exprType {
		t.Fatalf("expected oracle expression fact to win over unresolved source type; got %#v", got)
	}
}

func TestCompositeResolver_ResolveByNameFlat_OracleExpressionWinsOverLookupClass(t *testing.T) {
	fallback := &fakeTypeResolver{}

	exprType := &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypeClass}
	stub := &stubExprOracle{
		exprResult:  exprType,
		classResult: &typeinfer.ClassInfo{Name: "Other", FQN: "pkg.Other", Kind: "class"},
	}

	c := NewCompositeResolver(stub, fallback)
	got := c.ResolveByNameFlat("s", 0, nil)
	if got != exprType {
		t.Fatalf("expected oracle expression fact to win over LookupClass; got %#v", got)
	}
}

func TestCompositeResolver_ResolveByNameFlat_BothNilReturnsNil(t *testing.T) {
	fallback := &fakeTypeResolver{}
	stub := &stubExprOracle{}

	c := NewCompositeResolver(stub, fallback)
	if got := c.ResolveByNameFlat("missing", 0, nil); got != nil {
		t.Fatalf("expected nil when neither source nor oracle resolve; got %#v", got)
	}
}

func TestSmartCastCrossesModules(t *testing.T) {
	modules := map[string]string{
		"/repo/app/src/main/kotlin/Use.kt":     ":app",
		"/repo/app/src/test/kotlin/UseTest.kt": ":app",
		"/repo/lib/src/main/kotlin/Model.kt":   ":lib",
	}
	moduleOf := func(path string) string { return modules[path] }
	diag := func(decls ...string) Diagnostic {
		return Diagnostic{FactoryName: "UNNECESSARY_SAFE_CALL", SmartCastDecls: decls}
	}
	c := NewCompositeResolver(NewFakeOracle(), typeinfer.NewResolver())
	if c.SmartCastCrossesModules("/repo/app/src/main/kotlin/Use.kt", diag("/repo/lib/src/main/kotlin/Model.kt")) {
		t.Fatal("without a module map every verdict must stand")
	}
	c.SetModuleOf(moduleOf)
	cases := []struct {
		name string
		file string
		d    Diagnostic
		want bool
	}{
		{"another module", "/repo/app/src/main/kotlin/Use.kt", diag("/repo/lib/src/main/kotlin/Model.kt"), true},
		{"friend source set", "/repo/app/src/test/kotlin/UseTest.kt", diag("/repo/app/src/main/kotlin/Use.kt"), false},
		{"any link of the chain", "/repo/app/src/test/kotlin/UseTest.kt", diag("/repo/app/src/main/kotlin/Use.kt", "/repo/lib/src/main/kotlin/Model.kt"), true},
		{"no smart cast", "/repo/app/src/main/kotlin/Use.kt", diag(), false},
		{"unknown declaring module", "/repo/app/src/main/kotlin/Use.kt", diag("/elsewhere/Gen.kt"), true},
		{"undeterminable declaration", "/repo/app/src/main/kotlin/Use.kt", diag(UnknownSmartCastDeclaration), true},
		{"unknown beside same module", "/repo/app/src/main/kotlin/Use.kt", diag("/repo/app/src/main/kotlin/Use.kt", UnknownSmartCastDeclaration), true},
		{"unknown file module", "/elsewhere/Use.kt", diag("/repo/lib/src/main/kotlin/Model.kt"), false},
	}
	for _, tc := range cases {
		if got := c.SmartCastCrossesModules(tc.file, tc.d); got != tc.want {
			t.Errorf("%s: SmartCastCrossesModules = %v, want %v", tc.name, got, tc.want)
		}
	}
	c.SetModuleOf(nil)
	if c.SmartCastCrossesModules("/repo/app/src/main/kotlin/Use.kt", diag("/repo/lib/src/main/kotlin/Model.kt")) {
		t.Fatal("a cleared module map must keep every verdict")
	}
}

func TestCompositeResolverIgnoresCrossModuleSmartCastTypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app", "Use.kt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fun f(a: Account) = a.profile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseKotlinFileCached(context.Background(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	var nav uint32
	file.FlatWalkNodes(0, "navigation_expression", func(idx uint32) { nav = idx })
	if nav == 0 {
		t.Fatal("expected a navigation expression")
	}
	decl := filepath.Join(dir, "lib", "Account.kt")
	o, err := LoadFromData(&Data{
		Version: 1, KotlinVersion: "2.1.0",
		Files: map[string]*File{path: {Expressions: map[string]*ExpressionType{
			"1:21": {
				Type: "com.example.Profile", StartByte: int(file.FlatStartByte(nav)), EndByte: int(file.FlatEndByte(nav)),
				SmartCastDecls: []string{decl},
			},
		}}},
		Dependencies: map[string]*Class{},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := NewCompositeResolver(o, typeinfer.NewResolver())
	if got := c.OracleExpressionFlat(file, nav); got == nil || got.FQN != "com.example.Profile" {
		t.Fatalf("without a module map the smart-cast type must stand, got %#v", got)
	}
	modules := map[string]string{path: ":app", decl: ":lib"}
	c.SetModuleOf(func(p string) string { return modules[p] })
	if got := c.OracleExpressionFlat(file, nav); got != nil {
		t.Fatalf("a smart cast of another module's property must not be authoritative, got %#v", got)
	}
	if got := c.ResolveFlatNode(nav, file); got != nil && got.FQN == "com.example.Profile" && !got.IsNullable() {
		t.Fatalf("ResolveFlatNode must not return the cross-module smart-cast type, got %#v", got)
	}
	modules[decl] = ":app"
	if got := c.OracleExpressionFlat(file, nav); got == nil || got.FQN != "com.example.Profile" {
		t.Fatalf("a smart cast within one module must stand, got %#v", got)
	}
}
