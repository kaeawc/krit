package pipeline

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

// These caches are written without fsync, so after an OS crash or power loss
// an entry can be empty, cut short, zero-filled, or hold unrelated bytes. Each
// must load as a miss.

func tornPipelineVariants(valid []byte) map[string][]byte {
	variants := map[string][]byte{
		"empty":       {},
		"zero-filled": make([]byte, len(valid)),
		"garbage":     bytes.Repeat([]byte("garbage!"), len(valid)/8+1),
	}
	for _, n := range []int{1, 4, len(valid) / 4, len(valid) / 2, len(valid) - 1} {
		if n > 0 && n < len(valid) {
			variants["truncated-"+strconv.Itoa(n)] = append([]byte(nil), valid[:n]...)
		}
	}
	return variants
}

func assertPipelineTornMiss(t *testing.T, path string, load func() bool) {
	t.Helper()
	valid, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read valid entry: %v", err)
	}
	if !load() {
		t.Fatal("valid entry did not load")
	}
	for name, data := range tornPipelineVariants(valid) {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if load() {
				t.Fatalf("damaged entry (%d of %d bytes) loaded as a hit", len(data), len(valid))
			}
		})
	}
}

func TestResourceSourceBundleManifest_TornEntryIsMiss(t *testing.T) {
	cacheDir := t.TempDir()
	if err := saveResourceSourceBundleManifest(cacheDir, "key-a", "bundle", map[string]string{"/r/A.kt": "abcdef0123456789"}); err != nil {
		t.Fatal(err)
	}
	assertPipelineTornMiss(t, resourceSourceBundleManifestPath(cacheDir, "key-a"), func() bool {
		_, ok := loadResourceSourceBundleManifest(cacheDir, "key-a")
		return ok
	})
}

func TestMergedResourceIndexBundle_TornEntryIsMiss(t *testing.T) {
	cacheDir := t.TempDir()
	if err := saveMergedResourceIndexBundle(cacheDir, "key-a", &android.ResourceIndex{Strings: map[string]string{"title": "Title"}}); err != nil {
		t.Fatal(err)
	}
	assertPipelineTornMiss(t, mergedResourceIndexBundlePath(cacheDir, "key-a"), func() bool {
		_, ok := loadMergedResourceIndexBundle(cacheDir, "key-a")
		return ok
	})
}

func TestResDirFingerprintCache_TornFileIsMiss(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	writeLayout(t, resDir, "a", cleanLayout)
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")
	want := newResDirFingerprintCache(cacheDir).fingerprint(resDir)
	if want == "" {
		t.Fatal("empty fingerprint")
	}
	path := newResDirFingerprintCache(cacheDir).path
	assertPipelineTornMiss(t, path, func() bool {
		c := newResDirFingerprintCache(cacheDir)
		c.load()
		_, ok := c.data.Entries[resDir]
		return ok
	})
	// A damaged file is recomputed to the same fingerprint, not trusted.
	if got := newResDirFingerprintCache(cacheDir).fingerprint(resDir); got != want {
		t.Fatalf("fingerprint after damaged cache = %q, want %q", got, want)
	}
}
