package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheIdentityTracksJarAndVersions(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	t.Setenv("KRIT_FIR_JAR", jar)
	if err := os.WriteFile(jar, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := CacheIdentity(nil)
	if err := os.WriteFile(jar, []byte("second, longer"), 0o600); err != nil {
		t.Fatal(err)
	}
	if second := CacheIdentity(nil); second == first {
		t.Fatal("CacheIdentity ignored a jar content change")
	}
	if !strings.Contains(first, "oracle-v") || !strings.Contains(first, "facts-v") {
		t.Fatalf("CacheIdentity %q lacks protocol/schema versions", first)
	}
}
