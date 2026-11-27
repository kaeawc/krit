package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/trackedfiles"
)

// The git file index persists its listing keyed on the stat of .git/index
// and HEAD, not their content. --no-cache, the fresh-analysis baseline, must
// ask git instead: a stale listing used to decide which files it scanned
// (#1831).
func TestNoCacheIgnoresStaleTrackedFilesCache(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	for name, content := range map[string]string{
		"A.kt": "package p\n\nfun a() {\n    val x = 42\n}\n",
		"B.kt": "package p\n\nfun b() {\n    val y = 43\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "add", "A.kt", "B.kt"},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	count := func(extra ...string) int {
		t.Helper()
		args := append([]string{"--no-fir", "--no-daemon", "--no-type-oracle", "-f", "json", "-q"}, extra...)
		stdout, stderr, code := runKrit(t, append(args, dir)...)
		if code > 1 {
			t.Fatalf("krit %v exited %d: %s", extra, code, stderr)
		}
		var report struct {
			Findings []struct {
				Rule string `json:"rule"`
			} `json:"findings"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("invalid JSON for %v: %v", extra, err)
		}
		n := 0
		for _, f := range report.Findings {
			if f.Rule == "UnusedVariable" {
				n++
			}
		}
		return n
	}

	fresh := count("--no-cache")
	if fresh != 2 {
		t.Fatalf("fresh --no-cache run: %d UnusedVariable findings, want 2", fresh)
	}
	if _, err := os.Stat(trackedfiles.CacheDir(dir)); !os.IsNotExist(err) {
		t.Errorf("cold --no-cache run created %s (stat err %v)", trackedfiles.CacheDir(dir), err)
	}
	// Seed a listing that names only A.kt under the current snapshot, as a
	// listing persisted before B.kt was added could be.
	snapshot, ok := trackedfiles.FileMetadataProvider{}.Snapshot(dir)
	if !ok {
		t.Fatal("no git snapshot for the test repository")
	}
	if err := (trackedfiles.DiskStore{}).Save(dir, snapshot, []string{"A.kt"}); err != nil {
		t.Fatal(err)
	}
	if warm := count(); warm != 1 {
		t.Fatalf("a run that reads the seeded listing found %d UnusedVariable findings, want 1; the seed no longer reproduces a stale listing", warm)
	}
	if got := count("--no-cache"); got != fresh {
		t.Fatalf("--no-cache over a stale tracked-files listing = %d findings, fresh = %d", got, fresh)
	}
}
