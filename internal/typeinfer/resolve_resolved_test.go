package typeinfer

import "testing"

func TestMakeResolvedTypeTracksRealEvidence(t *testing.T) {
	resolver := NewResolver()
	resolver.classes["LocalType"] = &ClassInfo{Name: "LocalType", FQN: "test.LocalType"}
	imports := &ImportTable{
		Explicit: map[string]string{"ImportedType": "example.ImportedType"},
		Aliases:  map[string]string{},
	}

	tests := []struct {
		name     string
		typeName string
		imports  *ImportTable
		wantFQN  string
		resolved bool
	}{
		{name: "primitive", typeName: "Int", wantFQN: "kotlin.Int", resolved: true},
		{name: "stdlib", typeName: "List", wantFQN: "kotlin.collections.List", resolved: true},
		{name: "import", typeName: "ImportedType", imports: imports, wantFQN: "example.ImportedType", resolved: true},
		{name: "declaration", typeName: "LocalType", wantFQN: "LocalType", resolved: true},
		{name: "unresolved bare name", typeName: "UnknownFoo", wantFQN: "UnknownFoo", resolved: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolver.makeResolvedType(tt.typeName, tt.imports, false)
			if got.Resolved != tt.resolved {
				t.Fatalf("Resolved = %v, want %v: %#v", got.Resolved, tt.resolved, got)
			}
			if got.FQN != tt.wantFQN {
				t.Fatalf("FQN = %q, want %q: %#v", got.FQN, tt.wantFQN, got)
			}
		})
	}

	unresolvedAnnotation := resolver.makeResolvedType("UnknownFoo", nil, false)
	if unresolvedAnnotation.Nullable {
		t.Fatalf("explicit non-null unresolved annotation became nullable: %#v", unresolvedAnnotation)
	}
	if unresolvedAnnotation.IsResolved() {
		t.Fatalf("explicit non-null unresolved annotation was marked resolved: %#v", unresolvedAnnotation)
	}
	if UnknownType().IsResolved() {
		t.Fatal("UnknownType must remain unresolved by zero value")
	}
}
