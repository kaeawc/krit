package parity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/oracle"
)

func TestFirOracleSymlinkedSourcePathsCacheUnderCallerForm(t *testing.T) {
	root := repoRoot(t)
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	t.Setenv("KRIT_DAEMON_CACHE", "on")
	// Scan through an explicit symlink so every platform exercises the
	// non-canonical spelling (macOS's /var -> /private/var alone would leave
	// this test vacuous on Linux).
	tmp := t.TempDir()
	real := filepath.Join(tmp, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(tmp, "linked")
	if err := os.Symlink(real, repo); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	srcDir := filepath.Join(repo, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lib := filepath.Join(srcDir, "Lib.kt")
	use := filepath.Join(srcDir, "Use.kt")
	writeFile(t, lib, "package p\nclass R\nfun helper(): R? = R()\n")
	writeFile(t, use, "package p\nfun use() = helper()!!\n")
	t.Cleanup(func() {
		if d, err := oracle.ConnectOrStartDaemon(firJar, []string{srcDir}, nil, false); err == nil {
			_ = d.Shutdown()
		}
	})
	cacheDir, err := oracle.CacheDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{lib, use}
	for _, name := range []string{"cold", "warm"} {
		if !t.Run(name, func(t *testing.T) {
			if name == "warm" {
				hits, misses := oracle.ClassifyFilesScopedV3(cacheDir, paths, "", "", oracle.BackendFIR.CacheApproximation())
				if len(hits) != 2 || len(misses) != 0 {
					t.Fatalf("before warm run: hits=%d misses=%v", len(hits), misses)
				}
			}
			out := filepath.Join(repo, name+".json")
			if _, err := oracle.InvokeCachedWithOptions(firJar, []string{srcDir}, repo, out, "", true, nil,
				oracle.InvocationOptions{Backend: oracle.BackendFIR}); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			var data oracle.Data
			if err := json.Unmarshal(raw, &data); err != nil {
				t.Fatal(err)
			}
			// A file compiled under two spellings comes back twice. Redeclaration
			// errors can't be asserted here: ERROR-severity diagnostics are not
			// retained in the oracle payload.
			if len(data.Files) != 2 {
				t.Fatalf("files=%d, want 2: %v", len(data.Files), data.Files)
			}
			for _, path := range paths {
				if data.Files[path] == nil {
					t.Fatalf("missing Go path %q; keys: %v", path, data.Files)
				}
			}
			hits, misses := oracle.ClassifyFilesScopedV3(cacheDir, paths, "", "", oracle.BackendFIR.CacheApproximation())
			if len(hits) != 2 || len(misses) != 0 {
				t.Fatalf("Go path cache classification: hits=%d misses=%v", len(hits), misses)
			}
		}) {
			t.FailNow()
		}
	}
}
