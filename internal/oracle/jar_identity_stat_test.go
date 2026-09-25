package oracle

import (
	"os"
	"testing"
	"time"
)

func TestJarIdentityChangesWithStat(t *testing.T) {
	p := testJar(t)
	first := JarIdentity(p)
	if len(first) != 8 || first == "missing" {
		t.Fatalf("initial identity = %q", first)
	}
	if err := os.WriteFile(p, []byte("longer"), 0644); err != nil {
		t.Fatal(err)
	}
	second := JarIdentity(p)
	if second == first {
		t.Fatal("size change did not change identity")
	}
	stamp := time.Now().Add(10 * time.Second)
	if err := os.Chtimes(p, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if JarIdentity(p) == second {
		t.Fatal("mtime change did not change identity")
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if JarIdentity(p) != "missing" {
		t.Fatal("missing jar did not get missing identity")
	}
}
