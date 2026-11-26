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
