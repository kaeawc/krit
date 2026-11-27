package serve

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/trackedfiles"
)

// A daemon --no-cache request must detect the Android project from a fresh
// git listing, as the one-shot CLI does: Android detection used to read the
// persisted tracked-files listing, and the resident project slot could hold
// a detection built from it (#1831).
func TestAnalyzeProject_NoCacheIgnoresStaleTrackedFilesCache(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	isolateFromHelperJars(t)
	startServer := func(t *testing.T, root string) string {
		socket, _ := startServerWith(t, root, 5*time.Second)
		return socket
	}
	root := t.TempDir()
	writeKotlinFile(t, root, "A.kt", "package demo\n\nclass A\n")
	writeKotlinFile(t, root, "build.gradle.kts", "dependencies {\n    implementation(\"com.example:lib:1.+\")\n}\n")
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "add", "A.kt", "build.gradle.kts"},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "init"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gradleFinding := func(socket string, noCache bool) bool {
		t.Helper()
		var r daemon.AnalyzeProjectResult
		args := daemon.AnalyzeProjectArgs{NoFir: true, NoTypeOracle: true, NoCache: noCache, EnableRules: "DynamicVersion"}
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject, args, &r); err != nil {
			t.Fatal(err)
		}
		return strings.Contains(string(r.Findings), "build.gradle.kts")
	}

	if !gradleFinding(startServer(t, root), true) {
		t.Fatal("cold --no-cache request: no DynamicVersion finding in build.gradle.kts")
	}
	if _, err := os.Stat(trackedfiles.CacheDir(root)); !os.IsNotExist(err) {
		t.Errorf("cold --no-cache request created %s (stat err %v)", trackedfiles.CacheDir(root), err)
	}
	// Seed a listing that leaves out build.gradle.kts under the current
	// snapshot, as one persisted before the file was added could.
	snapshot, ok := trackedfiles.FileMetadataProvider{}.Snapshot(root)
	if !ok {
		t.Fatal("no git snapshot for the test repository")
	}
	if err := (trackedfiles.DiskStore{}).Save(root, snapshot, []string{"A.kt"}); err != nil {
		t.Fatal(err)
	}
	listing := trackedfiles.CacheDir(root)
	before, err := os.Stat(listing)
	if err != nil {
		t.Fatal(err)
	}
	// On a new daemon, a cached request reads the seeded listing and fills
	// the resident project slot from it; the --no-cache request after it
	// must use neither.
	socket := startServer(t, root)
	if gradleFinding(socket, false) {
		t.Fatal("a request that reads the seeded listing still found build.gradle.kts; the seed no longer reproduces a stale listing")
	}
	if !gradleFinding(socket, true) {
		t.Fatal("--no-cache request over a stale tracked-files listing lost the build.gradle.kts finding")
	}
	if after, err := os.Stat(listing); err != nil || !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("--no-cache request touched the tracked-files listing (stat err %v)", err)
	}
}
