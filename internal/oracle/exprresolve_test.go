package oracle

import (
	"testing"

	"github.com/kaeawc/krit/internal/typeinfer"
)

// TestSetExpressionFact_WritesAtPackedKey verifies the sink writes
// into the same packed-line:col key shape that LookupExpression reads.
func TestSetExpressionFact_WritesAtPackedKey(t *testing.T) {
	o := &Oracle{
		expressions: make(map[string]map[uint64]*typeinfer.ResolvedType),
	}
	rt := &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypeClass}
	o.SetExpressionFact("/a.kt", 7, 13, rt)

	got := o.LookupExpression("/a.kt", 7, 13)
	if got == nil {
		t.Fatal("LookupExpression returned nil after SetExpressionFact")
	}
	if got.Name != "String" || got.FQN != "kotlin.String" {
		t.Errorf("unexpected fact: %+v", got)
	}
}

func TestSetExpressionFact_SkipsNilType(t *testing.T) {
	o := &Oracle{
		expressions: make(map[string]map[uint64]*typeinfer.ResolvedType),
	}
	o.SetExpressionFact("/a.kt", 1, 1, nil)
	if got := o.LookupExpression("/a.kt", 1, 1); got != nil {
		t.Errorf("nil fact should not have been written; got %+v", got)
	}
}

func TestSetExpressionFact_NilOracleNoOp(t *testing.T) {
	// Should not panic.
	var o *Oracle
	o.SetExpressionFact("/a.kt", 1, 1, &typeinfer.ResolvedType{Name: "Int"})
}

func TestSetExpressionFact_AppendsToExistingFile(t *testing.T) {
	o := &Oracle{
		expressions: map[string]map[uint64]*typeinfer.ResolvedType{
			"/a.kt": {packLineCol(1, 1): {Name: "Pre", Kind: typeinfer.TypeClass}},
		},
	}
	o.SetExpressionFact("/a.kt", 5, 9, &typeinfer.ResolvedType{Name: "New", Kind: typeinfer.TypeClass})

	if len(o.expressions["/a.kt"]) != 2 {
		t.Errorf("expected two facts at /a.kt; got %d", len(o.expressions["/a.kt"]))
	}
	if got := o.LookupExpression("/a.kt", 1, 1); got == nil || got.Name != "Pre" {
		t.Errorf("pre-existing fact lost; got %+v", got)
	}
	if got := o.LookupExpression("/a.kt", 5, 9); got == nil || got.Name != "New" {
		t.Errorf("new fact missing; got %+v", got)
	}
}

func TestParseExpressionPositionKey(t *testing.T) {
	cases := []struct {
		in    string
		want  ExpressionPosition
		valid bool
	}{
		{"7:13", ExpressionPosition{7, 13}, true},
		{"1:1", ExpressionPosition{1, 1}, true},
		{"100:200", ExpressionPosition{100, 200}, true},
		{"abc", ExpressionPosition{}, false},
		{"7", ExpressionPosition{}, false},
		{":13", ExpressionPosition{}, false},
		{"", ExpressionPosition{}, false},
	}
	for _, tc := range cases {
		got, ok := parseExpressionPositionKey(tc.in)
		if ok != tc.valid {
			t.Errorf("parseExpressionPositionKey(%q) ok = %v, want %v", tc.in, ok, tc.valid)
		}
		if ok && got != tc.want {
			t.Errorf("parseExpressionPositionKey(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestFactToResolvedType_PrimitiveDeducesKind(t *testing.T) {
	for _, name := range []string{"Int", "Long", "Boolean"} {
		got := factToResolvedType(resolvedExpressionFact{Name: name, FQN: "kotlin." + name, Nullable: false})
		if got.Kind != typeinfer.TypePrimitive {
			t.Errorf("%s: expected TypePrimitive; got %v", name, got.Kind)
		}
	}
}

func TestFactToResolvedType_NullableTrumpsKind(t *testing.T) {
	got := factToResolvedType(resolvedExpressionFact{Name: "Int", FQN: "kotlin.Int", Nullable: true})
	if got.Kind != typeinfer.TypeNullable {
		t.Errorf("nullable should set Kind=TypeNullable; got %v", got.Kind)
	}
	if !got.Nullable {
		t.Errorf("nullable flag not preserved")
	}
}

func TestFactToResolvedType_UnitAndNothing(t *testing.T) {
	if got := factToResolvedType(resolvedExpressionFact{Name: "Unit", FQN: "kotlin.Unit"}); got.Kind != typeinfer.TypeUnit {
		t.Errorf("Unit: expected TypeUnit; got %v", got.Kind)
	}
	if got := factToResolvedType(resolvedExpressionFact{Name: "Nothing", FQN: "kotlin.Nothing"}); got.Kind != typeinfer.TypeNothing {
		t.Errorf("Nothing: expected TypeNothing; got %v", got.Kind)
	}
}

func TestDaemon_ResolveExpressionTypes_SingleFact(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()
	fake.Responses["resolveExpressionTypes"] = `{"types":{"/a.kt":{"7:13":{"name":"String","fqn":"kotlin.String","nullable":false}}}}`
	d := fake.ConnectDaemon(t)
	defer d.Close()

	got, err := d.ResolveExpressionTypes(map[string][]ExpressionPosition{
		"/a.kt": {{Line: 7, Col: 13}},
	})
	if err != nil {
		t.Fatalf("ResolveExpressionTypes error: %v", err)
	}
	rt := got["/a.kt"][ExpressionPosition{Line: 7, Col: 13}]
	if rt == nil {
		t.Fatal("expected fact at 7:13")
	}
	if rt.Name != "String" || rt.FQN != "kotlin.String" || rt.Nullable {
		t.Errorf("unexpected fact: %+v", rt)
	}
	// Kind classification mirrors makeResolvedType (oracle.go) — String is
	// in typeinfer.PrimitiveTypes so it lands as TypePrimitive, not TypeClass.
	// We don't pin the exact Kind here to avoid coupling to typeinfer's
	// PrimitiveTypes set, but the broader contract is covered by the
	// dedicated factToResolvedType_* tests.
}

func TestDaemon_ResolveExpressionTypes_NullableFlagShapesKind(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()
	fake.Responses["resolveExpressionTypes"] = `{"types":{"/a.kt":{"1:1":{"name":"Int","fqn":"kotlin.Int","nullable":true}}}}`
	d := fake.ConnectDaemon(t)
	defer d.Close()

	got, err := d.ResolveExpressionTypes(map[string][]ExpressionPosition{
		"/a.kt": {{Line: 1, Col: 1}},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	rt := got["/a.kt"][ExpressionPosition{Line: 1, Col: 1}]
	if rt == nil {
		t.Fatal("expected fact at 1:1")
	}
	if rt.Kind != typeinfer.TypeNullable || !rt.Nullable {
		t.Errorf("nullable wire flag should produce TypeNullable + Nullable=true; got %+v", rt)
	}
}

func TestDaemon_ResolveExpressionTypes_EmptyResponse(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()
	fake.Responses["resolveExpressionTypes"] = `{"types": {}}`
	d := fake.ConnectDaemon(t)
	defer d.Close()

	got, err := d.ResolveExpressionTypes(map[string][]ExpressionPosition{
		"/a.kt": {{Line: 1, Col: 1}},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result; got %v", got)
	}
}

func TestDaemon_ResolveExpressionTypes_DropsMalformedKeys(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()
	fake.Responses["resolveExpressionTypes"] = `{"types":{"/a.kt":{"good:wrong":{"name":"X","fqn":"p.X","nullable":false},"7:13":{"name":"OK","fqn":"p.OK","nullable":false}}}}`
	d := fake.ConnectDaemon(t)
	defer d.Close()

	got, err := d.ResolveExpressionTypes(map[string][]ExpressionPosition{
		"/a.kt": {{Line: 7, Col: 13}},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(got["/a.kt"]) != 1 {
		t.Errorf("expected exactly one valid fact (malformed key dropped); got %v", got["/a.kt"])
	}
	if rt := got["/a.kt"][ExpressionPosition{Line: 7, Col: 13}]; rt == nil || rt.Name != "OK" {
		t.Errorf("expected the well-keyed fact to survive; got %+v", rt)
	}
}

func TestDaemon_ResolveExpressionTypes_DaemonError(t *testing.T) {
	fake := NewFakeDaemon(t)
	defer fake.Close()
	// Don't register a response — fake will return an error envelope.
	d := fake.ConnectDaemon(t)
	defer d.Close()

	_, err := d.ResolveExpressionTypes(map[string][]ExpressionPosition{
		"/a.kt": {{Line: 1, Col: 1}},
	})
	if err == nil {
		t.Fatal("expected error when daemon has no canned response for resolveExpressionTypes")
	}
}
