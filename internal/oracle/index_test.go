package oracle

import (
	"testing"
)

func newTestOracleData() *Data {
	return &Data{
		Version: 1,
		Files: map[string]*File{
			"a.kt": {
				Package: "com.example",
				Declarations: []*Class{
					{
						FQN:  "com.example.A",
						Kind: "class",
						Members: []*Member{
							{Name: "foo", Kind: "function", ReturnType: "kotlin.String", Params: []*Param{
								{Name: "x", Type: "kotlin.Int"},
							}},
						},
					},
				},
				Expressions: map[string]*ExpressionType{
					"10:5": {Type: "kotlin.String", Nullable: false, CallTarget: "com.example.B.bar"},
					"11:7": {Type: "kotlin.collections.List<kotlin.String>", Nullable: true},
					"12:1": {Type: "kotlin.Int"},
				},
			},
			"b.kt": {
				Package: "com.example",
				Declarations: []*Class{
					{
						FQN:  "com.example.B",
						Kind: "class",
						Members: []*Member{
							{Name: "bar", Kind: "function", ReturnType: "kotlin.Unit"},
						},
					},
				},
				Expressions: map[string]*ExpressionType{
					"3:3": {Type: "kotlin.String", CallTarget: "com.example.A.foo"},
					"4:3": {Type: "kotlin.String", CallTarget: "com.example.A.foo"},
				},
			},
			"empty.kt": {
				Package: "com.example",
			},
		},
		Dependencies: map[string]*Class{
			"kotlinx.coroutines.CoroutineScope": {
				FQN:     "kotlinx.coroutines.CoroutineScope",
				Kind:    "interface",
				JARPath: "/tmp/kotlinx-coroutines-core-1.7.3.jar",
			},
		},
	}
}

func TestBuildIndexFindDeclarationBySimpleName(t *testing.T) {
	idx := BuildIndex(newTestOracleData())

	hits := idx.FindDeclarationBySimpleName("A")
	if len(hits) != 1 || hits[0].FQN != "com.example.A" {
		t.Errorf("FindDeclarationBySimpleName(A) = %+v", hits)
	}

	hits = idx.FindDeclarationBySimpleName("foo")
	if len(hits) != 1 || hits[0].FQN != "com.example.A.foo" {
		t.Errorf("FindDeclarationBySimpleName(foo) = %+v", hits)
	}

	hits = idx.FindDeclarationBySimpleName("CoroutineScope")
	if len(hits) != 1 || hits[0].FQN != "kotlinx.coroutines.CoroutineScope" {
		t.Errorf("FindDeclarationBySimpleName(CoroutineScope) = %+v", hits)
	}

	if hits := idx.FindDeclarationBySimpleName("Missing"); len(hits) != 0 {
		t.Errorf("expected no hits, got %+v", hits)
	}
}

func TestBuildIndexFindDeclarationByFQN(t *testing.T) {
	idx := BuildIndex(newTestOracleData())

	d, ok := idx.FindDeclarationByFQN("com.example.A")
	if !ok {
		t.Fatal("expected declaration for com.example.A")
	}
	if d.File != "a.kt" {
		t.Errorf("File = %q, want a.kt", d.File)
	}
	if d.Kind != "class" {
		t.Errorf("Kind = %q, want class", d.Kind)
	}
	if d.Signature != "class com.example.A" {
		t.Errorf("Signature = %q", d.Signature)
	}

	m, ok := idx.FindDeclarationByFQN("com.example.A.foo")
	if !ok {
		t.Fatal("expected declaration for com.example.A.foo")
	}
	if want := "fun foo(x: kotlin.Int): kotlin.String"; m.Signature != want {
		t.Errorf("Signature = %q, want %q", m.Signature, want)
	}

	dep, ok := idx.FindDeclarationByFQN("kotlinx.coroutines.CoroutineScope")
	if !ok || dep.Kind != "interface" {
		t.Errorf("expected dependency CoroutineScope as interface, got %+v ok=%v", dep, ok)
	}
	if dep.JARPath != "/tmp/kotlinx-coroutines-core-1.7.3.jar" {
		t.Errorf("dependency JARPath = %q", dep.JARPath)
	}

	if _, ok := idx.FindDeclarationByFQN("does.not.exist"); ok {
		t.Error("unexpected hit for missing FQN")
	}
}

func TestBuildIndexFindReferencesByFQN(t *testing.T) {
	idx := BuildIndex(newTestOracleData())

	refs := idx.FindReferencesByFQN("com.example.A.foo")
	if len(refs) != 3 {
		t.Fatalf("len refs = %d, want 3: %+v", len(refs), refs)
	}
	declCount, useCount := 0, 0
	for _, r := range refs {
		if r.IsDeclaration {
			declCount++
			continue
		}
		useCount++
		if r.File != "b.kt" {
			t.Errorf("ref File = %q", r.File)
		}
	}
	if declCount != 1 || useCount != 2 {
		t.Errorf("decl=%d use=%d, want 1/2", declCount, useCount)
	}

	bbar := idx.FindReferencesByFQN("com.example.B.bar")
	for _, r := range bbar {
		if r.IsDeclaration {
			continue
		}
		if r.File != "a.kt" || r.Line != 10 || r.Column != 5 {
			t.Errorf("unexpected B.bar reference: %+v", r)
		}
	}
}

func TestBuildIndexTypeAtExpression(t *testing.T) {
	idx := BuildIndex(newTestOracleData())

	ti, ok := idx.TypeAtExpression("a.kt:12:1")
	if !ok || ti.FQN != "kotlin.Int" || ti.Nullable {
		t.Errorf("simple type lookup: %+v ok=%v", ti, ok)
	}

	ti, ok = idx.TypeAtExpression("a.kt:11:7")
	if !ok {
		t.Fatal("expected generic expr")
	}
	if ti.FQN != "kotlin.collections.List" || !ti.Nullable {
		t.Errorf("generic FQN/nullable: %+v", ti)
	}
	if len(ti.Arguments) != 1 || ti.Arguments[0].FQN != "kotlin.String" {
		t.Errorf("generic args: %+v", ti.Arguments)
	}

	if _, ok := idx.TypeAtExpression("a.kt:99:99"); ok {
		t.Error("unexpected hit for missing expr id")
	}
}

func TestBuildIndexEmptyAndNil(t *testing.T) {
	if got := BuildIndex(nil); got == nil {
		t.Fatal("BuildIndex(nil) returned nil")
	}
	idx := BuildIndex(&Data{Version: 1, Files: map[string]*File{}})
	if d, ok := idx.FindDeclarationByFQN("anything"); ok || d != nil {
		t.Errorf("empty data hit: %+v ok=%v", d, ok)
	}
	if refs := idx.FindReferencesByFQN("anything"); refs != nil {
		t.Errorf("empty refs: %+v", refs)
	}
	if _, ok := idx.TypeAtExpression("x:1:1"); ok {
		t.Error("empty exprs hit")
	}
}

func TestApplyFileUpdateReplacesDeclsAndExpressions(t *testing.T) {
	idx := BuildIndex(newTestOracleData())

	// Sanity: original a.kt declares com.example.A and references B.bar.
	if _, ok := idx.FindDeclarationByFQN("com.example.A"); !ok {
		t.Fatal("baseline missing com.example.A")
	}
	if refs := idx.FindReferencesByFQN("com.example.B.bar"); len(refs) == 0 {
		t.Fatal("baseline missing B.bar references")
	}

	// Replace a.kt: rename A -> A2, drop the call to B.bar, add a call to
	// kotlin.println at a new position.
	updated := &File{
		Package: "com.example",
		Declarations: []*Class{
			{
				FQN:  "com.example.A2",
				Kind: "class",
				Members: []*Member{
					{Name: "foo", Kind: "function", ReturnType: "kotlin.Unit"},
				},
			},
		},
		Expressions: map[string]*ExpressionType{
			"20:5": {Type: "kotlin.Unit", CallTarget: "kotlin.println"},
		},
	}
	idx.ApplyFileUpdate("a.kt", updated)

	if _, ok := idx.FindDeclarationByFQN("com.example.A"); ok {
		t.Error("old declaration com.example.A still present after update")
	}
	if _, ok := idx.FindDeclarationByFQN("com.example.A.foo"); ok {
		t.Error("old member com.example.A.foo still present after update")
	}
	d, ok := idx.FindDeclarationByFQN("com.example.A2")
	if !ok || d.File != "a.kt" {
		t.Errorf("new declaration com.example.A2 missing or wrong file: %+v ok=%v", d, ok)
	}

	// References to B.bar from a.kt should be gone; b.kt -> A.foo refs untouched.
	for _, r := range idx.FindReferencesByFQN("com.example.B.bar") {
		if r.File == "a.kt" {
			t.Errorf("stale a.kt reference to B.bar after update: %+v", r)
		}
	}
	if refs := idx.FindReferencesByFQN("com.example.A.foo"); len(refs) != 2 {
		t.Errorf("b.kt -> A.foo references lost: %d", len(refs))
	}

	// Old expression keys gone; new key present.
	if _, ok := idx.TypeAtExpression("a.kt:10:5"); ok {
		t.Error("stale expression a.kt:10:5 still present")
	}
	ti, ok := idx.TypeAtExpression("a.kt:20:5")
	if !ok || ti.FQN != "kotlin.Unit" {
		t.Errorf("new expression missing or wrong: %+v ok=%v", ti, ok)
	}

	// New call target picked up as a reference from a.kt.
	pRefs := idx.FindReferencesByFQN("kotlin.println")
	if len(pRefs) != 1 || pRefs[0].File != "a.kt" || pRefs[0].Line != 20 {
		t.Errorf("kotlin.println refs: %+v", pRefs)
	}
}

func TestRemoveFileEvictsAllEntries(t *testing.T) {
	idx := BuildIndex(newTestOracleData())
	idx.RemoveFile("b.kt")

	if _, ok := idx.FindDeclarationByFQN("com.example.B"); ok {
		t.Error("com.example.B still present after RemoveFile")
	}
	if _, ok := idx.FindDeclarationByFQN("com.example.B.bar"); ok {
		t.Error("com.example.B.bar still present after RemoveFile")
	}
	for _, r := range idx.FindReferencesByFQN("com.example.A.foo") {
		if r.File == "b.kt" {
			t.Errorf("stale b.kt reference: %+v", r)
		}
	}
	// a.kt entries untouched.
	if _, ok := idx.FindDeclarationByFQN("com.example.A"); !ok {
		t.Error("com.example.A removed unexpectedly")
	}
}

func TestApplyFileUpdateNilDropsFile(t *testing.T) {
	idx := BuildIndex(newTestOracleData())
	idx.ApplyFileUpdate("a.kt", nil)
	if _, ok := idx.FindDeclarationByFQN("com.example.A"); ok {
		t.Error("nil update should drop file entries")
	}
	if _, ok := idx.TypeAtExpression("a.kt:10:5"); ok {
		t.Error("nil update should drop expressions")
	}
}

func TestRemoveFileEvictsDeclWithNoExternalRefs(t *testing.T) {
	idx := BuildIndex(&Data{
		Version: 1,
		Files: map[string]*File{
			"only.kt": {
				Declarations: []*Class{
					{FQN: "com.example.Solo", Kind: "object"},
				},
			},
		},
	})
	if refs := idx.FindReferencesByFQN("com.example.Solo"); len(refs) != 1 {
		t.Fatalf("baseline self-ref missing: %+v", refs)
	}
	idx.RemoveFile("only.kt")
	if _, ok := idx.FindDeclarationByFQN("com.example.Solo"); ok {
		t.Error("decl still present")
	}
	if refs := idx.FindReferencesByFQN("com.example.Solo"); len(refs) != 0 {
		t.Errorf("self-ref not evicted: %+v", refs)
	}
}

func TestApplyFileUpdateInsertsNewFile(t *testing.T) {
	idx := BuildIndex(nil)
	idx.ApplyFileUpdate("c.kt", &File{
		Declarations: []*Class{
			{FQN: "com.example.C", Kind: "object"},
		},
		Expressions: map[string]*ExpressionType{
			"1:1": {Type: "kotlin.Int"},
		},
	})
	if d, ok := idx.FindDeclarationByFQN("com.example.C"); !ok || d.File != "c.kt" {
		t.Errorf("inserted decl: %+v ok=%v", d, ok)
	}
	if ti, ok := idx.TypeAtExpression("c.kt:1:1"); !ok || ti.FQN != "kotlin.Int" {
		t.Errorf("inserted expr: %+v ok=%v", ti, ok)
	}
}

func TestParseTypeInfoNested(t *testing.T) {
	ti := parseTypeInfo("kotlin.collections.Map<kotlin.String, kotlin.collections.List<kotlin.Int>>", false)
	if ti.FQN != "kotlin.collections.Map" {
		t.Fatalf("outer FQN: %q", ti.FQN)
	}
	if len(ti.Arguments) != 2 {
		t.Fatalf("args: %+v", ti.Arguments)
	}
	if ti.Arguments[0].FQN != "kotlin.String" {
		t.Errorf("arg0: %+v", ti.Arguments[0])
	}
	if ti.Arguments[1].FQN != "kotlin.collections.List" || len(ti.Arguments[1].Arguments) != 1 {
		t.Errorf("arg1: %+v", ti.Arguments[1])
	}
	if ti.Arguments[1].Arguments[0].FQN != "kotlin.Int" {
		t.Errorf("nested arg: %+v", ti.Arguments[1].Arguments[0])
	}
}
