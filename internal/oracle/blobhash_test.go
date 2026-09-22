package oracle

import "testing"

func TestBlobHash_DeterministicAcrossInsertionOrder(t *testing.T) {
	declA := &Class{
		FQN:         "demo.A",
		Kind:        "class",
		Supertypes:  []string{"demo.Z", "demo.Base"},
		Annotations: []string{"demo.Second", "demo.First"},
		Members: []*Member{{
			Name:        "value",
			Kind:        "function",
			ReturnType:  "kotlin.String",
			Nullable:    true,
			Annotations: []string{"demo.B", "demo.A"},
		}},
	}
	declB := &Class{FQN: "demo.B", Kind: "object"}
	memberReordered := *declA.Members[0]
	memberReordered.Annotations = []string{"demo.A", "demo.B"}
	declAReordered := *declA
	declAReordered.Supertypes = []string{"demo.Base", "demo.Z"}
	declAReordered.Annotations = []string{"demo.First", "demo.Second"}
	declAReordered.Members = []*Member{&memberReordered}
	exprA := &ExpressionType{
		Type:               "kotlin.String",
		Nullable:           true,
		StartByte:          20,
		EndByte:            25,
		CallTarget:         "demo.A.value",
		CallTargetResolved: true,
		Annotations:        []string{"demo.Second", "demo.First"},
	}
	exprB := &ExpressionType{Type: "kotlin.Int", StartByte: 5, EndByte: 8}
	diagA := &Diagnostic{FactoryName: "A", Severity: "WARNING", Message: "a", Line: 4, Col: 2}
	diagB := &Diagnostic{FactoryName: "B", Severity: "ERROR", Message: "b", Line: 1, Col: 9}

	forward := &File{
		Declarations: []*Class{declA, declB},
		Expressions: map[string]*ExpressionType{
			"4:2": exprA,
			"1:9": exprB,
		},
		Diagnostics: []*Diagnostic{diagA, diagB},
	}
	reversed := &File{
		Declarations: []*Class{declB, &declAReordered},
		Expressions: map[string]*ExpressionType{
			"1:9": exprB,
			"4:2": {
				Type:               exprA.Type,
				Nullable:           exprA.Nullable,
				StartByte:          exprA.StartByte,
				EndByte:            exprA.EndByte,
				CallTarget:         exprA.CallTarget,
				CallTargetResolved: exprA.CallTargetResolved,
				Annotations:        []string{"demo.First", "demo.Second"},
			},
		},
		Diagnostics: []*Diagnostic{diagB, diagA},
	}

	if got, want := BlobHash(reversed), BlobHash(forward); got != want {
		t.Fatalf("BlobHash changed with insertion order: got %q want %q", got, want)
	}
	if got := BlobHash(nil); got == "" {
		t.Fatal("BlobHash(nil) must be a valid non-empty empty-facts hash")
	}
}

func TestBlobHash_SchemaVersionChangesHash(t *testing.T) {
	f := &File{Expressions: map[string]*ExpressionType{
		"1:1": {Type: "kotlin.String", StartByte: 0, EndByte: 3},
	}}
	current := blobHashWithSchemaVersion(f, FactSchemaVersion)
	next := blobHashWithSchemaVersion(f, FactSchemaVersion+1)
	if current == next {
		t.Fatalf("schema version bump did not change BlobHash: %q", current)
	}
}
