package parity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
)

// TestKAAOracleCacheRefreshesCallersWhenNewFileAddsOverload covers a change
// no cached dependency closure can see: Use.kt resolved pick(1) to Lib.kt's
// pick(Any), and a brand-new file in the same package adds pick(Int), which
// the call now resolves to. Nothing in Use.kt's closure changed, so closure
// verification kept its stale facts (a missing
// UNNECESSARY_NOT_NULL_ASSERTION). The warm run must match a clean run, for
// the persistent daemon and the one-shot path alike. A new file that cannot
// affect Use.kt must not re-analyze it.
func TestKAAOracleCacheRefreshesCallersWhenNewFileAddsOverload(t *testing.T) {
	root := repoRoot(t)
	jar := filepath.Join(root, "tools", "krit-types", "build", "libs", "krit-types.jar")
	if !isExecutableKAAJar(jar) {
		t.Skip("executable krit-types jar not found; run `cd tools/krit-types && ./gradlew shadowJar`")
	}
	stdlib := findKotlinStdlib()
	if stdlib == "" {
		t.Skip("kotlin-stdlib jar not found in Gradle cache")
	}
	classpath := []string{stdlib}
	const diagnostic = "UNNECESSARY_NOT_NULL_ASSERTION"

	cases := []struct {
		name, newFile string
		// wantMisses is the warm run's re-analysis count: the new file
		// alone, or the new file plus Use.kt and Lib.kt (which also mentions pick).
		wantMisses int64
		wantDiag   bool
	}{
		{"overload", "package p\n\nfun pick(x: Int): Int = 1\n", 3, true},
		{"unrelated", "package p\n\nfun other(): Int = 1\n", 1, false},
		{"other-package", "package q\n\nfun pick(x: Int): Int = 1\n", 1, false},
	}

	// hasDiagnostic runs the cached oracle and reports whether Use.kt has
	// the diagnostic, plus the number of files the run re-analyzed.
	hasDiagnostic := func(t *testing.T, repo, src, label string, tracker perf.Tracker) (bool, int64) {
		t.Helper()
		out := filepath.Join(repo, label+".json")
		opts := oracle.InvocationOptions{Backend: oracle.BackendKAA, Classpath: classpath, Tracker: tracker}
		if _, err := oracle.InvokeCachedWithOptions(jar, []string{src}, repo, out, "", false, nil, opts); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		raw, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		var data oracle.Data
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Fatalf("%s: parse: %v", label, err)
		}
		file := indexByBasename(data.Files)["Use.kt"]
		if file == nil {
			t.Fatalf("%s: no facts for Use.kt", label)
		}
		var misses int64
		if tracker != nil {
			for _, e := range tracker.GetTimings() {
				if e.Name == "cacheClassify" {
					misses = e.Metrics["misses"]
				}
			}
		}
		for _, d := range file.Diagnostics {
			if d.FactoryName == diagnostic {
				return true, misses
			}
		}
		return false, misses
	}
	newRepo := func(t *testing.T) (string, string) {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		src := filepath.Join(repo, "src")
		if err := os.MkdirAll(src, 0o755); err != nil {
			t.Fatal(err)
		}
		return repo, src
	}

	for _, mode := range []string{"on", "off"} {
		for _, tc := range cases {
			t.Run("daemon-"+mode+"/"+tc.name, func(t *testing.T) {
				t.Setenv("KRIT_DAEMON_CACHE", mode)
				repo, src := newRepo(t)
				writeFile(t, filepath.Join(src, "Lib.kt"), "package p\n\nfun pick(x: Any): Int? = null\n")
				writeFile(t, filepath.Join(src, "Use.kt"), "package p\n\nfun usePick() = pick(1)!!\n")
				if mode == "on" {
					t.Cleanup(func() {
						if d, err := oracle.ConnectOrStartDaemon(jar, []string{src}, classpath, false); err == nil {
							_ = d.Shutdown()
						}
					})
				}

				if got, _ := hasDiagnostic(t, repo, src, "seed", nil); got {
					t.Fatalf("seed run unexpectedly reported %s", diagnostic)
				}
				writeFile(t, filepath.Join(src, "New.kt"), tc.newFile)
				warm, misses := hasDiagnostic(t, repo, src, "warm", perf.New(true))
				if warm != tc.wantDiag {
					t.Fatalf("warm run reported %s = %v, want %v", diagnostic, warm, tc.wantDiag)
				}
				if misses != tc.wantMisses {
					t.Fatalf("warm run re-analyzed %d file(s), want %d", misses, tc.wantMisses)
				}

				// A clean run, with no cache, must agree with the warm one.
				cleanRepo, cleanSrc := newRepo(t)
				if mode == "on" {
					t.Cleanup(func() {
						if d, err := oracle.ConnectOrStartDaemon(jar, []string{cleanSrc}, classpath, false); err == nil {
							_ = d.Shutdown()
						}
					})
				}
				for _, name := range []string{"Lib.kt", "Use.kt", "New.kt"} {
					body, err := os.ReadFile(filepath.Join(src, name))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(cleanSrc, name), string(body))
				}
				if clean, _ := hasDiagnostic(t, cleanRepo, cleanSrc, "clean", nil); clean != warm {
					t.Fatalf("warm (%v) disagrees with clean (%v) for %s", warm, clean, diagnostic)
				}
			})
		}
	}
}
