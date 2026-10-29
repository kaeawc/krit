package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

// TestCachedTypesJSONSatisfies pins the freshness gate's fact-scope check:
// a cached types.json is reused only with a facts record written for it
// under this run's jar, and never with --no-cache-oracle.
func TestCachedTypesJSONSatisfies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "types.json")
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar version X"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jar)
	scope := oracle.NewStoreScope(oracle.BackendFIR, jar)

	if cachedTypesJSONSatisfies(IndexInput{}, path, nil) {
		t.Error("reused a types.json with no facts record")
	}
	if err := oracle.RecordTypesFacts(path, scope); err != nil {
		t.Fatal(err)
	}
	if !cachedTypesJSONSatisfies(IndexInput{}, path, nil) {
		t.Error("a recorded types.json from this scope should be reused")
	}
	if cachedTypesJSONSatisfies(IndexInput{NoCacheOracle: true}, path, nil) {
		t.Error("--no-cache-oracle must never reuse the cached types.json")
	}

	if err := os.WriteFile(jar, []byte("jar version Y with different bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if cachedTypesJSONSatisfies(IndexInput{}, path, nil) {
		t.Error("reused types.json after jar changed")
	}
}
