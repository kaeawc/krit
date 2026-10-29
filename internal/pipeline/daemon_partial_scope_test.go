package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/buildid"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/oracle"
)

// The daemon's partial reanalyze merges fresh facts for the stale paths into the
// cached types.json and keeps every other file's facts. After a settings-only
// change plus one edit that would keep old-settings facts for the untouched
// files, so a merge is allowed only into a types.json stamped with this scope.
func TestPartialMergeRequiresTheRunsCompileScope(t *testing.T) {
	typesPath := filepath.Join(t.TempDir(), "types.json")
	if err := os.WriteFile(typesPath, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	scopeOf := func(lv string) oracle.StoreScope {
		return oracle.NewStoreScopeWithSettings(oracle.BackendFIR, "", nil, "", gradlemodel.KotlinSettings{LanguageVersion: lv, APIVersion: lv})
	}
	v20, v22 := scopeOf("2.0"), scopeOf("2.2")

	if partialMergeAllowed(typesPath, v20, true, nil) {
		t.Fatal("a types.json with no scope record must not be merged into")
	}
	recordTypesScope(typesPath, v20, true, nil, t.Logf)
	if !partialMergeAllowed(typesPath, v20, true, nil) {
		t.Fatal("types.json stamped with this scope must allow a partial merge")
	}
	if partialMergeAllowed(typesPath, v22, true, nil) {
		t.Fatal("settings-only change allowed a partial merge into old-settings facts")
	}
	if partialMergeAllowed(typesPath, v20, false, nil) {
		t.Fatal("an unresolved scope must not allow a merge")
	}
	// The fallback full result is stamped with the new scope, so partial resumes.
	full := &oracle.Data{Version: 1, Files: map[string]*oracle.File{}}
	if _, err := analyzeAllAndPersist(func() (*oracle.Data, error) { return full, nil }, typesPath, t.Logf, v22, true, nil); err != nil {
		t.Fatal(err)
	}
	if !partialMergeAllowed(typesPath, v22, true, nil) || partialMergeAllowed(typesPath, v20, true, nil) {
		t.Fatalf("persisted full result is not stamped with the new scope (build token %s)", buildid.Token())
	}
}
