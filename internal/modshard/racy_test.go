package modshard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/testutil"
)

// racyFixture is a classpath jar and a class directory holding one class.
func racyFixture(t *testing.T) (jar, classes, class string) {
	t.Helper()
	root := t.TempDir()
	jar = filepath.Join(root, "dep.jar")
	classes = filepath.Join(root, "classes")
	class = filepath.Join(classes, "A.class")
	if err := os.MkdirAll(classes, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{jar, class} {
		if err := os.WriteFile(p, []byte("v1"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return jar, classes, class
}

func TestIdentitySeesSameTickRewrite(t *testing.T) {
	jar, classes, class := racyFixture(t)
	for p, target := range map[string]string{jar: jar, classes: class} {
		before := newIdentities().of(p)
		testutil.RewriteKeepingStat(t, target, "v2")
		if newIdentities().of(p) == before {
			t.Errorf("identity of %s did not change after a same-size rewrite within one mtime tick", p)
		}
	}
}

func TestIdentityIsStableAcrossProcesses(t *testing.T) {
	jar, classes, class := racyFixture(t)
	first := map[string]string{jar: newIdentities().of(jar), classes: newIdentities().of(classes)}
	hashutil.ResetDefault()
	testutil.Age(t, jar)
	testutil.Age(t, class)
	for p, want := range first {
		if got := newIdentities().of(p); got != want {
			t.Errorf("identity of unchanged %s changed between processes: %s then %s", p, want, got)
		}
	}
}
