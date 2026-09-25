package parity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/oracle"
)

// TestFirOracleCacheRefreshesAfterUnclassifiedChange drives the cached oracle
// path against a real krit-fir jar, through both the persistent daemon and
// the one-shot CLI. Lib.kt is never classified, because the oracle filter
// keeps only Use.kt or because it lives under a pruned directory, yet
// Use.kt's facts depend on it. Changing Lib.kt produces no cache miss among
// the classified files, and the cache used to serve Use.kt's facts from the
// previous compilation.
func TestFirOracleCacheRefreshesAfterUnclassifiedChange(t *testing.T) {
	root := repoRoot(t)
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}

	cases := []struct {
		name       string
		libPath    string // Lib.kt's path under src; testData/ is pruned from classification
		lib        string // initial Lib.kt
		change     func(t *testing.T, lib string)
		wantBefore bool // UNNECESSARY_NOT_NULL_ASSERTION on Use.kt
	}{
		{
			name:       "edit",
			libPath:    "Lib.kt",
			lib:        "package p\n\nclass R\nfun helper(): R? = R()\n",
			change:     func(t *testing.T, lib string) { writeFile(t, lib, "package p\n\nclass R\nfun helper(): R = R()\n") },
			wantBefore: false,
		},
		{
			name:    "delete",
			libPath: "Lib.kt",
			lib:     "package p\n\nclass R\nfun helper(): R = R()\n",
			change: func(t *testing.T, lib string) {
				if err := os.Remove(lib); err != nil {
					t.Fatal(err)
				}
			},
			wantBefore: true,
		},
		{
			// krit-fir compiles files that classification prunes, such as
			// generated or testData sources. A change there must count.
			name:       "edit-pruned",
			libPath:    filepath.Join("testData", "Lib.kt"),
			lib:        "package p\n\nclass R\nfun helper(): R? = R()\n",
			change:     func(t *testing.T, lib string) { writeFile(t, lib, "package p\n\nclass R\nfun helper(): R = R()\n") },
			wantBefore: false,
		},
	}
	for _, mode := range []string{"daemon", "one-shot"} {
		for _, tc := range cases {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				if mode == "daemon" {
					t.Setenv("KRIT_DAEMON_CACHE", "on")
				} else {
					t.Setenv("KRIT_DAEMON_CACHE", "off")
				}
				// Canonical paths: krit-fir compiles a symlinked path (macOS
				// temp dirs) under both of its names, a separate defect.
				repo, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				srcDir := filepath.Join(repo, "src")
				if err := os.MkdirAll(srcDir, 0o755); err != nil {
					t.Fatal(err)
				}
				lib := filepath.Join(srcDir, tc.libPath)
				if err := os.MkdirAll(filepath.Dir(lib), 0o755); err != nil {
					t.Fatal(err)
				}
				use := filepath.Join(srcDir, "Use.kt")
				writeFile(t, lib, tc.lib)
				writeFile(t, use, "package p\n\nfun use() = helper()!!\n")
				filter := filepath.Join(repo, "filter.txt")
				writeFile(t, filter, use+"\n")
				if mode == "daemon" {
					t.Cleanup(func() {
						if d, err := oracle.ConnectOrStartDaemon(firJar, []string{srcDir}, nil, false); err == nil {
							_ = d.Shutdown()
						}
					})
				}

				run := func(name string) bool {
					t.Helper()
					out := filepath.Join(repo, name+".json")
					opts := oracle.InvocationOptions{Backend: oracle.BackendFIR}
					if _, err := oracle.InvokeCachedWithOptions(firJar, []string{srcDir}, repo, out, filter, false, nil, opts); err != nil {
						t.Fatalf("%s: %v", name, err)
					}
					raw, err := os.ReadFile(out)
					if err != nil {
						t.Fatal(err)
					}
					var data oracle.Data
					if err := json.Unmarshal(raw, &data); err != nil {
						t.Fatalf("%s: parse: %v", name, err)
					}
					f := indexByBasename(data.Files)["Use.kt"]
					if f == nil {
						t.Fatalf("%s: no facts for Use.kt", name)
					}
					for _, d := range f.Diagnostics {
						if d.FactoryName == "UNNECESSARY_NOT_NULL_ASSERTION" {
							return true
						}
					}
					return false
				}

				if got := run("cold"); got != tc.wantBefore {
					t.Fatalf("cold run: unnecessary `!!` reported = %v, want %v", got, tc.wantBefore)
				}
				// The cold run must have cached Use.kt, or the warm runs below
				// recompute everything and prove nothing.
				cacheDir, err := oracle.CacheDir(repo)
				if err != nil {
					t.Fatal(err)
				}
				hits, _ := oracle.ClassifyFilesScopedV3(cacheDir, []string{use}, "", "", oracle.BackendFIR.CacheApproximation())
				if len(hits) != 1 || hits[0].CompilationFingerprint == "" {
					t.Fatalf("cold run did not cache Use.kt for krit-fir: hits=%d", len(hits))
				}
				if got := run("warm-unchanged"); got != tc.wantBefore {
					t.Fatalf("warm run with no change: unnecessary `!!` reported = %v, want %v", got, tc.wantBefore)
				}
				tc.change(t, lib)
				if got := run("warm-changed"); got == tc.wantBefore {
					t.Fatalf("after the Lib.kt %s, Use.kt kept facts from the previous compilation", tc.name)
				}
			})
		}
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
