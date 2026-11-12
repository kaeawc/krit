package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResDirFingerprintCoversSiblingSourceSets(t *testing.T) {
	src := filepath.Join(t.TempDir(), "app", "src")
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main/res/values/strings.xml", `<resources><string name="a">A</string></resources>`)
	write("debug/res/values-de/strings.xml", `<resources></resources>`)
	write("test/res/values-de/strings.xml", `<resources></resources>`)
	mainRes := filepath.Join(src, "main", "res")
	for _, cacheDir := range []string{"", t.TempDir()} {
		fresh := func() string {
			fps := newResDirFingerprints(cacheDir, 1)
			return fps.fingerprint(mainRes)
		}
		before := fresh()
		write("test/res/values-de/strings.xml", `<resources><string name="t">T</string></resources>`)
		if got := fresh(); got != before {
			t.Fatalf("cacheDir=%q: a test source set change altered the fingerprint", cacheDir)
		}
		write("debug/res/values-de/strings.xml", `<resources><string name="a">A`+cacheDir+`</string></resources>`)
		if got := fresh(); got == before {
			t.Fatalf("cacheDir=%q: a debug source set change kept the fingerprint", cacheDir)
		}
		if got := mergedResourceIndexFingerprint([]string{mainRes}); got != mergedResourceIndexFingerprintWith([]string{mainRes}, nil) {
			t.Fatalf("merged fingerprints disagree")
		}
	}
}
