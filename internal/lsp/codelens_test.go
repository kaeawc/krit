package lsp

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestFunctionComplexity_BaseIsOne(t *testing.T) {
	lines := []string{
		"fun greet(): String {",
		"    return \"hello\"",
		"}",
	}
	got := functionComplexity(lines, 1, len(lines)+1)
	if got != 1 {
		t.Errorf("linear function complexity = %d, want 1", got)
	}
}

func TestFunctionComplexity_BranchesIncrement(t *testing.T) {
	lines := []string{
		"fun pick(x: Int, y: Int?): Int {",
		"    if (x > 0 && y != null) {",
		"        for (i in 0..x) print(i)",
		"    } else if (x < 0 || y == null) { ",
		"        return -1",
		"    }",
		"    return y ?: 0",
		"}",
	}
	got := functionComplexity(lines, 1, len(lines)+1)
	want := 1 + 6 // base + if + && + for + else-if + || + ?:
	if got != want {
		t.Errorf("complexity = %d, want %d", got, want)
	}
}

func TestFunctionComplexity_SkipsLineComments(t *testing.T) {
	lines := []string{
		"fun f() {",
		"    // if (x) { for (y) {} }  -- commented out branches",
		"    return",
		"}",
	}
	if got := functionComplexity(lines, 1, len(lines)+1); got != 1 {
		t.Errorf("complexity with commented branches = %d, want 1", got)
	}
}

func TestFunctionComplexity_HandlesOutOfRangeArgs(t *testing.T) {
	lines := []string{"fun f() {}"}
	if got := functionComplexity(lines, 0, 999); got != 1 {
		t.Errorf("complexity with bad bounds = %d, want 1", got)
	}
	if got := functionComplexity(lines, -5, -1); got != 1 {
		t.Errorf("complexity with negative bounds = %d, want 1", got)
	}
}

func TestSymbolName_PrefersFQN(t *testing.T) {
	sym := scanner.Symbol{Name: "doThing", Owner: "Foo", FQN: "com.example.Foo.doThing"}
	if got := symbolName(sym); got != "com.example.Foo.doThing" {
		t.Errorf("symbolName = %q, want FQN", got)
	}
}

func TestSymbolName_FallsBackToOwnerDotName(t *testing.T) {
	sym := scanner.Symbol{Name: "doThing", Owner: "Foo"}
	if got := symbolName(sym); got != "Foo.doThing" {
		t.Errorf("symbolName = %q, want Foo.doThing", got)
	}
}

func TestSymbolName_BareName(t *testing.T) {
	sym := scanner.Symbol{Name: "topLevel"}
	if got := symbolName(sym); got != "topLevel" {
		t.Errorf("symbolName = %q, want topLevel", got)
	}
}

func TestFunctionsInFile_FiltersByPathAndKind(t *testing.T) {
	idx := &scanner.CodeIndex{
		Symbols: []scanner.Symbol{
			{Name: "a", Kind: "function", File: "main.kt", Line: 10},
			{Name: "b", Kind: "function", File: "other.kt", Line: 5},
			{Name: "MyClass", Kind: "class", File: "main.kt", Line: 1},
			{Name: "c", Kind: "function", File: "main.kt", Line: 3},
		},
	}
	got := functionsInFile(idx, "main.kt")
	if len(got) != 2 {
		t.Fatalf("got %d functions, want 2", len(got))
	}
	// Sorted by line ascending.
	if got[0].Name != "c" || got[1].Name != "a" {
		t.Errorf("ordering: got [%s, %s], want [c, a]", got[0].Name, got[1].Name)
	}
}

func TestFunctionsInFile_SortsByNameWhenLineTies(t *testing.T) {
	idx := &scanner.CodeIndex{
		Symbols: []scanner.Symbol{
			{Name: "zeta", Kind: "function", File: "f.kt", Line: 5},
			{Name: "alpha", Kind: "function", File: "f.kt", Line: 5},
		},
	}
	got := functionsInFile(idx, "f.kt")
	if len(got) != 2 || got[0].Name != "alpha" || got[1].Name != "zeta" {
		t.Errorf("tie-break ordering wrong: %+v", got)
	}
}

func TestBuildFunctionCodeLenses_NilInputsReturnEmpty(t *testing.T) {
	if got := buildFunctionCodeLenses("file:///x.kt", nil, nil); len(got) != 0 {
		t.Errorf("nil file/index should yield empty lenses, got %d", len(got))
	}
}

func TestBuildFunctionCodeLenses_TitleAndRange(t *testing.T) {
	file := &scanner.File{
		Path: "src/Main.kt",
		Lines: []string{
			"fun greet() {",
			"    if (x) print()",
			"}",
		},
	}
	idx := &scanner.CodeIndex{
		Symbols: []scanner.Symbol{
			{Name: "greet", Kind: "function", File: "src/Main.kt", Line: 1, FQN: "com.example.greet"},
		},
	}
	lenses := buildFunctionCodeLenses("file:///src/Main.kt", file, idx)
	if len(lenses) != 1 {
		t.Fatalf("want 1 lens, got %d", len(lenses))
	}

	got := lenses[0]
	if got.Range.Start.Line != 0 || got.Range.End.Line != 0 {
		t.Errorf("lens should anchor to line 0 (0-based), got start=%d end=%d",
			got.Range.Start.Line, got.Range.End.Line)
	}
	if got.Command == nil || got.Command.Command != "krit.showReferences" {
		t.Errorf("expected command krit.showReferences, got %+v", got.Command)
	}
	// "if" branch -> complexity 2; no consumers populated -> 0.
	wantTitle := "complexity=2 | 0 consumers"
	if got.Command.Title != wantTitle {
		t.Errorf("title = %q, want %q", got.Command.Title, wantTitle)
	}
}

func TestBuildFunctionCodeLenses_NeighborLimitsScanRange(t *testing.T) {
	// The first lens's complexity must stop at the second function's
	// start line; otherwise b's branches leak into a's score.
	file := &scanner.File{
		Path: "src/Two.kt",
		Lines: []string{
			"fun a() {",
			"    val x = 1",
			"}",
			"fun b() {",
			"    if (x) {",
			"        if (y) {}",
			"    }",
			"}",
		},
	}
	idx := &scanner.CodeIndex{
		Symbols: []scanner.Symbol{
			{Name: "a", Kind: "function", File: "src/Two.kt", Line: 1},
			{Name: "b", Kind: "function", File: "src/Two.kt", Line: 4},
		},
	}
	lenses := buildFunctionCodeLenses("file:///src/Two.kt", file, idx)
	if len(lenses) != 2 {
		t.Fatalf("want 2 lenses, got %d", len(lenses))
	}
	// Functions are sorted by line; lenses[0] is `a`, lenses[1] is `b`.
	if lenses[0].Command.Title != "complexity=1 | 0 consumers" {
		t.Errorf("a's title = %q, want complexity=1", lenses[0].Command.Title)
	}
	if lenses[1].Command.Title != "complexity=3 | 0 consumers" {
		t.Errorf("b's title = %q, want complexity=3", lenses[1].Command.Title)
	}
}
