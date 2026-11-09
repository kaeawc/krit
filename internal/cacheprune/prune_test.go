package cacheprune

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

var testGroup = regexp.MustCompile(`^krit-types-([0-9a-f]{12})(?:[-.]|$)`)

func groupOf(name string) (string, bool) {
	m := testGroup.FindStringSubmatch(name)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func touch(t *testing.T, path string, age time.Duration, now time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	when := now.Add(-age)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

func TestPruneKeepsNewestProtectedAndLocked(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	day := 24 * time.Hour
	touch(t, filepath.Join(dir, "krit-types-aaaaaaaaaaaa.jsa"), 10*day, now)
	touch(t, filepath.Join(dir, "krit-types-bbbbbbbbbbbb.jsa"), 9*day, now)
	touch(t, filepath.Join(dir, "krit-types-bbbbbbbbbbbb-fir.aot"), 9*day, now)
	touch(t, filepath.Join(dir, "krit-types-cccccccccccc.jsa"), 8*day, now)
	touch(t, filepath.Join(dir, "krit-types-dddddddddddd.jsa"), 7*day, now)
	touch(t, filepath.Join(dir, "krit-types-eeeeeeeeeeee.jsa"), 6*day, now)
	touch(t, filepath.Join(dir, "krit-types-ffffffffffff.jsa"), 30*time.Minute, now)
	touch(t, filepath.Join(dir, "krit-types-111111111111.jsa"), 20*day, now)
	touch(t, filepath.Join(dir, "krit-types-222222222222.aot.lock"), time.Hour, now)
	touch(t, filepath.Join(dir, "krit-types-222222222222.jsa"), 25*day, now)
	touch(t, filepath.Join(dir, "unrelated.txt"), 90*day, now)

	removed := Prune(dir, Options{Keep: 3, MinAge: day, Protect: []string{"cccccccccccc"}, GroupOf: groupOf, Now: func() time.Time { return now }})
	if len(removed) != 5 {
		t.Fatalf("removed %v, want groups a, b (2 files), d, 1", removed)
	}
	for _, name := range []string{"krit-types-aaaaaaaaaaaa.jsa", "krit-types-bbbbbbbbbbbb.jsa", "krit-types-bbbbbbbbbbbb-fir.aot", "krit-types-111111111111.jsa", "krit-types-dddddddddddd.jsa"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s should be removed", name)
		}
	}
	for _, name := range []string{"krit-types-cccccccccccc.jsa", "krit-types-eeeeeeeeeeee.jsa", "krit-types-ffffffffffff.jsa", "krit-types-222222222222.jsa", "unrelated.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s should survive: %v", name, err)
		}
	}
}

func TestPruneIntervalSkipsSecondRun(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	day := 24 * time.Hour
	touch(t, filepath.Join(dir, "krit-types-aaaaaaaaaaaa.jsa"), 10*day, now)
	opts := Options{Keep: 0, MinAge: day, Interval: day, GroupOf: groupOf, Now: func() time.Time { return now }}
	if got := Prune(dir, opts); len(got) != 1 {
		t.Fatalf("first run removed %v", got)
	}
	touch(t, filepath.Join(dir, "krit-types-bbbbbbbbbbbb.jsa"), 10*day, now)
	if got := Prune(dir, opts); len(got) != 0 {
		t.Fatalf("second run within interval removed %v", got)
	}
}
