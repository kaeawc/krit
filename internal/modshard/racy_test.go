package modshard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/testutil"
)

func TestIdentitySeesSameTickRewrite(t *testing.T) {
	root := t.TempDir()
	jar := filepath.Join(root, "dep.jar")
	classes := filepath.Join(root, "classes")
	class := filepath.Join(classes, "A.class")
	if err := os.MkdirAll(classes, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{jar, class} {
		if err := os.WriteFile(p, []byte("v1"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{jar, classes} {
		before := newIdentities().of(p)
		target := p
		if p == classes {
			target = class
		}
		testutil.RewriteKeepingStat(t, target, "v2")
		if newIdentities().of(p) == before {
			t.Errorf("identity of %s did not change after a same-size rewrite within one mtime tick", p)
		}
	}
}
