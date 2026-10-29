package oracle

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

func TestTypesJSONSatisfiesNeedsCurrentRecord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "types.json")
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	scope := storeScopeForToken(BackendFIR, "jar-x")

	if TypesJSONSatisfies(path, scope, "") {
		t.Fatal("types.json without a facts record must not be trusted")
	}

	if err := RecordTypesFacts(path, scope); err != nil {
		t.Fatal(err)
	}
	rec, ok := readTypesFacts(path)
	if !ok || !TypesJSONSatisfies(path, scope, rec.BuildToken) {
		t.Fatal("types.json with a matching record was not reusable")
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
	if TypesJSONSatisfies(path, scope, rec.BuildToken) {
		t.Fatal("record for a replaced types.json must not vouch for the new file")
	}
}

func TestTypesJSONSatisfiesBuildAndBackend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "types.json")
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	scope := storeScopeForToken(BackendFIR, "jar-x")
	if err := RecordTypesFacts(path, scope); err != nil {
		t.Fatal(err)
	}
	rec, ok := readTypesFacts(path)
	if !ok || !TypesJSONSatisfies(path, scope, rec.BuildToken) {
		t.Fatal("matching facts record was not reusable")
	}
	if TypesJSONSatisfies(path, scope, "different-build") {
		t.Fatal("different build reused types.json")
	}
	if TypesJSONSatisfies(path, storeScopeForToken(BackendFIR, "jar-y"), rec.BuildToken) {
		t.Fatal("different jar reused types.json")
	}
}

// A settings-only change (same sources, jar, classpath) must not reuse a
// types.json computed under other Kotlin compiler settings or JVM target.
func TestTypesJSONSatisfiesRejectsSettingsOnlyChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "types.json")
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	v20 := NewStoreScopeWithSettings(BackendFIR, "", nil, "17", gradlemodel.KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0"})
	v22 := NewStoreScopeWithSettings(BackendFIR, "", nil, "17", gradlemodel.KotlinSettings{LanguageVersion: "2.2", APIVersion: "2.2"})
	if err := RecordTypesFacts(path, v20); err != nil {
		t.Fatal(err)
	}
	rec, _ := readTypesFacts(path)
	if !TypesJSONSatisfies(path, v20, rec.BuildToken) {
		t.Fatal("same settings must reuse")
	}
	if TypesJSONSatisfies(path, v22, rec.BuildToken) {
		t.Fatal("settings-only change reused a stale types.json")
	}
	other := NewStoreScopeWithSettings(BackendFIR, "", nil, "21", gradlemodel.KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0"})
	if TypesJSONSatisfies(path, other, rec.BuildToken) {
		t.Fatal("JVM target change reused types.json")
	}
	if TypesJSONSatisfies(path, NewStoreScopeWithSettings(BackendFIR, "", nil, "", gradlemodel.KotlinSettings{}), rec.BuildToken) {
		t.Fatal("no-settings scope matched a settings record")
	}
}
