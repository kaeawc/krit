package serve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestAnalyzeProject_InputTypesPathHonoured asserts that an explicit
// --input-types JSON path is forwarded through the verb wire and lands
// in the pipeline. The daemon does not have a krit-types.jar in its
// search path (TempDir root), so without InputTypesPath it would
// silently leave OracleEnabled=false. With InputTypesPath set to a
// readable oracle JSON dump, the verb should accept the request and
// return findings without erroring on the missing JVM.
//
// The test does NOT assert that any particular oracle-backed finding
// fires — that would require pinning a specific rule's KAA dependence.
// It asserts the wire contract: the path is accepted and the call
// completes successfully.
func TestAnalyzeProject_InputTypesPathHonoured(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Foo.kt", "package demo\n\nclass Foo\n")

	// Minimal valid oracle JSON dump: valid envelope, empty file map.
	// IndexPhase.runAutoDetectOracle reads this via NewLazyLookup +
	// Preload; an empty Files map is a no-op merge into the resolver.
	oraclePath := filepath.Join(state.root, "types.json")
	if err := os.WriteFile(oraclePath, []byte(`{"version":1,"kotlinVersion":"","files":{},"dependencies":{}}`), 0o644); err != nil {
		t.Fatalf("write oracle: %v", err)
	}

	var got daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{
			InputTypesPath: oraclePath,
		}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if len(got.Findings) == 0 {
		t.Fatal("expected findings JSON envelope")
	}
}

// TestAnalyzeProject_InputTypesPathInvalidStillReturnsFindings pins
// the documented graceful-degrade behaviour: a malformed oracle JSON
// path warns through the lazy lookup error callback but does not fail
// the verb. Rules that don't need the oracle still produce findings.
func TestAnalyzeProject_InputTypesPathInvalidStillReturnsFindings(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Foo.kt", "package demo\n\nclass Foo\n")

	bogusPath := filepath.Join(state.root, "missing-types.json")

	var got daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{
			InputTypesPath: bogusPath,
		}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if len(got.Findings) == 0 {
		t.Fatal("expected non-empty Findings JSON envelope")
	}
}
