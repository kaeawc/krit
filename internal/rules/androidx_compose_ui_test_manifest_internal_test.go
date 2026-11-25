package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
)

// The rule ID is the Lint issue ID, so a lint.xml entry naming it turns the rule off.
func TestTestManifestGradleConfigurationHonorsLintXMLIgnore(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "lint.xml"), []byte(`<lint>
  <issue id="TestManifestGradleConfiguration" severity="ignore"/>
</lint>`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadAndMergeDefaults("", root)
	if err != nil {
		t.Fatal(err)
	}
	if !androidLintTurnsOff(cfg, registeredRule(t, "TestManifestGradleConfiguration")) {
		t.Error("lint.xml ignore did not turn off TestManifestGradleConfiguration")
	}
}
