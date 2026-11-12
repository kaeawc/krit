package fsutil

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectStateDirDefaultsUnderProject(t *testing.T) {
	t.Setenv(StateDirEnv, "")
	repo := t.TempDir()
	if got, want := ProjectStateDir(repo), filepath.Join(repo, ".krit"); got != want {
		t.Fatalf("ProjectStateDir = %q, want %q", got, want)
	}
}

func TestProjectStateDirHonorsOverride(t *testing.T) {
	state := t.TempDir()
	t.Setenv(StateDirEnv, state)
	got := ProjectStateDir(t.TempDir())
	if filepath.Dir(filepath.Dir(got)) != state || filepath.Base(got) != ".krit" {
		t.Fatalf("ProjectStateDir = %q, want %s/<project>/.krit", got, state)
	}
}

func TestProjectStateDirResolvesRelativeOverride(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	t.Setenv(StateDirEnv, "state")
	got := ProjectStateDir("/unused")
	want, err := filepath.Abs("state")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, want+string(filepath.Separator)) {
		t.Fatalf("ProjectStateDir = %q, want it under %q", got, want)
	}
}

func TestProjectStateDirOverrideSeparatesProjects(t *testing.T) {
	t.Setenv(StateDirEnv, t.TempDir())
	a, b := t.TempDir(), t.TempDir()
	if ProjectStateDir(a) == ProjectStateDir(b) {
		t.Fatalf("projects %q and %q share state dir %q", a, b, ProjectStateDir(a))
	}
	t.Chdir(a)
	if got, want := ProjectStateDir("."), ProjectStateDir(a); got != want {
		t.Fatalf("ProjectStateDir(.) = %q, want %q for the same root", got, want)
	}
	if got, want := ProjectStateDir(filepath.Join(a, "sub", "..")), ProjectStateDir(a); got != want {
		t.Fatalf("unclean root = %q, want %q", got, want)
	}
}

func TestUserStateDirDefaultsUnderHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv(HomeEnv, "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	got, err := UserStateDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".krit"); got != want {
		t.Fatalf("UserStateDir = %q, want %q", got, want)
	}
}

func TestUserStateDirHonorsOverrideWithoutHome(t *testing.T) {
	kritHome := t.TempDir()
	t.Setenv(HomeEnv, kritHome)
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	got, err := UserStateDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != kritHome {
		t.Fatalf("UserStateDir = %q, want %q", got, kritHome)
	}
}

func TestUserStateDirOverrideWinsOverHome(t *testing.T) {
	home := t.TempDir()
	kritHome := t.TempDir()
	t.Setenv(HomeEnv, kritHome)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	got, err := UserStateDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != kritHome {
		t.Fatalf("UserStateDir = %q, want %q", got, kritHome)
	}
}
