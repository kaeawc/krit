package parity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
)

// TestKAAOracleCacheNameDigestAcrossInvocations covers the cases a changed
// declaration reaches a cached caller without the declaring file being
// analyzed in the same run: the oracle filter list drops it, an earlier
// invocation (a subpath scan) analyzed and cached it, or it was deleted. Use.kt
// resolves pick(1) to Lib.kt's pick(Any), so nothing in its dependency closure
// changes when New.kt adds pick(Int); only the declared-name digest stored in
// its cache entry can notice. (Once Use.kt resolves to New.kt, deleting it is
// also seen through the closure; the unit tests isolate the digest.) Each warm
// run must match a clean run.
func TestKAAOracleCacheNameDigestAcrossInvocations(t *testing.T) {
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
	const (
		diagnostic = "UNNECESSARY_NOT_NULL_ASSERTION"
		lib        = "package p\n\nfun pick(x: Any): Int? = null\n"
		use        = "package p\n\nfun usePick() = pick(1)!!\n"
		newFile    = "package p\n\nfun pick(x: Int): Int = 1\n"
	)

	newRepo := func(t *testing.T, files map[string]string) (repo, src string) {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		src = filepath.Join(repo, "src")
		for name, body := range files {
			path := filepath.Join(src, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, body)
		}
		return repo, src
	}
	// scan runs the cached oracle over dirs with a filter list naming only
	// the given base names (none: no filter), and reports whether Use.kt has
	// the diagnostic, how many files it re-analyzed, and which.
	scan := func(t *testing.T, repo, label string, dirs []string, filterBases []string) (has bool, misses int64) {
		t.Helper()
		filter := ""
		if filterBases != nil {
			var lines []string
			for _, dir := range dirs {
				_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return nil //nolint:nilerr // best-effort listing
					}
					for _, base := range filterBases {
						if filepath.Base(p) == base {
							lines = append(lines, p)
						}
					}
					return nil
				})
			}
			filter = filepath.Join(repo, label+".filter")
			writeFile(t, filter, strings.Join(lines, "\n")+"\n")
		}
		out := filepath.Join(repo, label+".json")
		tracker := perf.New(true)
		opts := oracle.InvocationOptions{Backend: oracle.BackendKAA, Classpath: classpath, Tracker: tracker}
		if _, err := oracle.InvokeCachedWithOptions(jar, dirs, repo, out, filter, false, nil, opts); err != nil {
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
		for _, e := range tracker.GetTimings() {
			if e.Name == "cacheClassify" {
				misses = e.Metrics["misses"]
			}
		}
		facts := indexByBasename(data.Files)["Use.kt"]
		if facts == nil {
			if label == "subpath" {
				return false, misses // Use.kt is outside the scanned directory
			}
			t.Fatalf("%s: no facts for Use.kt", label)
		}
		for _, d := range facts.Diagnostics {
			if d.FactoryName == diagnostic {
				has = true
			}
		}
		return has, misses
	}
	copyTree := func(t *testing.T, from, to string) {
		t.Helper()
		_ = filepath.Walk(from, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil //nolint:nilerr // best-effort copy
			}
			rel, _ := filepath.Rel(from, p)
			body, rerr := os.ReadFile(p)
			if rerr != nil {
				t.Fatal(rerr)
			}
			dst := filepath.Join(to, rel)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, dst, string(body))
			return nil
		})
	}
	// clean is what a run with no cache reports for src's current sources.
	clean := func(t *testing.T, src string, filterBases []string) bool {
		t.Helper()
		repo, cleanSrc := newRepo(t, nil)
		copyTree(t, src, cleanSrc)
		has, _ := scan(t, repo, "clean", []string{cleanSrc}, filterBases)
		return has
	}
	stopDaemon := func(t *testing.T, mode, src string) {
		if mode == "on" {
			t.Cleanup(func() {
				if d, err := oracle.ConnectOrStartDaemon(jar, []string{src}, classpath, false); err == nil {
					_ = d.Shutdown()
				}
			})
		}
	}

	for _, mode := range []string{"on", "off"} {
		t.Run("daemon-"+mode+"/filter-list-excludes-declaring-file", func(t *testing.T) {
			t.Setenv("KRIT_DAEMON_CACHE", mode)
			repo, src := newRepo(t, map[string]string{"Lib.kt": lib, "Use.kt": use})
			stopDaemon(t, mode, src)
			listed := []string{"Lib.kt", "Use.kt"}
			if got, _ := scan(t, repo, "seed", []string{src}, listed); got {
				t.Fatalf("seed run unexpectedly reported %s", diagnostic)
			}
			// New.kt contains none of the active rules' identifiers, so the
			// filter list leaves it out. It must still reach the callers.
			writeFile(t, filepath.Join(src, "New.kt"), newFile)
			warm, misses := scan(t, repo, "warm", []string{src}, listed)
			if !warm {
				t.Fatalf("warm run lost %s that a clean run reports (re-analyzed %d file(s))", diagnostic, misses)
			}
			if want := clean(t, src, listed); warm != want {
				t.Fatalf("warm (%v) disagrees with clean (%v)", warm, want)
			}
		})

		t.Run("daemon-"+mode+"/declaring-file-cached-by-a-subpath-scan", func(t *testing.T) {
			t.Setenv("KRIT_DAEMON_CACHE", mode)
			repo, src := newRepo(t, map[string]string{"Lib.kt": lib, "Use.kt": use})
			stopDaemon(t, mode, src)
			if got, _ := scan(t, repo, "seed", []string{src}, nil); got {
				t.Fatalf("seed run unexpectedly reported %s", diagnostic)
			}
			if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(src, "sub", "New.kt"), newFile)
			scan(t, repo, "subpath", []string{filepath.Join(src, "sub")}, nil)
			warm, misses := scan(t, repo, "warm", []string{src}, nil)
			if !warm {
				t.Fatalf("full scan after the subpath scan lost %s that a clean run reports (re-analyzed %d file(s))", diagnostic, misses)
			}
			if want := clean(t, src, nil); warm != want {
				t.Fatalf("warm (%v) disagrees with clean (%v)", warm, want)
			}
		})

		t.Run("daemon-"+mode+"/declaring-file-deleted", func(t *testing.T) {
			t.Setenv("KRIT_DAEMON_CACHE", mode)
			repo, src := newRepo(t, map[string]string{"Lib.kt": lib, "Use.kt": use, "sub/New.kt": newFile})
			stopDaemon(t, mode, src)
			if got, _ := scan(t, repo, "seed", []string{src}, nil); !got {
				t.Fatalf("seed run did not report %s", diagnostic)
			}
			if err := os.Remove(filepath.Join(src, "sub", "New.kt")); err != nil {
				t.Fatal(err)
			}
			warm, misses := scan(t, repo, "warm", []string{src}, nil)
			if warm {
				t.Fatalf("warm run still reports %s after the declaring file was deleted (re-analyzed %d file(s))", diagnostic, misses)
			}
			if want := clean(t, src, nil); warm != want {
				t.Fatalf("warm (%v) disagrees with clean (%v)", warm, want)
			}
		})
	}
}
