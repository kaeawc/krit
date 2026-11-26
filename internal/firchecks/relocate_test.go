package firchecks

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

// copyTree copies src to dst, .krit/ included, like a CI cache restore into
// a fresh checkout.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

type relocationCheckout struct {
	root, src, a, b, jar, dep string
}

// newRelocationCheckout lays out a module under root and its jars under
// gradleHome, the two locations a CI agent can change between builds.
func newRelocationCheckout(t *testing.T, root, gradleHome string) relocationCheckout {
	t.Helper()
	c := relocationCheckout{root: root, src: filepath.Join(root, "app", "src")}
	c.a, c.b = filepath.Join(c.src, "A.kt"), filepath.Join(c.src, "BTest.kt")
	c.jar, c.dep = filepath.Join(gradleHome, "krit-fir.jar"), filepath.Join(gradleHome, "caches", "dep.jar")
	for path, content := range map[string]string{
		c.a: "class A { val x = 1 }\n", c.b: "class BTest\n", c.jar: "fir jar bytes", c.dep: "dependency jar bytes",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func (c relocationCheckout) invoke(t *testing.T, calls *int) *Result {
	t.Helper()
	run := func(_ string, files, _, _, _ []string, _ RuleConfigs, _ FileFacts, _, _ bool, _ ...CompileTarget) (*CheckResponse, error) {
		*calls++
		resp := &CheckResponse{Rules: []string{"R"}}
		for _, f := range files {
			if f == c.a {
				resp.Findings = append(resp.Findings, FirFinding{Path: f, Line: 1, Col: 7, Rule: "R", Severity: "warning", Message: "m"})
			}
		}
		return resp, nil
	}
	facts := FileFacts{
		TestFiles: []string{c.b},
		SDKLevels: map[string]android.SDKLevels{c.a: {MinSdk: 21, TargetSdk: 34}},
	}
	res, err := invokeCachedWith(run, c.jar, []string{c.a, c.b}, []string{c.src}, []string{c.dep}, []string{"R"}, nil, facts, c.root, false, false)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// #1662: a .krit/ written by one CI agent and restored into a checkout at
// another absolute path, with the jars under another GRADLE_USER_HOME,
// serves every unchanged file from the FIR cache.
func TestFirCacheHitsAfterRelocatingCheckoutAndJars(t *testing.T) {
	base := t.TempDir()
	first := newRelocationCheckout(t, filepath.Join(base, "agent-1", "repo"), filepath.Join(base, "home-1"))
	calls := 0
	first.invoke(t, &calls)
	if calls != 1 {
		t.Fatalf("seed run: checker calls = %d, want 1", calls)
	}

	copyTree(t, first.root, filepath.Join(base, "agent-2", "repo"))
	copyTree(t, filepath.Join(base, "home-1"), filepath.Join(base, "home-2"))
	second := newRelocationCheckout(t, filepath.Join(base, "agent-2", "repo"), filepath.Join(base, "home-2"))
	res := second.invoke(t, &calls)
	if calls != 1 {
		t.Fatalf("relocated run re-ran the checker (calls = %d): the restored cache missed", calls)
	}
	if len(res.Findings) != 1 || res.Findings[0].File != second.a {
		t.Fatalf("relocated findings = %+v, want one finding at %s", res.Findings, second.a)
	}
	for _, f := range res.Findings {
		if strings.HasPrefix(f.File, first.root) {
			t.Fatalf("a cached finding still names the old checkout: %s", f.File)
		}
	}

	// An edit in the new checkout still invalidates the compilation.
	if err := os.WriteFile(second.a, []byte("class A { val x = 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second.invoke(t, &calls)
	if calls != 2 {
		t.Fatalf("an edit after relocation was served from the cache (calls = %d)", calls)
	}
}

// A changed jar under the new GRADLE_USER_HOME is a different input.
func TestFirCacheMissesAfterRelocatingWithChangedJar(t *testing.T) {
	base := t.TempDir()
	first := newRelocationCheckout(t, filepath.Join(base, "agent-1", "repo"), filepath.Join(base, "home-1"))
	calls := 0
	first.invoke(t, &calls)
	copyTree(t, first.root, filepath.Join(base, "agent-2", "repo"))
	second := newRelocationCheckout(t, filepath.Join(base, "agent-2", "repo"), filepath.Join(base, "home-2"))
	if err := os.WriteFile(second.dep, []byte("another dependency"), 0o644); err != nil {
		t.Fatal(err)
	}
	second.invoke(t, &calls)
	if calls != 2 {
		t.Fatalf("a different dependency jar was served from the cache (calls = %d)", calls)
	}
}
