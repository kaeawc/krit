package pipeline

import (
	"path/filepath"
	"testing"
)

// residentManifestStore is a tiny in-memory stand-in for the daemon's resident
// manifest mirror (WorkspaceState.ResidentResourceSourceManifest /
// StoreResidentResourceSourceManifest). The pipeline-level tests only need the
// map semantics, not the FIFO eviction, so this stays minimal.
type residentManifestStore struct {
	m map[string]resourceSourceBundleManifest
}

func newResidentManifestStore() *residentManifestStore {
	return &residentManifestStore{m: map[string]resourceSourceBundleManifest{}}
}

func (s *residentManifestStore) get(key string) (resourceSourceBundleManifest, bool) {
	manifest, ok := s.m[key]
	return manifest, ok
}

func (s *residentManifestStore) put(key string, manifest resourceSourceBundleManifest) {
	s.m[key] = manifest
}

// TestPersistResourceSourceBundleManifest_DeferredWriteServedFromResident pins
// the load-bearing safety property of deferring the manifest disk write: when
// BackgroundSave holds the write (does not run it inline), the very next
// lookup must still see the manifest via the resident mirror. Were that not
// true, every warm analyze between the persist and the eventual flush would
// width-mismatch the on-disk (absent) manifest and force a full O(N) re-sweep.
func TestPersistResourceSourceBundleManifest_DeferredWriteServedFromResident(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	resident := newResidentManifestStore()
	var deferred []func() // BackgroundSave parks writes here; never run inline.

	in := AndroidInput{
		CacheDir:                            cacheDir,
		ResidentResourceSourceManifest:      resident.get,
		StoreResidentResourceSourceManifest: resident.put,
		BackgroundSave:                      func(fn func()) { deferred = append(deferred, fn) },
	}

	const key = "manifest-key"
	const bundleKey = "bundle-key"
	hashes := map[string]string{"/src/A.kt": "aaaa", "/src/B.kt": "bbbb"}

	in.persistResourceSourceBundleManifest(key, bundleKey, hashes)

	// The disk write is parked, not run: nothing on disk yet.
	if _, ok := loadResourceSourceBundleManifest(cacheDir, key); ok {
		t.Fatal("expected no on-disk manifest before the deferred write runs")
	}
	if len(deferred) != 1 {
		t.Fatalf("expected exactly one parked write, got %d", len(deferred))
	}

	// The resident mirror serves the lookup immediately, with canonical-width
	// hashes byte-identical to what a later disk read would return.
	got, ok := in.lookupResourceSourceBundleManifest(key)
	if !ok {
		t.Fatal("expected resident mirror to serve the manifest before flush")
	}
	if got.BundleKey != bundleKey {
		t.Fatalf("BundleKey = %q, want %q", got.BundleKey, bundleKey)
	}
	for path, raw := range hashes {
		if want := canonResourceSourceHash(raw); got.Hashes[path] != want {
			t.Fatalf("resident hash[%s] = %q, want canonical %q", path, got.Hashes[path], want)
		}
	}

	// Draining the parked write lands the manifest on disk, byte-identical to
	// the resident value.
	for _, fn := range deferred {
		fn()
	}
	disk, ok := loadResourceSourceBundleManifest(cacheDir, key)
	if !ok {
		t.Fatal("expected on-disk manifest after flushing the deferred write")
	}
	if disk.BundleKey != got.BundleKey {
		t.Fatalf("disk BundleKey = %q, want %q", disk.BundleKey, got.BundleKey)
	}
	for path, h := range got.Hashes {
		if disk.Hashes[path] != h {
			t.Fatalf("disk hash[%s] = %q, want %q", path, disk.Hashes[path], h)
		}
	}
}

// TestLookupResourceSourceBundleManifest_DiskHitPopulatesResident verifies the
// loader populates the resident mirror on a disk hit, so a second daemon
// session that started cold reads disk once and serves from memory thereafter.
func TestLookupResourceSourceBundleManifest_DiskHitPopulatesResident(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	const key = "manifest-key"
	const bundleKey = "bundle-key"
	hashes := map[string]string{"/src/A.kt": "aaaa"}
	if err := saveResourceSourceBundleManifest(cacheDir, key, bundleKey, hashes); err != nil {
		t.Fatalf("saveResourceSourceBundleManifest: %v", err)
	}

	resident := newResidentManifestStore()
	in := AndroidInput{
		CacheDir:                            cacheDir,
		ResidentResourceSourceManifest:      resident.get,
		StoreResidentResourceSourceManifest: resident.put,
	}

	if _, ok := resident.get(key); ok {
		t.Fatal("resident mirror should start empty")
	}
	if _, ok := in.lookupResourceSourceBundleManifest(key); !ok {
		t.Fatal("expected disk hit")
	}
	if _, ok := resident.get(key); !ok {
		t.Fatal("expected disk hit to populate the resident mirror")
	}
}

// TestPersistResourceSourceBundleManifest_NilHooksWriteInline verifies CLI mode
// (no BackgroundSave, no resident hooks): the write happens inline so the
// manifest is on disk immediately, and lookup falls through to the disk read.
func TestPersistResourceSourceBundleManifest_NilHooksWriteInline(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	in := AndroidInput{CacheDir: cacheDir} // all daemon hooks nil

	const key = "manifest-key"
	const bundleKey = "bundle-key"
	hashes := map[string]string{"/src/A.kt": "aaaa"}

	in.persistResourceSourceBundleManifest(key, bundleKey, hashes)

	// Inline write: on disk immediately.
	if _, ok := loadResourceSourceBundleManifest(cacheDir, key); !ok {
		t.Fatal("expected inline write to land on disk for CLI mode")
	}
	// Lookup falls through to disk with no resident mirror.
	got, ok := in.lookupResourceSourceBundleManifest(key)
	if !ok {
		t.Fatal("expected lookup to read the inline-written manifest from disk")
	}
	if got.BundleKey != bundleKey {
		t.Fatalf("BundleKey = %q, want %q", got.BundleKey, bundleKey)
	}
}

// TestPersistResourceSourceBundleManifest_RejectsEmptyInputs verifies the guard
// rails: empty key / bundleKey / hashes are no-ops that neither touch the
// resident mirror nor enqueue a write.
func TestPersistResourceSourceBundleManifest_RejectsEmptyInputs(t *testing.T) {
	resident := newResidentManifestStore()
	var enqueued int
	in := AndroidInput{
		CacheDir:                            t.TempDir(),
		ResidentResourceSourceManifest:      resident.get,
		StoreResidentResourceSourceManifest: resident.put,
		BackgroundSave:                      func(func()) { enqueued++ },
	}

	cases := []struct {
		name      string
		key       string
		bundleKey string
		hashes    map[string]string
	}{
		{"empty key", "", "bundle", map[string]string{"/a": "h"}},
		{"empty bundleKey", "key", "", map[string]string{"/a": "h"}},
		{"empty hashes", "key", "bundle", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in.persistResourceSourceBundleManifest(tc.key, tc.bundleKey, tc.hashes)
		})
	}
	if len(resident.m) != 0 {
		t.Fatalf("expected no resident entries, got %d", len(resident.m))
	}
	if enqueued != 0 {
		t.Fatalf("expected no enqueued writes, got %d", enqueued)
	}
}
