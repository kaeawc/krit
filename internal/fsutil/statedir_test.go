package fsutil

import (
	"path/filepath"
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
	if got := ProjectStateDir(t.TempDir()); got != state {
		t.Fatalf("ProjectStateDir = %q, want %q", got, state)
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
	if got != want {
		t.Fatalf("ProjectStateDir = %q, want %q", got, want)
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
