package cache

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/fsutil"
)

func TestDefaultDirHonorsStateDirOverride(t *testing.T) {
	state := t.TempDir()
	t.Setenv(fsutil.StateDirEnv, state)
	repo := t.TempDir()
	if got, want := DefaultDir(repo), filepath.Join(fsutil.ProjectStateDir(repo), "cache"); got != want || !strings.HasPrefix(got, state) {
		t.Fatalf("DefaultDir = %q, want %q", got, want)
	}
}
