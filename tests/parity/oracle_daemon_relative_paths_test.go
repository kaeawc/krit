package parity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/oracle"
)

var (
	relA = filepath.Join("src", "main", "kotlin", "app", "A.kt")
	relB = filepath.Join("src", "main", "kotlin", "app", "B.kt")
)

type relativeOracleCase struct {
	name      string
	backend   oracle.Backend
	jar       func(root string) string
	requested []string
	// symlink reaches the project through a symlinked directory, so the
	// absolute paths krit sends are not canonical: the daemon must echo the
	// spelling it was sent, not the resolved one.
	symlink bool
}

// relativeOracleCases are the oracle daemons a `krit .` scan can use, each
// reached directly and through a symlink. krit-fir answers for the whole
// compilation, so it also reports files it was not asked about (walked from
// the source root); krit-types answers only for the requested files.
func relativeOracleCases() []relativeOracleCase {
	backends := []relativeOracleCase{
		{name: "fir", backend: oracle.BackendFIR, jar: func(root string) string {
			jar := firchecks.FindFirJar([]string{root})
			if jar == "" || !isExecutableJar(jar) {
				return ""
			}
			return jar
		}, requested: []string{relA}},
		{name: "kaa", backend: oracle.BackendKAA, jar: func(root string) string {
			return oracle.FindJar([]string{root})
		}, requested: []string{relA, relB}},
	}
	var out []relativeOracleCase
	for _, b := range backends {
		out = append(out, b)
		b.name += "-symlink"
		b.symlink = true
		out = append(out, b)
	}
	return out
}

// relativeOracleProject writes a two-file project and chdirs into it, the
// way `krit .` runs: FindSourceDirs then reports "src/main/kotlin" and the
// scanner and the oracle cache spell files relative to it.
func relativeOracleProject(t *testing.T, tc relativeOracleCase) string {
	t.Helper()
	jar := tc.jar(repoRoot(t))
	if jar == "" {
		t.Skip("oracle jar not found; run `./gradlew shadowJar` in tools/krit-fir or tools/krit-types")
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	// A private registry: the daemons this test starts are its own.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Cleanup(func() { killRegisteredDaemons(t, home) })

	dir := t.TempDir()
	if tc.symlink {
		// Resolve the temp dir first so the link is the only non-canonical
		// step on every platform (macOS's /var -> /private/var aside).
		real, err := filepath.EvalSymlinks(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(real, "real"), 0o755); err != nil {
			t.Fatal(err)
		}
		dir = filepath.Join(real, "linked")
		if err := os.Symlink(filepath.Join(real, "real"), dir); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, relA)), 0o755); err != nil {
		t.Fatal(err)
	}
	// r!! on a non-null R is UNNECESSARY_NOT_NULL_ASSERTION, a retained
	// diagnostic that needs nothing from the stdlib.
	writeFile(t, filepath.Join(dir, relA), "package app\n\nopen class R\n\nfun f(r: R): R = r!!\n")
	// B.kt's class extends A.kt's, so the closure has an edge to map back.
	writeFile(t, filepath.Join(dir, relB), "package app\n\nclass S : R()\n")
	t.Chdir(dir)
	if tc.symlink {
		if abs, err := filepath.Abs(relA); err != nil || abs != filepath.Join(dir, relA) {
			t.Fatalf("working directory does not keep the symlinked spelling: %q, %v", abs, err)
		}
	}
	return jar
}

// The oracle daemon is sent absolute paths (it does not share the caller's
// working directory); its facts must still come back under the caller's
// relative spelling, which is how Oracle lookups and the cache index them.
func TestOracleDaemonRelativeRequestKeepsCallerSpelling(t *testing.T) {
	for _, tc := range relativeOracleCases() {
		t.Run(tc.name, func(t *testing.T) {
			jar := relativeOracleProject(t, tc)
			d, err := oracle.ConnectOrStartDaemon(jar, []string{filepath.Join("src", "main", "kotlin")}, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = d.Release() })

			data, deps, err := d.AnalyzeWithDeps(tc.requested)
			if err != nil {
				t.Fatal(err)
			}
			if len(data.Files) != 2 || data.Files[relA] == nil || data.Files[relB] == nil {
				t.Fatalf("Files = %v, want %q and %q", keys(data.Files), relA, relB)
			}
			for path, entry := range deps.Files {
				if filepath.IsAbs(path) {
					t.Fatalf("CacheDeps.Files keyed by absolute %q; keys: %v", path, keys(deps.Files))
				}
				for _, dep := range entry.DepPaths {
					if filepath.IsAbs(dep) {
						t.Fatalf("CacheDeps.Files[%q] has absolute edge %q", path, dep)
					}
				}
			}
			if entry := deps.Files[relB]; entry == nil || !contains(entry.DepPaths, relA) {
				t.Fatalf("CacheDeps.Files[%q] = %+v, want an edge to %q; keys: %v", relB, entry, relA, keys(deps.Files))
			}
			if len(deps.Crashed) != 0 {
				t.Fatalf("crashed = %v", deps.Crashed)
			}
			o, err := oracle.LoadFromData(data)
			if err != nil {
				t.Fatal(err)
			}
			if !lookedUpDiagnostic(o.LookupDiagnostics(relA), "UNNECESSARY_NOT_NULL_ASSERTION") {
				t.Fatalf("LookupDiagnostics(%q) = %v", relA, o.LookupDiagnostics(relA))
			}
		})
	}
}

// A relative `krit .` oracle run through the daemon must cache the analyzed
// files as real entries, not as jar-skipped poison entries.
func TestOracleDaemonRelativeScanCachesRealEntries(t *testing.T) {
	for _, tc := range relativeOracleCases() {
		t.Run(tc.name, func(t *testing.T) {
			jar := relativeOracleProject(t, tc)
			t.Setenv("KRIT_DAEMON_CACHE", "on")
			sourceDirs := oracle.FindSourceDirs([]string{"."})
			if len(sourceDirs) != 1 || filepath.IsAbs(sourceDirs[0]) {
				t.Fatalf("sourceDirs = %v, want one relative root", sourceDirs)
			}
			out := filepath.Join(t.TempDir(), "types.json")
			if _, err := oracle.InvokeCachedWithOptions(jar, sourceDirs, ".", out, "", false, nil,
				oracle.InvocationOptions{Backend: tc.backend}); err != nil {
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
			if len(data.Files) != 2 || data.Files[relA] == nil || data.Files[relB] == nil {
				t.Fatalf("output files = %v, want %q and %q", keys(data.Files), relA, relB)
			}
			cacheDir, err := oracle.CacheDir(".")
			if err != nil {
				t.Fatal(err)
			}
			hits, misses := oracle.ClassifyFilesScopedV3(cacheDir, []string{relA, relB}, "", "", tc.backend.CacheApproximation())
			if len(hits) != 2 || len(misses) != 0 {
				t.Fatalf("cache: hits=%d misses=%v", len(hits), misses)
			}
			for _, hit := range hits {
				if hit.Crashed {
					t.Fatalf("cached %q as a crash: %s", hit.FilePath, hit.CrashError)
				}
				if hit.FilePath == relA && (hit.FileResult == nil || !hasDiagnostic(hit.FileResult, "UNNECESSARY_NOT_NULL_ASSERTION")) {
					t.Fatalf("cached %q lacks the diagnostic: %+v", relA, hit.FileResult)
				}
			}
		})
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func lookedUpDiagnostic(diags []oracle.Diagnostic, factory string) bool {
	for _, d := range diags {
		if d.FactoryName == factory {
			return true
		}
	}
	return false
}
