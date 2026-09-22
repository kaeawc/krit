package oracle

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTypesJSONHasDiagnostics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "types.json")
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if TypesJSONHasDiagnostics(path) {
		t.Fatal("types.json without a facts record must not be trusted to hold diagnostics")
	}

	if err := RecordTypesFacts(path, true); err != nil {
		t.Fatal(err)
	}
	if TypesJSONHasDiagnostics(path) {
		t.Fatal("types.json recorded with diagnostics omitted reported as having them")
	}

	if err := RecordTypesFacts(path, false); err != nil {
		t.Fatal(err)
	}
	if !TypesJSONHasDiagnostics(path) {
		t.Fatal("types.json recorded with diagnostics reported as lacking them")
	}

	// Another writer (for example an older krit) replaces types.json
	// without updating the record: the record no longer describes it.
	later := time.Now().Add(2 * time.Second)
	if err := os.WriteFile(path, []byte(`{"version":1,"files":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if TypesJSONHasDiagnostics(path) {
		t.Fatal("record for a replaced types.json must not vouch for the new file")
	}
}
