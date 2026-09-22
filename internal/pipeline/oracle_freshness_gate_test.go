package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

// TestCachedTypesJSONSatisfies pins the freshness gate's fact-scope check.
// A run whose rules consume compiler diagnostics must not reuse a types.json
// written with diagnostics omitted; reusing it silently dropped every
// projected finding until a source file changed.
func TestCachedTypesJSONSatisfies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "types.json")
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	needsDiagnostics := IndexInput{OracleDiagnostics: true}
	noDiagnostics := IndexInput{}

	if err := oracle.RecordTypesFacts(path, true); err != nil {
		t.Fatal(err)
	}
	if cachedTypesJSONSatisfies(needsDiagnostics, nil, path) {
		t.Error("diagnostics run reused a types.json written without diagnostics")
	}
	if !cachedTypesJSONSatisfies(noDiagnostics, nil, path) {
		t.Error("run without diagnostic rules should reuse a diagnostics-less types.json")
	}

	if err := oracle.RecordTypesFacts(path, false); err != nil {
		t.Fatal(err)
	}
	if !cachedTypesJSONSatisfies(needsDiagnostics, nil, path) {
		t.Error("diagnostics run should reuse a types.json that holds diagnostics")
	}
	if cachedTypesJSONSatisfies(IndexInput{NoCacheOracle: true}, nil, path) {
		t.Error("--no-cache-oracle must never reuse the cached types.json")
	}

	unrecorded := filepath.Join(t.TempDir(), "types.json")
	if err := os.WriteFile(unrecorded, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if cachedTypesJSONSatisfies(needsDiagnostics, nil, unrecorded) {
		t.Error("diagnostics run trusted a types.json with no facts record")
	}
}
