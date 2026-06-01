package pipeline

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestTryAffectedSetDispatch_BailReasons pins the observable bail labels the
// early gates report, so the dispatchAffectedSetPath perf "reason" stays
// meaningful.
func TestTryAffectedSetDispatch_BailReasons(t *testing.T) {
	ctx := context.Background()
	args := ProjectArgs{Config: config.NewConfig()}
	host := ProjectHostState{}

	t.Setenv("KRIT_AFFECTED_SET_REPLAY", "off")
	if _, _, reason, err := tryAffectedSetDispatch(ctx, args, host, IndexResult{}, ParseResult{}, deltaManifestData{}); err != nil || reason != replayBailDisabled {
		t.Fatalf("kill switch -> reason %q err %v, want %q", reason, err, replayBailDisabled)
	}

	t.Setenv("KRIT_AFFECTED_SET_REPLAY", "on")
	if _, _, reason, _ := tryAffectedSetDispatch(ctx, args, host, IndexResult{}, ParseResult{}, deltaManifestData{}); reason != replayBailNoManifest {
		t.Errorf("manifest disabled -> %q, want %q", reason, replayBailNoManifest)
	}
	m := deltaManifestData{enabled: true, manifestKey: "k"}
	if _, _, reason, _ := tryAffectedSetDispatch(ctx, args, host, IndexResult{}, ParseResult{}, m); reason != replayBailNoIndex {
		t.Errorf("nil index -> %q, want %q", reason, replayBailNoIndex)
	}
	idx := IndexResult{CodeIndex: scanner.BuildIndexFromData(nil, nil)}
	if _, _, reason, _ := tryAffectedSetDispatch(ctx, args, host, idx, ParseResult{}, m); reason != replayBailFullBuild {
		t.Errorf("full build (no removed edges) -> %q, want %q", reason, replayBailFullBuild)
	}
}

// TestMergeParsedFiles appends without mutating the original and de-dups
// nothing (callers guarantee disjoint inputs).
func TestMergeParsedFiles(t *testing.T) {
	orig := ParseResult{
		KotlinFiles: []*scanner.File{{Path: "a.kt", Language: scanner.LangKotlin}},
		JavaFiles:   []*scanner.File{{Path: "C.java", Language: scanner.LangJava}},
	}
	merged := mergeParsedFiles(orig,
		[]*scanner.File{{Path: "b.kt", Language: scanner.LangKotlin}},
		[]*scanner.File{{Path: "D.java", Language: scanner.LangJava}},
	)
	if got := pathsOf(merged.KotlinFiles); len(got) != 2 {
		t.Errorf("merged kotlin = %v, want 2", got)
	}
	if got := pathsOf(merged.JavaFiles); len(got) != 2 {
		t.Errorf("merged java = %v, want 2", got)
	}
	// Original slices must be untouched.
	if len(orig.KotlinFiles) != 1 || len(orig.JavaFiles) != 1 {
		t.Errorf("mergeParsedFiles mutated the original: kt=%d java=%d",
			len(orig.KotlinFiles), len(orig.JavaFiles))
	}
}

// TestMaterializeAffectedFiles_AlreadyParsed returns the input unchanged when
// every affected file is already present.
func TestMaterializeAffectedFiles_AlreadyParsed(t *testing.T) {
	p := ParseResult{KotlinFiles: []*scanner.File{{Path: "a.kt", Language: scanner.LangKotlin}}}
	args := ProjectArgs{Config: config.NewConfig()}
	got, bail := materializeAffectedFiles(context.Background(), args, ProjectHostState{}, p, []string{"a.kt"})
	if bail != "" {
		t.Fatalf("all-present affected set must succeed; got bail %q", bail)
	}
	if len(got.KotlinFiles) != 1 {
		t.Errorf("expected the input passed through; got %v", pathsOf(got.KotlinFiles))
	}
}

// TestMaterializeAffectedFiles_ParsesMissingDependent parses an affected
// reverse-dependency source file that the warm parse skipped.
func TestMaterializeAffectedFiles_ParsesMissingDependent(t *testing.T) {
	dir := t.TempDir()
	dirty := filepath.Join(dir, "Dirty.kt")
	dep := filepath.Join(dir, "Dep.kt")
	writeKt(t, dirty, "package test\nclass Dirty\n")
	writeKt(t, dep, "package test\nclass Dep\n")

	// parseResult holds only the dirty file (warm regime).
	p := ParseResult{KotlinFiles: []*scanner.File{{Path: dirty, Language: scanner.LangKotlin}}}
	args := ProjectArgs{Config: config.NewConfig(), Paths: []string{dir}}

	got, bail := materializeAffectedFiles(context.Background(), args, ProjectHostState{}, p, []string{dirty, dep})
	if bail != "" {
		t.Fatalf("missing source dependent must be materialized, not bailed; got %q", bail)
	}
	parsed := parsedPathSet(got)
	if !parsed[dirty] || !parsed[dep] {
		t.Errorf("dispatch parse must contain both files; got %v", pathsOf(got.KotlinFiles))
	}
}

// TestMaterializeAffectedFiles_BailsOnXML bails when an affected file is not
// Kotlin/Java source (XML referrers cannot be re-dispatched this way).
func TestMaterializeAffectedFiles_BailsOnXML(t *testing.T) {
	p := ParseResult{KotlinFiles: []*scanner.File{{Path: "a.kt", Language: scanner.LangKotlin}}}
	args := ProjectArgs{Config: config.NewConfig(), Paths: []string{t.TempDir()}}
	if _, bail := materializeAffectedFiles(context.Background(), args, ProjectHostState{}, p, []string{"a.kt", "res/layout/main.xml"}); bail != replayBailNonSource {
		t.Errorf("an XML affected file must bail with %q; got %q", replayBailNonSource, bail)
	}
}

// TestMaterializeAffectedFiles_BailsOnDeletedSource bails when an affected
// source file is gone — it never comes back parsed, so the set is incomplete.
func TestMaterializeAffectedFiles_BailsOnDeletedSource(t *testing.T) {
	dir := t.TempDir()
	p := ParseResult{KotlinFiles: []*scanner.File{{Path: filepath.Join(dir, "a.kt"), Language: scanner.LangKotlin}}}
	args := ProjectArgs{Config: config.NewConfig(), Paths: []string{dir}}
	gone := filepath.Join(dir, "Gone.kt") // never created on disk
	if _, bail := materializeAffectedFiles(context.Background(), args, ProjectHostState{}, p, []string{filepath.Join(dir, "a.kt"), gone}); bail != replayBailParseIncomplete {
		t.Errorf("a deleted/unparseable affected source file must bail with %q; got %q", replayBailParseIncomplete, bail)
	}
}
