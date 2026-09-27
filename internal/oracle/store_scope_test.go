package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/store"
)

func TestStoreScopeRetainsOriginalJarAtWrite(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("first jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	scope := NewStoreScope(BackendFIR, jar)
	s := store.New(filepath.Join(dir, "store"))
	release := BindStoreScope(s, scope)
	defer release()
	if err := os.WriteFile(jar, []byte("second jar with different bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := &CacheEntry{ContentHash: strings.Repeat("a", 64), FilePath: filepath.Join(dir, "A.kt"), Approximation: ApproximationFIRWholeCompilation}
	if err := WriteEntryToStore(s, entry); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get(oracleStoreKeyScoped(entry.ContentHash, scope)); !ok {
		t.Fatal("entry was not written under the original jar token")
	}
	newScope := NewStoreScope(BackendFIR, jar)
	if newScope.version == scope.version {
		t.Fatal("test jar rebuild did not change the token")
	}
	if _, ok := s.Get(oracleStoreKeyScoped(entry.ContentHash, newScope)); ok {
		t.Fatal("entry was written under the rebuilt jar token")
	}
}

func TestNestedProjectStoreScopeRoundTrip(t *testing.T) {
	repo := t.TempDir()
	app := filepath.Join(repo, "app")
	jar := filepath.Join(app, ".krit", "krit-fir.jar")
	if err := os.MkdirAll(filepath.Dir(jar), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jar, []byte("app jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(app, "src", "A.kt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("class A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := ContentHash(path)
	if err != nil {
		t.Fatal(err)
	}
	s := store.New(filepath.Join(repo, "store"))
	release := BindStoreScope(s, NewStoreScope(BackendFIR, jar))
	defer release()
	entry := &CacheEntry{ContentHash: hash, FilePath: path, Approximation: ApproximationFIRWholeCompilation}
	entry.Closure.Fingerprint, err = closureFingerprint(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteEntryToStore(s, entry); err != nil {
		t.Fatal(err)
	}
	cacheDir, err := CacheDir(app)
	if err != nil {
		t.Fatal(err)
	}
	hits, misses := ClassifyFilesWithStoreScopedV3(s, cacheDir, []string{path}, "", "", ApproximationFIRWholeCompilation)
	if len(hits) != 1 || len(misses) != 0 {
		t.Fatalf("nested project cache: hits=%d misses=%d", len(hits), len(misses))
	}
}

func BenchmarkOracleStoreKey(b *testing.B) {
	scope := storeScopeForToken(BackendFIR, "sha256:precomputed")
	hash := strings.Repeat("a", 64)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = oracleStoreKeyScoped(hash, scope)
	}
}
