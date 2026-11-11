package oracle

import (
	"os"
	"path/filepath"
	"strings"
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
	if got, want := CachePath([]string{repo}), filepath.Join(fsutil.ProjectStateDir(repo), "types.json"); got != want || !strings.HasPrefix(got, state) {
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

func TestCachePathSeparatesProjectsSharingStateDir(t *testing.T) {
	t.Setenv(fsutil.StateDirEnv, t.TempDir())
	a, b := t.TempDir(), t.TempDir()
	for _, repo := range []string{a, b} {
		if err := os.WriteFile(filepath.Join(repo, "settings.gradle.kts"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if CachePath([]string{a}) == CachePath([]string{b}) {
		t.Fatalf("projects share types.json at %q", CachePath([]string{a}))
	}
}
