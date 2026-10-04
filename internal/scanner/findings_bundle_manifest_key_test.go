package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

// The manifest records file paths in the scan path's spelling, so two
// spellings of one directory must not share a manifest: `krit proj`
// from the parent lists "proj/A.kt", which `krit .` from inside proj
// cannot open.
func TestFindingsBundleManifestKey_SeparatesSpellingsOfOnePath(t *testing.T) {
	repo := t.TempDir()
	proj := filepath.Join(repo, "proj")
	if err := os.Mkdir(proj, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	t.Chdir(repo)
	fromParent := FindingsBundleManifestKey(repo, []string{"proj"})
	absolute := FindingsBundleManifestKey(repo, []string{proj})
	if again := FindingsBundleManifestKey(repo, []string{"proj"}); again != fromParent {
		t.Errorf("key is not stable for one spelling: %s vs %s", fromParent, again)
	}

	t.Chdir(proj)
	fromInside := FindingsBundleManifestKey(repo, []string{"."})

	if fromParent == fromInside {
		t.Errorf(`"proj" from the parent and "." from inside share key %s`, fromParent)
	}
	if absolute == fromParent || absolute == fromInside {
		t.Errorf("the absolute spelling shares a key with a relative one: abs=%s parent=%s inside=%s",
			absolute, fromParent, fromInside)
	}
}

func TestFindingsBundleManifestKey_IgnoresScanPathOrder(t *testing.T) {
	repo := t.TempDir()
	t.Chdir(repo)
	if a, b := FindingsBundleManifestKey(repo, []string{"a", "b"}), FindingsBundleManifestKey(repo, []string{"b", "a"}); a != b {
		t.Errorf("key depends on scan path order: %s vs %s", a, b)
	}
}
