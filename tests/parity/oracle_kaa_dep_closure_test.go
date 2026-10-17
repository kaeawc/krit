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

	cases := []struct {
		name, initialLib, changedLib, use, diagnostic string
		// extra holds unchanging sources, keyed by path under src.
		extra map[string]string
	}{
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
		{
			// Use.kt reaches Impl only through a Java method's return type,
			// and foo() resolves to Base.kt's declaration. Java sources get
			// no dependency fragments of their own, so the edge to Impl.kt
			// must come from the resolved callable's return type.
			name:       "java-factory",
			initialLib: "package p\n\nclass Impl : Base()\n",
			changedLib: "package p\n\nclass Impl : Base() {\n    override fun foo(): String = \"x\"\n}\n",
			use:        "package p\n\nfun use() = JavaFactory.make().foo()!!\n",
			diagnostic: "UNNECESSARY_NOT_NULL_ASSERTION",
			extra: map[string]string{
				"Base.kt":            "package p\n\nopen class Base {\n    open fun foo(): String? = null\n}\n",
				"p/JavaFactory.java": "package p;\n\npublic class JavaFactory {\n    public static Impl make() { return new Impl(); }\n}\n",
			},
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
				for rel, body := range tc.extra {
					path := filepath.Join(src, rel)
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					writeFile(t, path, body)
				}
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

// kaaConventionRefreshCase describes one reference kind whose target only the
// use site's operator convention or failed-call candidates can reach. Lib.kt
// holds the referenced declaration; extra holds unchanging sources.
type kaaConventionRefreshCase struct {
	name, initialLib, changedLib, use, diagnostic string
	extra                                         map[string]string
}

// TestKAAOracleCacheRefreshesConventionAndCandidateDependents extends the
// callable-dependency refresh above to the references whose declaration is
// not named at the use site: constructors, operators, array access,
// iteration, delegation, destructuring, and the sole candidate of a call that
// does not resolve. Editing the referenced declaration in Lib.kt must refresh
// the warm dependent in Use.kt through both the persistent daemon and the
// one-shot CLI. The edits deprecate the declaration so that the compiler
// diagnostic DEPRECATION appears in Use.kt only if its facts were recomputed.
func TestKAAOracleCacheRefreshesConventionAndCandidateDependents(t *testing.T) {
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

	const deprecated = "@Deprecated(\"old\") "
	box := map[string]string{"Box.kt": "package p\n\nclass Box\n"}
	convention := func(name, decl, use string) kaaConventionRefreshCase {
		return kaaConventionRefreshCase{
			name:       name,
			initialLib: "package p\n\n" + decl + "\n",
			changedLib: "package p\n\n" + deprecated + decl + "\n",
			use:        "package p\n\n" + use + "\n",
			diagnostic: "DEPRECATION",
			extra:      box,
		}
	}
	cases := []kaaConventionRefreshCase{
		{
			name:       "constructor",
			initialLib: "package p\n\nclass R\n",
			changedLib: "package p\n\nclass R " + deprecated + "constructor()\n",
			use:        "package p\n\nfun use() = R()\n",
			diagnostic: "DEPRECATION",
		},
		convention("binary-operator", "operator fun Box.plus(o: Box): Box = o", "fun use(a: Box, b: Box) = a + b"),
		convention("array-access", "operator fun Box.get(i: Int): Int = i", "fun use(a: Box) = a[0]"),
		convention("iteration", "operator fun Box.iterator(): Iterator<Int> = listOf(1).iterator()", "fun use(a: Box) { for (x in a) {} }"),
		convention("delegation", "operator fun Box.getValue(t: Any?, p: kotlin.reflect.KProperty<*>): Int = 1", "fun use(a: Box) { val v: Int by a }"),
		convention("destructuring", "operator fun Box.component1(): Int = 1", "fun use(a: Box) { val (x) = a }"),
		{
			// helper("x") does not resolve, so helper(Int) is only a
			// candidate of the failed call. Deprecating it still reaches
			// Use.kt through that candidate edge.
			name:       "failed-call-candidate",
			initialLib: "package p\n\nfun helper(x: Int): Int = x\n",
			changedLib: "package p\n\n" + deprecated + "fun helper(x: Int): Int = x\n",
			use:        "package p\n\nfun use() = helper(\"not an int\")\n",
			diagnostic: "DEPRECATION",
		},
	}
	for _, mode := range []string{"on", "off"} {
		for _, tc := range cases {
			t.Run("daemon-"+mode+"/"+tc.name, func(t *testing.T) {
				runKAAConventionRefreshCase(t, jar, classpath, mode, tc)
			})
		}
	}
}

func runKAAConventionRefreshCase(t *testing.T, jar string, classpath []string, mode string, tc kaaConventionRefreshCase) {
	t.Helper()
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
	for rel, body := range tc.extra {
		writeFile(t, filepath.Join(src, rel), body)
	}
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
	// The cold run must have cached Use.kt, or the warm runs below
	// recompute everything and prove nothing.
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
