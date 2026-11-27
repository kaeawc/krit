package gradlemodel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/testutil"
)

func TestClasspathFingerprintSeesSameTickJarRewrite(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "dep.jar")
	if err := os.WriteFile(jar, []byte("jar-1"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := ClasspathFingerprint([]string{jar})
	testutil.RewriteKeepingStat(t, jar, "jar-2")
	if ClasspathFingerprint([]string{jar}) == before {
		t.Fatal("a same-size jar rewrite within one mtime tick kept the fingerprint")
	}
}

// A jar fingerprinted right after a rebuild must key the same in a later
// process, once the jar has aged, or that run misses and repeats the work.
func TestClasspathFingerprintIsStableAcrossProcesses(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "dep.jar")
	if err := os.WriteFile(jar, []byte("jar-1"), 0o644); err != nil {
		t.Fatal(err)
	}
	first := ClasspathFingerprint([]string{jar})
	hashutil.ResetDefault()
	testutil.Age(t, jar)
	if got := ClasspathFingerprint([]string{jar}); got != first {
		t.Fatalf("fingerprint of an unchanged jar changed between processes: %s then %s", first, got)
	}
}

// #1815: a same-size build-file edit inside one mtime tick must make the
// stamp stale in the process that stamped it, not only in a fresh one.
func TestCheckInputsStampSeesSameTickRewrite(t *testing.T) {
	root := t.TempDir()
	writeBuild(t, root)
	dir := filepath.Join(root, ".krit", "gradle-model")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	key, err := WriteInputsStamp(dir, root)
	if err != nil {
		t.Fatal(err)
	}
	catalog := filepath.Join(root, "gradle", "libs.versions.toml")
	testutil.RewriteKeepingStat(t, catalog, "[versions]\nkotlin = \"2.2.1\"")
	if _, changed, err := CheckInputsStamp(dir, root); changed != "gradle/libs.versions.toml" || err != nil {
		t.Fatalf("same-tick catalog edit: changed = %q, err = %v", changed, err)
	}
	if again, _ := InputsKey(root); again == key {
		t.Fatal("a same-tick catalog edit kept the inputs key")
	}
}

// An aged build file rewritten with its size and mtime kept is only provable
// by content. The daemon clears the shared memo per request, so the next
// request's stale check re-reads it.
func TestCheckInputsStampSeesStatPreservingRewriteAfterReset(t *testing.T) {
	root := t.TempDir()
	writeBuild(t, root)
	catalog := filepath.Join(root, "gradle", "libs.versions.toml")
	testutil.Age(t, catalog)
	dir := filepath.Join(root, ".krit", "gradle-model")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteInputsStamp(dir, root); err != nil {
		t.Fatal(err)
	}
	if _, changed, _ := CheckInputsStamp(dir, root); changed != "" {
		t.Fatalf("unchanged build: changed = %q", changed)
	}
	testutil.RewriteKeepingStat(t, catalog, "[versions]\nkotlin = \"2.2.1\"")
	hashutil.ResetDefault()
	if _, changed, err := CheckInputsStamp(dir, root); changed != "gradle/libs.versions.toml" || err != nil {
		t.Fatalf("stat-preserving catalog edit after reset: changed = %q, err = %v", changed, err)
	}
}
