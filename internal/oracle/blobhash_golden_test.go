package oracle

import "testing"

func goldenBlobFile() *File {
	members := func(n int) []*Member {
		out := make([]*Member, 0, n)
		for i := n; i > 0; i-- {
			out = append(out, &Member{Name: "m" + string(rune('a'+i)), Kind: "fun", ReturnType: "kotlin.Unit",
				Params: []*Param{{Name: "p", Type: "kotlin.String"}, nil}, Annotations: []string{"B", "A"}, Line: i, Column: 2})
		}
		return append(out, nil)
	}
	return &File{
		Declarations: []*Class{
			{FQN: "z.Z", Kind: "class", Supertypes: []string{"b.B", "a.A"}, Members: members(5)},
			nil,
			{FQN: "a.A", Kind: "interface", Members: members(3), Line: 4},
		},
		Expressions: map[string]*ExpressionType{"k": {Type: "kotlin.Int", StartByte: 3, EndByte: 9}},
		Diagnostics: []*Diagnostic{{FactoryName: "F", Severity: "WARNING", Message: "m", Line: 1, Col: 2}},
	}
}

// The hash feeds persistent cache keys, so its bytes must not change.
func TestBlobHash_GoldenValue(t *testing.T) {
	const want = "51ae925e5dff76ecfe062838ae4c18463e430aab88060c57e5e6b38f406e8622"
	if got := BlobHash(goldenBlobFile()); got != want {
		t.Fatalf("BlobHash = %s, want %s", got, want)
	}
}

func BenchmarkBlobHash(b *testing.B) {
	f := goldenBlobFile()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		BlobHash(f)
	}
}

// Overloads share a name, so ordering must also use the rest of the member,
// and duplicate-FQN declarations must order by their full content.
func TestBlobHash_OverloadAndDuplicateOrderIndependent(t *testing.T) {
	overload := func(paramType string) *Member {
		return &Member{Name: "m", Kind: "fun", ReturnType: "kotlin.Unit", Params: []*Param{{Name: "p", Type: paramType}}}
	}
	file := func(reverse bool) *File {
		ms := []*Member{overload("kotlin.String"), overload("kotlin.Int")}
		cs := []*Class{{FQN: "a.A", Kind: "class", Line: 1}, {FQN: "a.A", Kind: "class", Line: 2, Members: []*Member{overload("kotlin.Long")}}}
		if reverse {
			ms[0], ms[1] = ms[1], ms[0]
			cs[0], cs[1] = cs[1], cs[0]
		}
		return &File{Declarations: append(cs, &Class{FQN: "b.B", Kind: "class", Members: ms})}
	}
	if a, b := BlobHash(file(false)), BlobHash(file(true)); a != b {
		t.Fatalf("hash depends on declaration order: %s != %s", a, b)
	}
}
