package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

// The facts sidecar carries both the file-key spelling and the settings scope;
// a reuse must satisfy both.
func TestTypesFactsSatisfiedOnlyWhenKeyBaseAndSettingsScopeBothMatch(t *testing.T) {
	typesPath := filepath.Join(t.TempDir(), "types.json")
	if err := os.WriteFile(typesPath, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	scopeOf := func(lv string) StoreScope {
		return NewStoreScopeWithSettings(BackendFIR, "", nil, "", gradlemodel.KotlinSettings{LanguageVersion: lv, APIVersion: lv})
	}
	abs := []string{filepath.Dir(typesPath)}
	rel := []string{"."}
	if err := RecordTypesFactsForPaths(typesPath, false, abs, scopeOf("2.0")); err != nil {
		t.Fatal(err)
	}
	rec, _ := readTypesFacts(typesPath)
	token := rec.BuildToken
	if !TypesJSONSatisfiesForPaths(typesPath, false, scopeOf("2.0"), token, abs) {
		t.Fatal("same key base and same settings must satisfy")
	}
	if TypesJSONSatisfiesForPaths(typesPath, false, scopeOf("2.2"), token, abs) {
		t.Fatal("a settings-only change must not satisfy")
	}
	if TypesJSONSatisfiesForPaths(typesPath, false, scopeOf("2.0"), token, rel) {
		t.Fatal("a different key spelling must not satisfy")
	}
}
