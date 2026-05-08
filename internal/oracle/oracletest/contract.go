// Package oracletest provides a shared contract test that every
// implementation of oracle.Lookup must satisfy. Running the contract
// against both the real *oracle.Oracle and the *oracle.FakeOracle
// catches drift between the two — for example, a fake that silently
// relaxes a guarantee the real implementation provides.
//
// Usage from a *_test.go in the oracle package:
//
//	func TestRealOracleSatisfiesContract(t *testing.T) {
//	    oracletest.RunContract(t, "Oracle", oracletest.LoadFromDataBuilder)
//	}
//	func TestFakeOracleSatisfiesContract(t *testing.T) {
//	    oracletest.RunContract(t, "FakeOracle", oracletest.FakeOracleBuilder)
//	}
//
// The contract intentionally restricts assertions to behaviors both
// implementations populate the same way: lookup-by-FQN, exact-key
// reads, and per-file isolation. Implementation-specific extras
// (the real oracle's simple-name fallback, transitive supertypes,
// hit/miss counters) belong in their own implementation tests.
package oracletest

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// PositionFact pairs a (file, line, col) coordinate with the values
// expected by the various position-keyed lookups.
type PositionFact struct {
	File        string
	Line        int
	Col         int
	Type        *typeinfer.ResolvedType
	CallTarget  string
	Suspend     *bool // nil means "no resolved-target evidence"
	Annotations []string
}

// Spec describes the data both implementations must be primed with.
// Builders consume it and return a Lookup configured to answer the
// queries the contract makes.
type Spec struct {
	// Classes maps FQN to the class info that LookupClass(FQN) must return.
	Classes map[string]*typeinfer.ClassInfo
	// SealedVariants maps the parent FQN to the variant simple names.
	SealedVariants map[string][]string
	// EnumEntries maps the enum FQN to the entry names.
	EnumEntries map[string][]string
	// DirectSupertypes maps a type FQN to its direct supertype FQNs.
	// Only direct relationships are asserted — transitive subtypes are
	// implementation-specific.
	DirectSupertypes map[string][]string
	// Functions maps lookup keys to the type LookupFunction must return.
	Functions map[string]*typeinfer.ResolvedType
	// Annotations maps lookup keys to annotation FQNs.
	Annotations map[string][]string
	// Positions lists the position-keyed facts both implementations
	// must be able to answer.
	Positions []PositionFact
	// Diagnostics maps file path to the diagnostics LookupDiagnostics
	// must return for that file.
	Diagnostics map[string][]oracle.Diagnostic
}

// Builder produces a configured Lookup from a Spec. Implementations
// supply their own builder so the contract can construct each variant.
type Builder func(t *testing.T, spec Spec) oracle.Lookup

// runEmptyLookupContract verifies that an empty Lookup returns zero values for all queries.
func runEmptyLookupContract(t *testing.T, name string, build Builder) {
	t.Helper()
	t.Run(name+"/empty Lookup returns zero values", func(t *testing.T) {
		l := build(t, Spec{})
		if got := l.LookupClass("com.acme.Nope"); got != nil {
			t.Errorf("LookupClass on empty = %+v, want nil", got)
		}
		if got := l.LookupSealedVariants("com.acme.Sealed"); len(got) != 0 {
			t.Errorf("LookupSealedVariants on empty = %v, want empty", got)
		}
		if got := l.LookupEnumEntries("com.acme.Enum"); len(got) != 0 {
			t.Errorf("LookupEnumEntries on empty = %v, want empty", got)
		}
		if got := l.LookupFunction("Foo.bar"); got != nil {
			t.Errorf("LookupFunction on empty = %+v, want nil", got)
		}
		if got := l.LookupExpression("missing.kt", 1, 1); got != nil {
			t.Errorf("LookupExpression on empty = %+v, want nil", got)
		}
		if got := l.LookupAnnotations("Foo"); len(got) != 0 {
			t.Errorf("LookupAnnotations on empty = %v, want empty", got)
		}
		if got := l.LookupCallTarget("missing.kt", 1, 1); got != "" {
			t.Errorf("LookupCallTarget on empty = %q, want empty", got)
		}
		if isSuspend, ok := l.LookupCallTargetSuspend("missing.kt", 1, 1); ok || isSuspend {
			t.Errorf("LookupCallTargetSuspend on empty = (%v, %v), want (false, false)", isSuspend, ok)
		}
		if got := l.LookupCallTargetAnnotations("missing.kt", 1, 1); len(got) != 0 {
			t.Errorf("LookupCallTargetAnnotations on empty = %v, want empty", got)
		}
		if got := l.LookupDiagnostics("missing.kt"); len(got) != 0 {
			t.Errorf("LookupDiagnostics on empty = %v, want empty", got)
		}
	})
}

// runSubtypeContract verifies IsSubtype behavior.
func runSubtypeContract(t *testing.T, name string, build Builder) {
	t.Helper()
	t.Run(name+"/IsSubtype is reflexive", func(t *testing.T) {
		l := build(t, Spec{})
		if !l.IsSubtype("kotlin.String", "kotlin.String") {
			t.Error("IsSubtype(T, T) = false, want true")
		}
		if l.IsSubtype("kotlin.String", "kotlin.Int") {
			t.Error("IsSubtype(String, Int) = true, want false")
		}
	})

	t.Run(name+"/IsSubtype reports direct supertypes", func(t *testing.T) {
		l := build(t, Spec{
			DirectSupertypes: map[string][]string{
				"com.acme.Foo": {"com.acme.Base"},
			},
		})
		if !l.IsSubtype("com.acme.Foo", "com.acme.Base") {
			t.Error("IsSubtype on direct supertype = false, want true")
		}
		if l.IsSubtype("com.acme.Foo", "com.acme.Other") {
			t.Error("IsSubtype on unrelated = true, want false")
		}
	})
}

// runTypeIndexContract verifies LookupClass, LookupSealedVariants, LookupEnumEntries,
// LookupFunction, and LookupAnnotations.
func runTypeIndexContract(t *testing.T, name string, build Builder) {
	t.Helper()
	t.Run(name+"/LookupClass returns registered FQN", func(t *testing.T) {
		want := &typeinfer.ClassInfo{Name: "Foo", FQN: "com.acme.Foo", Kind: "class"}
		l := build(t, Spec{
			Classes: map[string]*typeinfer.ClassInfo{
				"com.acme.Foo": want,
			},
		})
		got := l.LookupClass("com.acme.Foo")
		if got == nil {
			t.Fatal("LookupClass returned nil for registered FQN")
		}
		if got.FQN != want.FQN {
			t.Errorf("got.FQN = %q, want %q", got.FQN, want.FQN)
		}
		if got.Name != want.Name {
			t.Errorf("got.Name = %q, want %q", got.Name, want.Name)
		}
	})

	t.Run(name+"/LookupSealedVariants returns registered variants", func(t *testing.T) {
		l := build(t, Spec{
			SealedVariants: map[string][]string{
				"com.acme.Sealed": {"VariantA", "VariantB"},
			},
		})
		assertStringSetEqual(t, "LookupSealedVariants", l.LookupSealedVariants("com.acme.Sealed"), []string{"VariantA", "VariantB"})
	})

	t.Run(name+"/LookupEnumEntries returns registered entries", func(t *testing.T) {
		l := build(t, Spec{
			EnumEntries: map[string][]string{
				"com.acme.Color": {"RED", "GREEN", "BLUE"},
			},
		})
		assertStringSetEqual(t, "LookupEnumEntries", l.LookupEnumEntries("com.acme.Color"), []string{"RED", "GREEN", "BLUE"})
	})

	t.Run(name+"/LookupFunction returns registered key", func(t *testing.T) {
		want := &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypeClass}
		l := build(t, Spec{
			Functions: map[string]*typeinfer.ResolvedType{
				"com.acme.Foo.bar": want,
			},
		})
		got := l.LookupFunction("com.acme.Foo.bar")
		if got == nil {
			t.Fatal("LookupFunction returned nil for registered key")
		}
		if got.Name != want.Name {
			t.Errorf("got.Name = %q, want %q", got.Name, want.Name)
		}
	})

	t.Run(name+"/LookupAnnotations returns registered key", func(t *testing.T) {
		l := build(t, Spec{
			Annotations: map[string][]string{
				"com.acme.Foo": {"kotlin.Deprecated"},
			},
		})
		assertStringSetEqual(t, "LookupAnnotations", l.LookupAnnotations("com.acme.Foo"), []string{"kotlin.Deprecated"})
	})
}

// runPositionContract verifies position-keyed lookups (expression, call-target,
// suspend, call-target annotations).
func runPositionContract(t *testing.T, name string, build Builder) {
	t.Helper()
	t.Run(name+"/LookupExpression respects file scope", func(t *testing.T) {
		typ := &typeinfer.ResolvedType{Name: "Int", FQN: "kotlin.Int", Kind: typeinfer.TypePrimitive}
		l := build(t, Spec{
			Positions: []PositionFact{
				{File: "a.kt", Line: 10, Col: 5, Type: typ},
			},
		})
		if got := l.LookupExpression("a.kt", 10, 5); got == nil || got.Name != "Int" {
			t.Errorf("LookupExpression(a.kt, 10, 5) = %+v, want Int", got)
		}
		if got := l.LookupExpression("b.kt", 10, 5); got != nil {
			t.Errorf("LookupExpression in different file = %+v, want nil", got)
		}
		if got := l.LookupExpression("a.kt", 99, 99); got != nil {
			t.Errorf("LookupExpression at unregistered position = %+v, want nil", got)
		}
	})

	t.Run(name+"/LookupCallTarget returns registered FQN", func(t *testing.T) {
		l := build(t, Spec{
			Positions: []PositionFact{
				{File: "a.kt", Line: 7, Col: 3, CallTarget: "com.acme.Foo.bar"},
			},
		})
		if got := l.LookupCallTarget("a.kt", 7, 3); got != "com.acme.Foo.bar" {
			t.Errorf("LookupCallTarget = %q, want com.acme.Foo.bar", got)
		}
		if got := l.LookupCallTarget("b.kt", 7, 3); got != "" {
			t.Errorf("LookupCallTarget in different file = %q, want empty", got)
		}
	})

	t.Run(name+"/LookupCallTargetSuspend distinguishes unknown from non-suspend", func(t *testing.T) {
		yes := true
		no := false
		l := build(t, Spec{
			Positions: []PositionFact{
				{File: "a.kt", Line: 1, Col: 1, CallTarget: "com.acme.Foo.suspend1", Suspend: &yes},
				{File: "a.kt", Line: 2, Col: 1, CallTarget: "com.acme.Foo.regular", Suspend: &no},
			},
		})
		if isSuspend, ok := l.LookupCallTargetSuspend("a.kt", 1, 1); !ok || !isSuspend {
			t.Errorf("Suspend at (1,1) = (%v, %v), want (true, true)", isSuspend, ok)
		}
		if isSuspend, ok := l.LookupCallTargetSuspend("a.kt", 2, 1); !ok || isSuspend {
			t.Errorf("Non-suspend at (2,1) = (%v, %v), want (false, true)", isSuspend, ok)
		}
		if _, ok := l.LookupCallTargetSuspend("a.kt", 99, 99); ok {
			t.Errorf("Suspend at unregistered position ok = true, want false")
		}
	})

	t.Run(name+"/LookupCallTargetAnnotations returns per-position annotations", func(t *testing.T) {
		l := build(t, Spec{
			Positions: []PositionFact{
				{File: "a.kt", Line: 1, Col: 1, CallTarget: "com.acme.Foo.bar", Annotations: []string{"kotlin.Deprecated"}},
			},
		})
		assertStringSetEqual(t, "LookupCallTargetAnnotations", l.LookupCallTargetAnnotations("a.kt", 1, 1), []string{"kotlin.Deprecated"})
		if got := l.LookupCallTargetAnnotations("b.kt", 1, 1); len(got) != 0 {
			t.Errorf("LookupCallTargetAnnotations in different file = %v, want empty", got)
		}
	})
}

// runDiagnosticsContract verifies LookupDiagnostics.
func runDiagnosticsContract(t *testing.T, name string, build Builder) {
	t.Helper()
	t.Run(name+"/LookupDiagnostics returns per-file diagnostics", func(t *testing.T) {
		want := oracle.Diagnostic{
			FactoryName: "UNREACHABLE_CODE",
			Severity:    "WARNING",
			Message:     "unreachable",
			Line:        5,
			Col:         1,
		}
		l := build(t, Spec{
			Diagnostics: map[string][]oracle.Diagnostic{
				"a.kt": {want},
			},
		})
		got := l.LookupDiagnostics("a.kt")
		if len(got) != 1 {
			t.Fatalf("LookupDiagnostics len = %d, want 1", len(got))
		}
		if got[0].FactoryName != want.FactoryName || got[0].Line != want.Line {
			t.Errorf("got %+v, want %+v", got[0], want)
		}
		if got := l.LookupDiagnostics("b.kt"); len(got) != 0 {
			t.Errorf("LookupDiagnostics in different file = %v, want empty", got)
		}
	})
}

// RunContract runs the shared assertions against one Lookup
// implementation. name is used to label sub-tests so failures point
// at the offending implementation.
func RunContract(t *testing.T, name string, build Builder) {
	t.Helper()
	runEmptyLookupContract(t, name, build)
	runSubtypeContract(t, name, build)
	runTypeIndexContract(t, name, build)
	runPositionContract(t, name, build)
	runDiagnosticsContract(t, name, build)
}

// assertStringSetEqual checks that got and want contain the same
// strings, ignoring order. The real Oracle indexes sealed variants
// from declarations and may interleave order with classBySimple
// fallback indexing, so set-equality is the appropriate contract.
func assertStringSetEqual(t *testing.T, what string, got, want []string) {
	t.Helper()
	gotSet := make(map[string]bool, len(got))
	for _, s := range got {
		gotSet[s] = true
	}
	if len(gotSet) != len(want) {
		t.Errorf("%s: got %v, want %v (different size)", what, got, want)
		return
	}
	for _, s := range want {
		if !gotSet[s] {
			t.Errorf("%s: missing %q (got %v)", what, s, got)
		}
	}
}
