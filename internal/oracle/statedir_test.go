package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/fsutil"
)

func TestCachePathHonorsStateDirOverride(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "settings.gradle.kts"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	t.Setenv(fsutil.StateDirEnv, state)
	if got, want := CachePath([]string{repo}), filepath.Join(state, "types.json"); got != want {
		t.Fatalf("CachePath = %q, want %q", got, want)
	}
}

func TestInstalledJarPathHonorsKritHome(t *testing.T) {
	kritHome := t.TempDir()
	t.Setenv(fsutil.HomeEnv, kritHome)
	got, err := installedJarPath(BackendFIR)
	if err != nil {
		t.Fatal(err)
	}
	if dir := filepath.Dir(got); dir != filepath.Join(kritHome, "jars") {
		t.Fatalf("installedJarPath dir = %q, want %q", dir, filepath.Join(kritHome, "jars"))
	}
}
