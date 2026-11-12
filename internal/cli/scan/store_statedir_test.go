package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/fsutil"
)

// With KRIT_STATE_DIR set, an existing ./.krit/store in the project must not
// receive writes; the default store lives under the state directory.
func TestResolvedStoreDefaultFollowsStateDir(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	if err := os.MkdirAll(filepath.Join(".krit", "store"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(fsutil.StateDirEnv, t.TempDir())
	empty := ""
	if st := resolvedStore(&empty); st != nil {
		t.Fatalf("resolvedStore used the project's .krit/store under KRIT_STATE_DIR")
	}
	if err := os.MkdirAll(filepath.Join(fsutil.ProjectStateDir("."), "store"), 0o755); err != nil {
		t.Fatal(err)
	}
	if st := resolvedStore(&empty); st == nil {
		t.Fatalf("resolvedStore ignored the store under KRIT_STATE_DIR")
	}
}

func TestResolvedStoreDefaultUnderProjectWithoutStateDir(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(fsutil.StateDirEnv, "")
	empty := ""
	if st := resolvedStore(&empty); st != nil {
		t.Fatalf("resolvedStore returned a store with no .krit/store")
	}
	if err := os.MkdirAll(filepath.Join(".krit", "store"), 0o755); err != nil {
		t.Fatal(err)
	}
	if st := resolvedStore(&empty); st == nil {
		t.Fatalf("resolvedStore ignored ./.krit/store")
	}
}
