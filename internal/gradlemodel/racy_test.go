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
// by content. This test clears the shared memo itself, so it does not pin the
// daemon's per-request reset; TestHandleAnalyzeProjectResetsHashMemoBetweenRequests
// in internal/cli/serve does.
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

// #1952: the CLI loads .krit/cache/hash-memo.gob into the shared memo, and
// that file holds digests of aged build files. The stale check must re-read
// its inputs whatever the memo holds, so it does not depend on running
// before the memo is loaded.
func TestCheckInputsStampIgnoresPopulatedMemo(t *testing.T) {
	hashutil.ResetDefault()
	t.Cleanup(hashutil.ResetDefault)
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
	// A previous process memoized the aged catalog and saved the memo.
	if _, err := hashutil.Default().HashFile(catalog, nil); err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(t.TempDir(), "hash-memo.gob")
	if err := hashutil.Default().SavePersisted(saved); err != nil {
		t.Fatal(err)
	}
	hashutil.ResetDefault()
	testutil.RewriteKeepingStat(t, catalog, "[versions]\nkotlin = \"2.2.1\"")
	if hashutil.Default().LoadPersisted(saved) == 0 {
		t.Fatal("the saved memo restored no entries")
	}
	if _, changed, err := CheckInputsStamp(dir, root); changed != "gradle/libs.versions.toml" || err != nil {
		t.Fatalf("stat-preserving catalog edit with a loaded memo: changed = %q, err = %v", changed, err)
	}
}
