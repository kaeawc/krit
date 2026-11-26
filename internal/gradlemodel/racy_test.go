package gradlemodel

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

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

// An old jar is keyed by stat alone, exactly as before racy handling, so
// existing fir-cache entries stay valid and warm runs never hash jars.
func TestClasspathFingerprintOfOldJarIsStatOnly(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "dep.jar")
	if err := os.WriteFile(jar, []byte("jar-1"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(jar, old, old); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(jar)
	if err != nil {
		t.Fatal(err)
	}
	want := hashutil.HashHex([]byte(jar + "\x00" + strconv.FormatInt(info.Size(), 10) + "\x00" + strconv.FormatInt(info.ModTime().UnixNano(), 10) + "\x00"))
	if got := ClasspathFingerprint([]string{jar}); got != want {
		t.Fatalf("old-jar fingerprint = %s, want the stat-only %s", got, want)
	}
}
