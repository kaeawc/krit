package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registeredRule(t *testing.T, id string) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("rule %s not registered", id)
	return nil
}

// A project that ignores an Android Lint issue (element-x-android and iosched
// ignore MissingTranslation) does not get krit's port of it, by ID or by lint
// alias, unless krit.yml sets the rule's active.
func TestAndroidLintTurnsOffIgnoredIssues(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "lint.xml"), []byte(`<lint>
  <issue id="MissingTranslation" severity="ignore"/>
  <issue id="ExportedService" severity="ignore"/>
  <issue id="MagicNumber" severity="ignore"/>
</lint>`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadAndMergeDefaults("", root)
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]bool{
		"MissingTranslation":      true,
		"ServiceExportedManifest": true, // lint alias ExportedService
		"ExtraTranslation":        false,
		"MagicNumber":             false, // not an Android Lint issue
	} {
		if got := androidLintTurnsOff(cfg, registeredRule(t, id)); got != want {
			t.Errorf("%s: turned off = %v, want %v", id, got, want)
		}
	}

	if err := os.WriteFile(filepath.Join(root, "krit.yml"), []byte("android-lint:\n  MissingTranslation:\n    active: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.LoadAndMergeDefaults("", root)
	if err != nil {
		t.Fatal(err)
	}
	if androidLintTurnsOff(cfg, registeredRule(t, "MissingTranslation")) {
		t.Error("krit.yml active: true did not override the lint.xml ignore")
	}
}
