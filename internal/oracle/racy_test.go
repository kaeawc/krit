package oracle

import (
	"testing"

	"github.com/kaeawc/krit/internal/testutil"
)

func TestCompilationFingerprintSeesSameTickClasspathRewrite(t *testing.T) {
	dir := t.TempDir()
	src := writeTempFile(t, dir, "A.kt", "class A\n")
	jar := writeTempFile(t, dir, "backend.jar", "jar")
	lib := writeTempFile(t, dir, "lib.jar", "lib-1")
	before := CompilationFingerprint([]string{src}, []string{lib}, jar)
	testutil.RewriteKeepingStat(t, lib, "lib-2")
	if CompilationFingerprint([]string{src}, []string{lib}, jar) == before {
		t.Fatal("a same-size classpath jar rewrite within one mtime tick kept the fingerprint")
	}
}
