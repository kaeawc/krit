package cache

import (
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/fsutil"
)

func TestDefaultDirHonorsStateDirOverride(t *testing.T) {
	state := t.TempDir()
	t.Setenv(fsutil.StateDirEnv, state)
	if got, want := DefaultDir(t.TempDir()), filepath.Join(state, "cache"); got != want {
		t.Fatalf("DefaultDir = %q, want %q", got, want)
	}
}
