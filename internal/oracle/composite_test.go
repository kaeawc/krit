package oracle

import (
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
	srcType := &typeinfer.ResolvedType{Name: "Int", FQN: "kotlin.Int", Kind: typeinfer.TypeClass}
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
