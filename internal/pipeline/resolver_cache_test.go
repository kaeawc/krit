package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// countingResolverCache is a test fake implementing ResolverCache that
// counts how often build() actually fires versus how often the cached
// pointer is returned.
type countingResolverCache struct {
	cached typeinfer.TypeResolver
	fp     string
	builds int
}

func (c *countingResolverCache) Resolver(fingerprint string, build func() typeinfer.TypeResolver) typeinfer.TypeResolver {
	if c.cached != nil && c.fp == fingerprint {
		return c.cached
	}
	c.cached = build()
	c.fp = fingerprint
	c.builds++
	return c.cached
}

// TestIndexPhase_ResolverCache_BuildsOnceAcrossRuns confirms the
// in-memory ResolverCache slot serves the second IndexPhase.Run when
// the file set is unchanged. Acceptance for #48: warm-no-change runs
// skip the entire perFileExtraction + merge + resolveSupertypes pass.
func TestIndexPhase_ResolverCache_BuildsOnceAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "Sample.kt")
	if err := os.WriteFile(src, []byte("package test\n\nclass Sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := scanner.ParseFile(src)
	if err != nil {
		t.Fatal(err)
	}

	rule := api.FakeRule("R", api.WithNeeds(api.NeedsResolver))

	cache := &countingResolverCache{}
	in := IndexInput{
		ParseResult: ParseResult{
			Paths:       []string{dir},
			KotlinFiles: []*scanner.File{parsed},
			ActiveRules: []*api.Rule{rule},
		},
		ResolverCache: cache,
	}

	r1, err := IndexPhase{SkipModules: true, SkipAndroid: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if r1.Resolver == nil {
		t.Fatal("first Run produced nil Resolver")
	}
	if cache.builds != 1 {
		t.Fatalf("after first Run, builds = %d, want 1", cache.builds)
	}

	r2, err := IndexPhase{SkipModules: true, SkipAndroid: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if cache.builds != 1 {
		t.Fatalf("after second Run, builds = %d, want 1 (cache miss)", cache.builds)
	}
	if r1.Resolver != r2.Resolver {
		t.Errorf("Resolver pointer changed across runs; cache is not reusing the cached value")
	}
}

// TestIndexPhase_ResolverCache_RebuildsOnContentChange confirms a
// content edit to one indexed file produces a fingerprint mismatch
// and forces a fresh resolver. This is the safety contract that
// prevents stale class/extension data from leaking across edits.
func TestIndexPhase_ResolverCache_RebuildsOnContentChange(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "Sample.kt")
	if err := os.WriteFile(src, []byte("package test\nclass A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed1, err := scanner.ParseFile(src)
	if err != nil {
		t.Fatal(err)
	}

	rule := api.FakeRule("R", api.WithNeeds(api.NeedsResolver))
	cache := &countingResolverCache{}

	mkInput := func(file *scanner.File) IndexInput {
		return IndexInput{
			ParseResult: ParseResult{
				Paths:       []string{dir},
				KotlinFiles: []*scanner.File{file},
				ActiveRules: []*api.Rule{rule},
			},
			ResolverCache: cache,
		}
	}

	if _, err := (IndexPhase{SkipModules: true, SkipAndroid: true}).Run(context.Background(), mkInput(parsed1)); err != nil {
		t.Fatal(err)
	}
	if cache.builds != 1 {
		t.Fatalf("after first Run, builds = %d, want 1", cache.builds)
	}

	// Rewrite content (different bytes → different content hash).
	if err := os.WriteFile(src, []byte("package test\nclass B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed2, err := scanner.ParseFile(src)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := (IndexPhase{SkipModules: true, SkipAndroid: true}).Run(context.Background(), mkInput(parsed2)); err != nil {
		t.Fatal(err)
	}
	if cache.builds != 2 {
		t.Fatalf("after content change, builds = %d, want 2 (rebuild on fingerprint mismatch)", cache.builds)
	}
}

func TestResolverFingerprint_StableAndDistinct(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "A.kt")
	b := filepath.Join(dir, "B.kt")
	if err := os.WriteFile(a, []byte("package x\nclass A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("package x\nclass B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pa, _ := scanner.ParseFile(a)
	pb, _ := scanner.ParseFile(b)

	fp1 := resolverFingerprint([]*scanner.File{pa, pb})
	fp2 := resolverFingerprint([]*scanner.File{pb, pa})
	if fp1 != fp2 {
		t.Errorf("fingerprint should be order-independent: %q vs %q", fp1, fp2)
	}
	fp3 := resolverFingerprint([]*scanner.File{pa})
	if fp1 == fp3 {
		t.Errorf("fingerprint should differ for distinct file sets: %q", fp1)
	}
}
