package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/hashutil"
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
	write("assets/libnative.so", "no longer native")
	if got := fingerprint(); got == added {
		t.Fatalf("a top-level asset ceasing to be an ELF binary did not change the res directory fingerprint")
	}
}

// The assets folders of the module's other source sets are read too (#1619), so they key the cached findings as well.
func TestResDirFingerprintCoversOtherSourceSetAssets(t *testing.T) {
	src := filepath.Join(t.TempDir(), "app", "src")
	resDir := filepath.Join(src, "main", "res")
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(src, rel)
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
	write("main/res/values/strings.xml", "<resources/>")
	base := fingerprint()
	write("test/assets/libtest.so", "\x7fELFtest")
	if got := fingerprint(); got != base {
		t.Fatalf("a test source set's asset changed the fingerprint; test assets are not read")
	}
	write("debug/assets/libdebug.so", "\x7fELFdebug")
	withDebug := fingerprint()
	if withDebug == base {
		t.Fatalf("adding a debug source set asset did not change the res directory fingerprint")
	}
	// The rule follows symlinks (os.Stat), so a symlinked ELF binary keys the findings too.
	target := filepath.Join(t.TempDir(), "libreal.so")
	if err := os.WriteFile(target, []byte("\x7fELFreal"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(src, "debug", "assets", "liblink.so")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if got := fingerprint(); got == withDebug {
		t.Fatalf("adding a symlinked ELF asset did not change the res directory fingerprint")
	}
}

// The rule reads only an asset's name and its 4-byte header, so the key depends on nothing else: a warm run must not
// hash large non-native assets (models, media) in full, and a rebuilt .so that stays an ELF binary yields the same
// findings.
func TestResDirAssetsFingerprintIgnoresContentBeyondHeader(t *testing.T) {
	src := filepath.Join(t.TempDir(), "app", "src")
	resDir := filepath.Join(src, "main", "res")
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(src, rel)
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
	write("main/res/values/strings.xml", "<resources/>")
	write("free/assets/model.bin", "weights-v1")
	write("free/assets/libfree.so", "\x7fELFv1")
	base := fingerprint()
	write("free/assets/model.bin", "weights-v2-retrained")
	write("free/assets/libfree.so", "\x7fELFv2-rebuilt")
	if got := fingerprint(); got != base {
		t.Fatalf("editing asset content past the ELF header changed the fingerprint")
	}
	write("free/assets/model.bin", "\x7fELFnow-native")
	if got := fingerprint(); got == base {
		t.Fatalf("an asset becoming an ELF binary did not change the fingerprint")
	}
}

// BenchmarkResDirAssetsFingerprintLargeAsset models a warm run: a fresh process (empty hash memo) keys a module with
// one large non-native asset.
func BenchmarkResDirAssetsFingerprintLargeAsset(b *testing.B) {
	src := filepath.Join(b.TempDir(), "app", "src")
	resDir := filepath.Join(src, "main", "res")
	if err := os.MkdirAll(filepath.Join(resDir, "values"), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "free", "assets"), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "free", "assets", "model.bin"), make([]byte, 64<<20), 0o644); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		hashutil.ResetDefault()
		_ = resDirAssetsFingerprint(resDir)
	}
}
