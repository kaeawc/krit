package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// countingCodeIndexCache is a test fake providing a CodeIndexCache
// that counts how often build() actually fires versus how often the
// cached pointer is returned.
type countingCodeIndexCache struct {
	cached *scanner.CodeIndex
	fp     string
	builds int
}

func (c *countingCodeIndexCache) Get(fingerprint string, build func() *scanner.CodeIndex) *scanner.CodeIndex {
	if c.cached != nil && c.fp == fingerprint {
		return c.cached
	}
	c.cached = build()
	c.fp = fingerprint
	c.builds++
	return c.cached
}

// TestCrossFilePhase_CodeIndexCache_BuildsOnceAcrossRuns confirms the
// in-memory CodeIndexCache slot serves the second CrossFilePhase.Run
// when the file set is unchanged. Acceptance for #48: warm runs reuse
// the constructed cross-file index instead of rebuilding.
func TestCrossFilePhase_CodeIndexCache_BuildsOnceAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "Sample.kt")
	if err := os.WriteFile(src, []byte("package test\n\nclass Sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := scanner.ParseFile(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}

	// CrossFilePhase only builds CodeIndex when at least one rule
	// declares NeedsCrossFile. Construct a synthetic one.
	rule := api.FakeRule("XF", api.WithNeeds(api.NeedsCrossFile))

	cache := &countingCodeIndexCache{}
	in := DispatchResult{
		IndexResult: IndexResult{
			ParseResult: ParseResult{
				Paths:       []string{dir},
				KotlinFiles: []*scanner.File{parsed},
				ActiveRules: []*api.Rule{rule},
			},
			CodeIndexCache: cache.Get,
		},
	}

	r1, err := CrossFilePhase{}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if r1.CodeIndex == nil {
		t.Fatal("first Run produced nil CodeIndex")
	}
	if cache.builds != 1 {
		t.Fatalf("after first Run, builds = %d, want 1", cache.builds)
	}

	r2, err := CrossFilePhase{}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if cache.builds != 1 {
		t.Fatalf("after second Run, builds = %d, want 1 (cache miss)", cache.builds)
	}
	if r1.CodeIndex != r2.CodeIndex {
		t.Errorf("CodeIndex pointer changed across runs; cache is not reusing the cached value")
	}
}

func TestCodeIndexFingerprint_StableAndDistinct(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "A.kt")
	b := filepath.Join(dir, "B.kt")
	if err := os.WriteFile(a, []byte("package x\nclass A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("package x\nclass B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pa, _ := scanner.ParseFile(context.Background(), a)
	pb, _ := scanner.ParseFile(context.Background(), b)

	fp1 := codeIndexFingerprint([]*scanner.File{pa, pb}, nil)
	fp2 := codeIndexFingerprint([]*scanner.File{pb, pa}, nil)
	if fp1 != fp2 {
		t.Errorf("fingerprint should be order-independent: %q vs %q", fp1, fp2)
	}
	fp3 := codeIndexFingerprint([]*scanner.File{pa}, nil)
	if fp1 == fp3 {
		t.Errorf("fingerprint should differ for distinct file sets: %q", fp1)
	}
}
