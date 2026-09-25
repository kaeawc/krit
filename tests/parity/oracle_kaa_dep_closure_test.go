package parity_test

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

// TestKAAOracleCacheRefreshesResolvedCallableDependents drives the cached
// krit-types oracle path through both the persistent daemon and the one-shot
// CLI. Use.kt resolves a top-level function declared in Lib.kt, from the same
// package or through an import, and extends nothing from it. krit-types used
// to record only imported classes and supertypes as cache dependencies, so
// when Lib.kt changed helper()'s signature or deprecated it, Use.kt stayed a
// cache hit and kept its previous compiler diagnostics.
func TestKAAOracleCacheRefreshesResolvedCallableDependents(t *testing.T) {
	root := repoRoot(t)
	jar := filepath.Join(root, "tools", "krit-types", "build", "libs", "krit-types.jar")
	if !isExecutableKAAJar(jar) {
		t.Skip("executable krit-types jar not found; run `cd tools/krit-types && ./gradlew shadowJar`")
	}
	// kotlin.Deprecated lives in the stdlib, which krit-types does not
	// bundle into its analysis classpath.
	stdlib := findKotlinStdlib()
	if stdlib == "" {
		t.Skip("kotlin-stdlib jar not found in Gradle cache")
	}
	classpath := []string{stdlib}

	cases := []struct{ name, initialLib, changedLib, use, diagnostic string }{
		{
			name:       "same-package",
			initialLib: "package p\n\nclass R\nfun helper(): R? = R()\n",
			changedLib: "package p\n\nclass R\nfun helper(): R = R()\n",
			use:        "package p\n\nfun use() = helper()!!\n",
			diagnostic: "UNNECESSARY_NOT_NULL_ASSERTION",
		},
		{
			name:       "imported",
			initialLib: "package p\n\nclass R\nfun helper(): R? = R()\n",
			changedLib: "package p\n\nclass R\nfun helper(): R = R()\n",
			use:        "package q\n\nimport p.helper\n\nfun use() = helper()!!\n",
			diagnostic: "UNNECESSARY_NOT_NULL_ASSERTION",
		},
		{
			name:       "deprecated",
			initialLib: "package p\n\nfun helper(): Int = 1\n",
			changedLib: "package p\n\n@Deprecated(\"old\")\nfun helper(): Int = 1\n",
			use:        "package p\n\nfun use(): Int = helper()\n",
			diagnostic: "DEPRECATION",
		},
	}
	for _, mode := range []string{"on", "off"} {
		for _, tc := range cases {
			t.Run("daemon-"+mode+"/"+tc.name, func(t *testing.T) {
				t.Setenv("KRIT_DAEMON_CACHE", mode)
				repo, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				src := filepath.Join(repo, "src")
				if err := os.MkdirAll(src, 0o755); err != nil {
					t.Fatal(err)
				}
				lib := filepath.Join(src, "Lib.kt")
				use := filepath.Join(src, "Use.kt")
				writeFile(t, lib, tc.initialLib)
				writeFile(t, use, tc.use)
				if mode == "on" {
					t.Cleanup(func() {
						if d, err := oracle.ConnectOrStartDaemon(jar, []string{src}, classpath, false); err == nil {
							_ = d.Shutdown()
						}
					})
				}

				run := func(label string) bool {
					t.Helper()
					out := filepath.Join(repo, label+".json")
					opts := oracle.InvocationOptions{Backend: oracle.BackendKAA, Classpath: classpath}
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
					for _, d := range file.Diagnostics {
						if d.FactoryName == tc.diagnostic {
							return true
						}
					}
					return false
				}

				if run("cold") {
					t.Fatalf("cold run unexpectedly reported %s", tc.diagnostic)
				}
				// The cold run must have cached Use.kt, or the warm runs
				// below recompute everything and prove nothing.
				cacheDir, err := oracle.CacheDir(repo)
				if err != nil {
					t.Fatal(err)
				}
				hits, _ := oracle.ClassifyFilesScopedV3(cacheDir, []string{use}, "", "", oracle.BackendKAA.CacheApproximation())
				if len(hits) != 1 {
					t.Fatalf("cold run did not cache Use.kt: hits=%d", len(hits))
				}
				if run("warm-unchanged") {
					t.Fatalf("unchanged warm run unexpectedly reported %s", tc.diagnostic)
				}
				writeFile(t, lib, tc.changedLib)
				if !run("warm-changed") {
					t.Fatalf("after the Lib.kt change, Use.kt kept stale facts: missing %s", tc.diagnostic)
				}
			})
		}
	}
}

func isExecutableKAAJar(path string) bool {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "META-INF/MANIFEST.MF" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return false
		}
		defer rc.Close()
		body, err := io.ReadAll(rc)
		return err == nil && strings.Contains(string(body), "Main-Class: dev.jasonpearson.krit.types.MainKt")
	}
	return false
}
