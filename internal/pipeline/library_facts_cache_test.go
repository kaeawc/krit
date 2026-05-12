package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
)

// countingLibraryFactsCache is a test fake providing a LibraryFactsCache
// that counts how often build() actually fires versus how often the cached
// pointer is returned.
type countingLibraryFactsCache struct {
	cached *librarymodel.Facts
	fp     string
	builds int
}

func (c *countingLibraryFactsCache) Get(fingerprint string, build func() *librarymodel.Facts) *librarymodel.Facts {
	if c.cached != nil && c.fp == fingerprint {
		return c.cached
	}
	c.cached = build()
	c.fp = fingerprint
	c.builds++
	return c.cached
}

func TestIndexPhase_LibraryFactsCache_BuildsOnceAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	gradlePath := filepath.Join(dir, "build.gradle.kts")
	if err := os.WriteFile(gradlePath, []byte("plugins { id(\"com.android.application\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cache := &countingLibraryFactsCache{}
	in := IndexInput{
		ParseResult:       ParseResult{Paths: []string{dir}},
		LibraryFactsCache: cache.Get,
	}

	r1, err := IndexPhase{SkipModules: true, SkipResolverIndex: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if r1.LibraryFacts == nil {
		t.Fatal("first Run produced nil LibraryFacts")
	}
	if cache.builds != 1 {
		t.Fatalf("after first Run, builds = %d, want 1", cache.builds)
	}

	r2, err := IndexPhase{SkipModules: true, SkipResolverIndex: true}.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if cache.builds != 1 {
		t.Fatalf("after second Run, builds = %d, want 1 (cache miss)", cache.builds)
	}
	if r1.LibraryFacts != r2.LibraryFacts {
		t.Errorf("LibraryFacts pointer changed across runs; cache is not reusing the cached value")
	}
}

func TestLibraryFactsFingerprint_StableAndDistinct(t *testing.T) {
	a := libraryFactsFingerprint([]string{"/x/build.gradle", "/y/build.gradle"})
	b := libraryFactsFingerprint([]string{"/y/build.gradle", "/x/build.gradle"})
	if a != b {
		t.Errorf("fingerprint should be order-independent: %q vs %q", a, b)
	}
	c := libraryFactsFingerprint([]string{"/x/build.gradle"})
	if a == c {
		t.Errorf("fingerprint should differ for distinct path sets: %q", a)
	}
}
