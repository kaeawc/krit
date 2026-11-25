package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

// UnsafeNativeCodeLocation reads the files directly inside src/main/assets, so a change there must change the res
// directory fingerprint that keys its cached findings.
func TestResDirFingerprintCoversTopLevelAssets(t *testing.T) {
	main := filepath.Join(t.TempDir(), "app", "src", "main")
	resDir := filepath.Join(main, "res")
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(main, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fingerprint := func() string {
		fps := newResDirFingerprints(filepath.Join(t.TempDir(), "cache"), 1)
		return fps.fingerprint(resDir)
	}
	write("res/values/strings.xml", "<resources/>")
	base := fingerprint()

	write("assets/sub/nested.so", "\x7fELFnested")
	if got := fingerprint(); got != base {
		t.Fatalf("a nested asset changed the fingerprint; only top-level assets are read")
	}
	write("assets/libnative.so", "\x7fELFone")
	added := fingerprint()
	if added == base {
		t.Fatalf("adding a top-level asset did not change the res directory fingerprint")
	}
	write("assets/libnative.so", "\x7fELFtwo-longer")
	if got := fingerprint(); got == added {
		t.Fatalf("editing a top-level asset did not change the res directory fingerprint")
	}
}
