package scan

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/fsutil"
)

// Two projects scanned as "." with one shared KRIT_STATE_DIR must not replay
// each other's file lists, even when their directory stats match.
func TestFilewalkCacheSeparatesProjectsSharingStateDir(t *testing.T) {
	t.Setenv(fsutil.StateDirEnv, t.TempDir())
	stamp := time.Unix(1_700_000_000, 0)
	projects := []string{t.TempDir(), t.TempDir()}
	for i, name := range []string{"A.kt", "B.kt"} {
		writeFile(t, filepath.Join(projects[i], "src", name), "")
		for _, dir := range []string{filepath.Join(projects[i], "src"), projects[i]} {
			if err := os.Chtimes(dir, stamp, stamp); err != nil {
				t.Fatal(err)
			}
		}
	}
	idx := &fakeTrackedFileIndex{}
	for i, want := range []string{"A.kt", "B.kt"} {
		t.Chdir(projects[i])
		cacheDir := filepath.Join(fsutil.ProjectStateDir("."), filewalkCacheDirName)
		files, err := CollectFilesCachedWithIndex([]string{"."}, ktFilters, cacheDir, idx)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 1 || filepath.Base(files[0]) != want {
			t.Fatalf("project %d files = %v, want [src/%s]", i, files, want)
		}
	}
}
