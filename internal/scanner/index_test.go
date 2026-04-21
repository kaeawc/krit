package scanner

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bits-and-blooms/bloom/v3"
)

func buildTestIndex(symbols []Symbol, refs []Reference) *CodeIndex {
	return buildCodeIndex(symbols, refs)
}

func TestBuildIndexFromData(t *testing.T) {
	symbols := []Symbol{
		{Name: "helperFunc", Kind: "function", Visibility: "public", File: "a.kt", Line: 10, StartByte: 3, EndByte: 18},
		{Name: "HelperClass", Kind: "class", Visibility: "internal", File: "b.kt", Line: 2, StartByte: 0, EndByte: 12},
	}
	refs := []Reference{
		{Name: "helperFunc", File: "a.kt", Line: 10, InComment: false},
		{Name: "helperFunc", File: "b.kt", Line: 4, InComment: false},
		{Name: "HelperClass", File: "c.kt", Line: 7, InComment: true},
	}

	idx := BuildIndexFromData(symbols, refs)
	if idx == nil {
		t.Fatal("expected non-nil index")
	}
	if len(idx.Symbols) != len(symbols) {
		t.Fatalf("expected %d symbols, got %d", len(symbols), len(idx.Symbols))
	}
	if len(idx.References) != len(refs) {
		t.Fatalf("expected %d references, got %d", len(refs), len(idx.References))
	}
	if got := idx.ReferenceCount("helperFunc"); got != 2 {
		t.Fatalf("ReferenceCount(helperFunc) = %d, want 2", got)
	}
	if !idx.refBloom.TestString("HelperClass") {
		t.Fatal("expected HelperClass to be present in ref bloom")
	}
	if got := idx.CountNonCommentRefsInFile("helperFunc", "b.kt"); got != 1 {
		t.Fatalf("CountNonCommentRefsInFile(helperFunc, b.kt) = %d, want 1", got)
	}
	if !idx.MayHaveReference("helperFunc") {
		t.Fatal("expected helperFunc to be present in ref bloom")
	}
	if idx.MayHaveReference("missingSymbol") {
		t.Fatal("did not expect missingSymbol to be present in ref bloom")
	}
}

func TestBuildCodeIndexWithBloom_PreGroupedMatchesLazy(t *testing.T) {
	symbols := []Symbol{
		{Name: "helperFunc", Kind: "function", Visibility: "public", File: "a.kt", Line: 10},
		{Name: "HelperClass", Kind: "class", Visibility: "internal", File: "b.kt", Line: 2},
	}
	refs := []Reference{
		{Name: "helperFunc", File: "b.kt", Line: 4, InComment: false},
		{Name: "HelperClass", File: "c.kt", Line: 7, InComment: true},
		{Name: "helperFunc", File: "a.kt", Line: 10, InComment: false},
		{Name: "helperFunc", File: "b.kt", Line: 8, InComment: true},
		{Name: "helperFunc", File: "b.kt", Line: 9, InComment: false},
		{Name: "HelperClass", File: "d.kt", Line: 12, InComment: false},
		{Name: "onlyCommented", File: "notes.kt", Line: 1, InComment: true},
		{Name: "onlyCommented", File: "notes.kt", Line: 2, InComment: true},
	}
	originalRefs := append([]Reference(nil), refs...)

	got := buildCodeIndexWithBloom(symbols, refs, nil)
	want := buildCodeIndexWithBloomLazyForTest(symbols, refs, nil)

	if !reflect.DeepEqual(refs, originalRefs) {
		t.Fatal("buildCodeIndexWithBloom mutated reference order")
	}
	if !reflect.DeepEqual(got.symbolsByName, want.symbolsByName) {
		t.Fatalf("symbolsByName mismatch:\ngot  %#v\nwant %#v", got.symbolsByName, want.symbolsByName)
	}
	if !reflect.DeepEqual(got.refCountByName, want.refCountByName) {
		t.Fatalf("refCountByName mismatch:\ngot  %#v\nwant %#v", got.refCountByName, want.refCountByName)
	}
	if !reflect.DeepEqual(got.refFilesByName, want.refFilesByName) {
		t.Fatalf("refFilesByName mismatch:\ngot  %#v\nwant %#v", got.refFilesByName, want.refFilesByName)
	}
	if !reflect.DeepEqual(got.nonCommentRefFilesByName, want.nonCommentRefFilesByName) {
		t.Fatalf("nonCommentRefFilesByName mismatch:\ngot  %#v\nwant %#v", got.nonCommentRefFilesByName, want.nonCommentRefFilesByName)
	}
	if !reflect.DeepEqual(got.nonCommentRefCountByNameFile, want.nonCommentRefCountByNameFile) {
		t.Fatalf("nonCommentRefCountByNameFile mismatch:\ngot  %#v\nwant %#v", got.nonCommentRefCountByNameFile, want.nonCommentRefCountByNameFile)
	}

	gotBloom, err := got.refBloom.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal got bloom: %v", err)
	}
	wantBloom, err := want.refBloom.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal want bloom: %v", err)
	}
	if !bytes.Equal(gotBloom, wantBloom) {
		t.Fatal("ref bloom bytes differ from lazy builder")
	}
}

func buildCodeIndexWithBloomLazyForTest(symbols []Symbol, refs []Reference, prebuilt *bloom.BloomFilter) *CodeIndex {
	idx := &CodeIndex{
		Symbols:                      symbols,
		References:                   refs,
		symbolsByName:                make(map[string][]Symbol),
		refCountByName:               make(map[string]int),
		refFilesByName:               make(map[string]map[string]bool),
		nonCommentRefFilesByName:     make(map[string]map[string]bool),
		nonCommentRefCountByNameFile: make(map[string]map[string]int),
	}

	if prebuilt != nil {
		idx.refBloom = prebuilt
	} else {
		estimatedRefs := uint(len(idx.References))
		if estimatedRefs < 1000 {
			estimatedRefs = 1000
		}
		idx.refBloom = bloom.NewWithEstimates(estimatedRefs, 0.01)
	}

	for _, sym := range idx.Symbols {
		idx.symbolsByName[sym.Name] = append(idx.symbolsByName[sym.Name], sym)
	}
	for _, ref := range idx.References {
		idx.refCountByName[ref.Name]++
		if idx.refFilesByName[ref.Name] == nil {
			idx.refFilesByName[ref.Name] = make(map[string]bool)
		}
		idx.refFilesByName[ref.Name][ref.File] = true
		if !ref.InComment {
			if idx.nonCommentRefFilesByName[ref.Name] == nil {
				idx.nonCommentRefFilesByName[ref.Name] = make(map[string]bool)
			}
			idx.nonCommentRefFilesByName[ref.Name][ref.File] = true
			if idx.nonCommentRefCountByNameFile[ref.Name] == nil {
				idx.nonCommentRefCountByNameFile[ref.Name] = make(map[string]int)
			}
			idx.nonCommentRefCountByNameFile[ref.Name][ref.File]++
		}
		if prebuilt == nil {
			idx.refBloom.AddString(ref.Name)
		}
	}

	return idx
}

func TestUnusedSymbols_IgnoreCommentReferences_True(t *testing.T) {
	idx := buildTestIndex(
		[]Symbol{{Name: "helperFunc", Kind: "function", Visibility: "public", File: "a.kt", Line: 10, StartByte: 0, EndByte: 50}},
		[]Reference{
			{Name: "helperFunc", File: "a.kt", Line: 10, InComment: false},
			{Name: "helperFunc", File: "b.kt", Line: 5, InComment: true},
		},
	)
	unused := idx.UnusedSymbols(true)
	if len(unused) != 1 {
		t.Errorf("expected 1 unused with ignoreComments=true, got %d", len(unused))
	}
}

func TestUnusedSymbols_IgnoreCommentReferences_False(t *testing.T) {
	idx := buildTestIndex(
		[]Symbol{{Name: "helperFunc", Kind: "function", Visibility: "public", File: "a.kt", Line: 10}},
		[]Reference{
			{Name: "helperFunc", File: "a.kt", Line: 10, InComment: false},
			{Name: "helperFunc", File: "b.kt", Line: 5, InComment: true},
		},
	)
	unused := idx.UnusedSymbols(false)
	if len(unused) != 0 {
		t.Errorf("expected 0 unused with ignoreComments=false, got %d", len(unused))
	}
}

func TestUnusedSymbols_RealCodeRef_NotDead(t *testing.T) {
	idx := buildTestIndex(
		[]Symbol{{Name: "usedFunc", Kind: "function", Visibility: "public", File: "a.kt", Line: 5}},
		[]Reference{
			{Name: "usedFunc", File: "a.kt", Line: 5, InComment: false},
			{Name: "usedFunc", File: "b.kt", Line: 20, InComment: false},
		},
	)
	for _, ic := range []bool{true, false} {
		if len(idx.UnusedSymbols(ic)) != 0 {
			t.Errorf("expected 0 unused (ignoreComments=%v)", ic)
		}
	}
}

func TestUnusedSymbols_NoRefsAtAll(t *testing.T) {
	idx := buildTestIndex(
		[]Symbol{{Name: "deadFunc", Kind: "function", Visibility: "public", File: "a.kt", Line: 5}},
		[]Reference{{Name: "deadFunc", File: "a.kt", Line: 5, InComment: false}},
	)
	for _, ic := range []bool{true, false} {
		if len(idx.UnusedSymbols(ic)) != 1 {
			t.Errorf("expected 1 unused (ignoreComments=%v)", ic)
		}
	}
}

func TestUnusedSymbols_PrivateSkipped(t *testing.T) {
	idx := buildTestIndex(
		[]Symbol{{Name: "privateHelper", Kind: "function", Visibility: "private", File: "a.kt", Line: 5}},
		[]Reference{{Name: "privateHelper", File: "a.kt", Line: 5, InComment: false}},
	)
	if len(idx.UnusedSymbols(true)) != 0 {
		t.Error("expected 0 unused (private skipped)")
	}
}

func TestUnusedSymbols_OverrideSkipped(t *testing.T) {
	idx := buildTestIndex(
		[]Symbol{{Name: "toString", Kind: "function", Visibility: "public", File: "a.kt", Line: 5, IsOverride: true}},
		[]Reference{{Name: "toString", File: "a.kt", Line: 5, InComment: false}},
	)
	if len(idx.UnusedSymbols(true)) != 0 {
		t.Error("expected 0 unused (override skipped)")
	}
}

func TestUnusedSymbols_LocalUsageNotDead(t *testing.T) {
	idx := buildTestIndex(
		[]Symbol{{Name: "localHelper", Kind: "function", Visibility: "public", File: "a.kt", Line: 5}},
		[]Reference{
			{Name: "localHelper", File: "a.kt", Line: 5, InComment: false},
			{Name: "localHelper", File: "a.kt", Line: 20, InComment: false},
		},
	)
	if len(idx.UnusedSymbols(true)) != 0 {
		t.Error("expected 0 unused (used locally)")
	}
}

func TestUnusedSymbols_CommentOnlyLocalRef(t *testing.T) {
	idx := buildTestIndex(
		[]Symbol{{Name: "commentedFunc", Kind: "function", Visibility: "public", File: "a.kt", Line: 5}},
		[]Reference{
			{Name: "commentedFunc", File: "a.kt", Line: 5, InComment: false},
			{Name: "commentedFunc", File: "a.kt", Line: 15, InComment: true},
		},
	)
	if len(idx.UnusedSymbols(true)) != 1 {
		t.Error("expected 1 unused with ignoreComments=true")
	}
	if len(idx.UnusedSymbols(false)) != 0 {
		t.Error("expected 0 unused with ignoreComments=false")
	}
}

func TestBloomFilterBasics(t *testing.T) {
	idx := buildTestIndex(
		[]Symbol{{Name: "myFunc", Kind: "function", Visibility: "public", File: "a.kt", Line: 1}},
		[]Reference{
			{Name: "myFunc", File: "a.kt", Line: 1},
			{Name: "myFunc", File: "b.kt", Line: 10},
		},
	)
	// Bloom filter should confirm "myFunc" is referenced
	if !idx.refBloom.TestString("myFunc") {
		t.Error("bloom filter should contain 'myFunc'")
	}
	// Name that was never added
	if idx.refBloom.TestString("nonexistentFunction12345") {
		// This is a possible false positive, not a failure — but extremely unlikely with our params
		t.Log("bloom filter false positive on 'nonexistentFunction12345' (expected ~1% rate)")
	}
	// Cross-ref bloom
	if got := idx.CountNonCommentRefsInFile("myFunc", "b.kt"); got != 1 {
		t.Errorf("CountNonCommentRefsInFile(myFunc, b.kt) = %d, want 1", got)
	}
}

// TestCollectDeclarationsFlat_PropertyNameNotFileSource is a regression test
// for the FlatFindChild sentinel-collision bug (issue #348). Before the fix,
// `FlatNodeText(FlatFindChild(propIdx, "simple_identifier"))` returned the
// entire file source when the direct simple_identifier child was missing
// (Kotlin puts property names under a variable_declaration), which bloated
// the cross-file shards to 3–4x their correct size and broke name-based
// lookups for ~49% of symbols on Signal-scale repos.
func TestCollectDeclarationsFlat_PropertyNameNotFileSource(t *testing.T) {
	src := []byte(`package demo

val x = 1
val y: String = "hi"
class Holder { val z = 2 }
`)
	parser := GetKotlinParser()
	defer PutKotlinParser(parser)
	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	file := NewParsedFile("demo.kt", src, tree)

	var symbols []Symbol
	collectDeclarationsFlat(file, &symbols)

	byName := map[string]Symbol{}
	for _, s := range symbols {
		byName[s.Name] = s
		if len(s.Name) > 64 {
			t.Errorf("symbol %q kind=%s has oversized Name (%d bytes) — sentinel-collision regression",
				truncate(s.Name, 80), s.Kind, len(s.Name))
		}
		if s.Name == string(src) {
			t.Errorf("symbol kind=%s Name == entire file source — sentinel-collision regression", s.Kind)
		}
	}
	for _, want := range []string{"x", "y", "Holder"} {
		if _, ok := byName[want]; !ok {
			t.Errorf("missing symbol %q in %+v", want, symbolNames(symbols))
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func symbolNames(syms []Symbol) []string {
	out := make([]string, 0, len(syms))
	for _, s := range syms {
		out = append(out, s.Name+":"+s.Kind)
	}
	return out
}

// writeTempKt writes a .kt file in dir and returns its path.
func writeTempKt(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
	return path
}

func TestBuildIndex(t *testing.T) {
	dir := t.TempDir()

	file1 := writeTempKt(t, dir, "file1.kt", `package test
fun sharedFunction() {}
class SharedClass {}
`)
	file2 := writeTempKt(t, dir, "file2.kt", `package test
fun caller() { sharedFunction() }
`)

	f1, err := ParseFile(file1)
	if err != nil {
		t.Fatalf("ParseFile file1: %v", err)
	}
	f2, err := ParseFile(file2)
	if err != nil {
		t.Fatalf("ParseFile file2: %v", err)
	}

	idx := BuildIndex([]*File{f1, f2}, 1)

	// Verify symbols are indexed
	if len(idx.Symbols) == 0 {
		t.Fatal("expected symbols to be indexed, got 0")
	}

	// Check that sharedFunction and SharedClass are among the symbols
	foundFunc := false
	foundClass := false
	for _, sym := range idx.Symbols {
		if sym.Name == "sharedFunction" {
			foundFunc = true
			if sym.Kind != "function" {
				t.Errorf("sharedFunction kind = %q, want function", sym.Kind)
			}
			if sym.File != file1 {
				t.Errorf("sharedFunction file = %q, want %q", sym.File, file1)
			}
		}
		if sym.Name == "SharedClass" {
			foundClass = true
			if sym.Kind != "class" {
				t.Errorf("SharedClass kind = %q, want class", sym.Kind)
			}
		}
	}
	if !foundFunc {
		t.Error("sharedFunction not found in symbols")
	}
	if !foundClass {
		t.Error("SharedClass not found in symbols")
	}

	// Verify references exist
	if len(idx.References) == 0 {
		t.Fatal("expected references to be indexed, got 0")
	}

	// sharedFunction should have references from both files
	refFiles := idx.ReferenceFiles("sharedFunction")
	if refFiles == nil {
		t.Fatal("expected reference files for sharedFunction, got nil")
	}
	if !refFiles[file1] {
		t.Errorf("expected sharedFunction referenced in file1")
	}
	if !refFiles[file2] {
		t.Errorf("expected sharedFunction referenced in file2")
	}
}

func TestReferenceCount(t *testing.T) {
	dir := t.TempDir()

	file1 := writeTempKt(t, dir, "file1.kt", `package test
fun sharedFunction() {}
`)
	file2 := writeTempKt(t, dir, "file2.kt", `package test
fun caller() {
    sharedFunction()
    sharedFunction()
}
`)

	f1, err := ParseFile(file1)
	if err != nil {
		t.Fatalf("ParseFile file1: %v", err)
	}
	f2, err := ParseFile(file2)
	if err != nil {
		t.Fatalf("ParseFile file2: %v", err)
	}

	idx := BuildIndex([]*File{f1, f2}, 1)

	count := idx.ReferenceCount("sharedFunction")
	// file1 declares it (1 ref from the declaration identifier) + file2 calls it twice
	if count < 3 {
		t.Errorf("ReferenceCount(sharedFunction) = %d, want >= 3", count)
	}

	// A symbol that doesn't exist should have 0 references
	if idx.ReferenceCount("nonExistentSymbol") != 0 {
		t.Errorf("ReferenceCount(nonExistentSymbol) = %d, want 0", idx.ReferenceCount("nonExistentSymbol"))
	}
}

func TestClassLikeFanInStats_CountsDistinctExternalFiles(t *testing.T) {
	idx := buildTestIndex(
		[]Symbol{
			{Name: "UtilityClass", Kind: "class", Visibility: "public", File: "utility.kt", Line: 3},
			{Name: "helper", Kind: "function", Visibility: "public", File: "utility.kt", Line: 8},
		},
		[]Reference{
			{Name: "UtilityClass", File: "utility.kt", Line: 3, InComment: false},
			{Name: "UtilityClass", File: "feature_a.kt", Line: 10, InComment: false},
			{Name: "UtilityClass", File: "feature_a.kt", Line: 14, InComment: false},
			{Name: "UtilityClass", File: "feature_b.kt", Line: 21, InComment: false},
			{Name: "helper", File: "feature_b.kt", Line: 30, InComment: false},
		},
	)

	stats := idx.ClassLikeFanInStats(true)
	if len(stats) != 1 {
		t.Fatalf("expected 1 class-like stat, got %d", len(stats))
	}
	if stats[0].Symbol.Name != "UtilityClass" {
		t.Fatalf("expected UtilityClass stat, got %q", stats[0].Symbol.Name)
	}
	if stats[0].FanIn != 2 {
		t.Fatalf("expected fan-in 2, got %d", stats[0].FanIn)
	}
	if got := stats[0].ReferencingFiles; len(got) != 2 || got[0] != "feature_a.kt" || got[1] != "feature_b.kt" {
		t.Fatalf("unexpected referencing files: %#v", got)
	}
}

func TestClassLikeFanInStats_IgnoreCommentsToggle(t *testing.T) {
	idx := buildTestIndex(
		[]Symbol{{Name: "Hotspot", Kind: "class", Visibility: "public", File: "hotspot.kt", Line: 1}},
		[]Reference{
			{Name: "Hotspot", File: "hotspot.kt", Line: 1, InComment: false},
			{Name: "Hotspot", File: "caller.kt", Line: 7, InComment: true},
		},
	)

	statsIgnoreComments := idx.ClassLikeFanInStats(true)
	if got := statsIgnoreComments[0].FanIn; got != 0 {
		t.Fatalf("expected fan-in 0 when ignoring comments, got %d", got)
	}

	statsWithComments := idx.ClassLikeFanInStats(false)
	if got := statsWithComments[0].FanIn; got != 1 {
		t.Fatalf("expected fan-in 1 when counting comments, got %d", got)
	}
}

func TestReferenceFiles(t *testing.T) {
	dir := t.TempDir()

	file1 := writeTempKt(t, dir, "file1.kt", `package test
fun sharedFunction() {}
class OnlyInFile1 {}
`)
	file2 := writeTempKt(t, dir, "file2.kt", `package test
fun caller() { sharedFunction() }
`)
	file3 := writeTempKt(t, dir, "file3.kt", `package test
fun anotherCaller() { sharedFunction() }
`)

	f1, err := ParseFile(file1)
	if err != nil {
		t.Fatalf("ParseFile file1: %v", err)
	}
	f2, err := ParseFile(file2)
	if err != nil {
		t.Fatalf("ParseFile file2: %v", err)
	}
	f3, err := ParseFile(file3)
	if err != nil {
		t.Fatalf("ParseFile file3: %v", err)
	}

	idx := BuildIndex([]*File{f1, f2, f3}, 2)

	// sharedFunction should be referenced in all 3 files
	files := idx.ReferenceFiles("sharedFunction")
	if len(files) != 3 {
		t.Errorf("ReferenceFiles(sharedFunction) has %d files, want 3", len(files))
	}
	for _, fp := range []string{file1, file2, file3} {
		if !files[fp] {
			t.Errorf("expected sharedFunction referenced in %s", fp)
		}
	}

	// OnlyInFile1 should only appear in file1
	onlyFiles := idx.ReferenceFiles("OnlyInFile1")
	if len(onlyFiles) != 1 {
		t.Errorf("ReferenceFiles(OnlyInFile1) has %d files, want 1", len(onlyFiles))
	}
	if !onlyFiles[file1] {
		t.Errorf("expected OnlyInFile1 referenced in file1")
	}

	// Non-existent symbol
	if idx.ReferenceFiles("doesNotExist") != nil {
		t.Error("expected nil for non-existent symbol")
	}
}

func TestIsReferencedOutsideFile(t *testing.T) {
	dir := t.TempDir()

	file1 := writeTempKt(t, dir, "file1.kt", `package test
fun sharedFunction() {}
fun localOnly() {}
`)
	file2 := writeTempKt(t, dir, "file2.kt", `package test
fun caller() { sharedFunction() }
`)

	f1, err := ParseFile(file1)
	if err != nil {
		t.Fatalf("ParseFile file1: %v", err)
	}
	f2, err := ParseFile(file2)
	if err != nil {
		t.Fatalf("ParseFile file2: %v", err)
	}

	idx := BuildIndex([]*File{f1, f2}, 1)

	// sharedFunction is declared in file1 and referenced in file2
	if !idx.IsReferencedOutsideFile("sharedFunction", file1) {
		t.Error("expected sharedFunction to be referenced outside file1")
	}

	// localOnly is only in file1, not referenced in file2
	if idx.IsReferencedOutsideFile("localOnly", file1) {
		t.Error("expected localOnly NOT to be referenced outside file1")
	}

	// Non-existent symbol
	if idx.IsReferencedOutsideFile("totallyFake", file1) {
		t.Error("expected totallyFake NOT to be referenced outside file1")
	}
}

func TestBloomStats(t *testing.T) {
	dir := t.TempDir()

	file1 := writeTempKt(t, dir, "file1.kt", `package test
fun sharedFunction() {}
class SharedClass {}
`)
	file2 := writeTempKt(t, dir, "file2.kt", `package test
fun caller() { sharedFunction() }
`)

	f1, err := ParseFile(file1)
	if err != nil {
		t.Fatalf("ParseFile file1: %v", err)
	}
	f2, err := ParseFile(file2)
	if err != nil {
		t.Fatalf("ParseFile file2: %v", err)
	}

	idx := BuildIndex([]*File{f1, f2}, 1)

	refBits, crossBits := idx.BloomStats()
	if refBits == 0 {
		t.Error("expected refBits > 0")
	}
	if crossBits != 0 {
		t.Errorf("expected crossBits = 0 after removing cross-ref bloom, got %d", crossBits)
	}

	// Verify bloom filters actually work for indexed symbols
	if !idx.refBloom.TestString("sharedFunction") {
		t.Error("bloom filter should contain 'sharedFunction'")
	}
	if !idx.refBloom.TestString("SharedClass") {
		t.Error("bloom filter should contain 'SharedClass'")
	}
}

func TestIsReferencedOutsideFileExcludingComments(t *testing.T) {
	dir := t.TempDir()
	f1 := writeTempKt(t, dir, "decl.kt", `package test
fun targetFunc() {}
`)
	f2 := writeTempKt(t, dir, "ref.kt", `package test
// targetFunc is documented here
fun caller() { targetFunc() }
`)
	file1, _ := ParseFile(f1)
	file2, _ := ParseFile(f2)
	idx := BuildIndex([]*File{file1, file2}, 1)

	// Should be referenced outside file (in actual code, not just comments)
	if !idx.IsReferencedOutsideFileExcludingComments("targetFunc", f1) {
		t.Error("expected targetFunc to be referenced outside decl.kt excluding comments")
	}
}

func TestCountNonCommentRefsInFile(t *testing.T) {
	dir := t.TempDir()
	f1 := writeTempKt(t, dir, "a.kt", `package test
fun myFunc() {}
fun caller() { myFunc() }
`)
	file1, _ := ParseFile(f1)
	idx := BuildIndex([]*File{file1}, 1)

	count := idx.CountNonCommentRefsInFile("myFunc", f1)
	if count < 1 {
		t.Errorf("expected at least 1 non-comment ref, got %d", count)
	}
}

func TestCountNonCommentRefsInFile_IgnoresCommentRefs(t *testing.T) {
	idx := buildTestIndex(
		nil,
		[]Reference{
			{Name: "myFunc", File: "a.kt", Line: 1, InComment: false},
			{Name: "myFunc", File: "a.kt", Line: 2, InComment: true},
			{Name: "myFunc", File: "a.kt", Line: 3, InComment: false},
		},
	)

	if got := idx.CountNonCommentRefsInFile("myFunc", "a.kt"); got != 2 {
		t.Fatalf("CountNonCommentRefsInFile(myFunc, a.kt) = %d, want 2", got)
	}
}

func TestIsReferencedOutsideFileExcludingComments_CommentOnlyExternalRef(t *testing.T) {
	idx := buildTestIndex(
		nil,
		[]Reference{
			{Name: "targetFunc", File: "decl.kt", Line: 1, InComment: false},
			{Name: "targetFunc", File: "other.kt", Line: 2, InComment: true},
		},
	)

	if idx.IsReferencedOutsideFileExcludingComments("targetFunc", "decl.kt") {
		t.Fatal("expected comment-only external refs to be ignored")
	}
}

func BenchmarkCodeIndexHotLookups(b *testing.B) {
	refs := make([]Reference, 0, 200000)
	for i := 0; i < 100000; i++ {
		refs = append(refs, Reference{Name: "hotSymbol", File: "decl.kt", Line: i + 1, InComment: false})
		refs = append(refs, Reference{Name: "hotSymbol", File: "other.kt", Line: i + 1, InComment: i%10 == 0})
	}
	idx := buildTestIndex(
		[]Symbol{{Name: "hotSymbol", Kind: "function", Visibility: "public", File: "decl.kt", Line: 1}},
		refs,
	)

	b.Run("IsReferencedOutsideFileExcludingComments", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if !idx.IsReferencedOutsideFileExcludingComments("hotSymbol", "decl.kt") {
				b.Fatal("expected external non-comment refs")
			}
		}
	})

	b.Run("CountNonCommentRefsInFile", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if got := idx.CountNonCommentRefsInFile("hotSymbol", "decl.kt"); got == 0 {
				b.Fatal("expected non-comment refs")
			}
		}
	})
}

func BenchmarkBuildCodeIndexFromRefs_SignalScale(b *testing.B) {
	refs := makeSignalScaleRefs(120000, 6700, 5000000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := BuildIndexFromData(nil, refs)
		if idx.ReferenceCount("Symbol000000") == 0 {
			b.Fatal("expected hot symbol refs")
		}
	}
}

func BenchmarkBuildCodeIndexFromRefs_LazySignalScale(b *testing.B) {
	refs := makeSignalScaleRefs(120000, 6700, 5000000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := buildCodeIndexWithBloomLazyForTest(nil, refs, nil)
		if idx.ReferenceCount("Symbol000000") == 0 {
			b.Fatal("expected hot symbol refs")
		}
	}
}

func makeSignalScaleRefs(nameCount, fileCount, refCount int) []Reference {
	names := make([]string, nameCount)
	for i := range names {
		names[i] = fmt.Sprintf("Symbol%06d", i)
	}
	files := make([]string, fileCount)
	for i := range files {
		files[i] = fmt.Sprintf("module/src/File%04d.kt", i)
	}

	rng := rand.New(rand.NewSource(355))
	zipf := rand.NewZipf(rng, 1.07, 1, uint64(nameCount-1))
	refs := make([]Reference, refCount)
	for i := range refs {
		refs[i] = Reference{
			Name:      names[zipf.Uint64()],
			File:      files[rng.Intn(fileCount)],
			Line:      (i % 200) + 1,
			InComment: rng.Intn(20) == 0,
		}
	}
	return refs
}

func TestIsXMLReferenceCandidate(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/repo/app/src/main/AndroidManifest.xml", true},
		{"/repo/app/src/main/res/layout/main.xml", true},
		{"/repo/app/src/main/res/layout-land/main.xml", true},
		{"/repo/app/src/main/res/navigation/nav_graph.xml", true},
		{"/repo/app/src/main/res/menu/main.xml", true},
		{"/repo/app/src/main/res/xml/preferences.xml", true},
		{"/repo/app/src/main/res/values/strings.xml", false},
		{"/repo/app/src/main/res/values-night/colors.xml", false},
		{"/repo/app/src/main/res/raw/data.xml", false},
		{"/repo/app/src/main/java/Foo.kt", false},
	}
	for _, tc := range cases {
		if got := isXMLReferenceCandidate(tc.path); got != tc.want {
			t.Fatalf("isXMLReferenceCandidate(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
