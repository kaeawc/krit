package modshard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/gradlemodel"
)

func TestShardsBalanceWorkAndKeepEveryUnit(t *testing.T) {
	var units []Unit
	for i, files := range []int{100, 90, 50, 40, 40, 30, 20, 10, 5, 5, 5, 1} {
		units = append(units, Unit{Spec: firchecks.ModuleSpec{ID: string(rune('a' + i))}, Files: files})
	}
	groups := Shards(units, 3)
	seen := 0
	loads := make([]int, len(groups))
	for i, g := range groups {
		for _, u := range g {
			seen++
			loads[i] += u.Files
		}
	}
	if seen != len(units) {
		t.Fatalf("sharding kept %d of %d units", seen, len(units))
	}
	lo, hi := loads[0], loads[0]
	for _, l := range loads {
		lo, hi = min(lo, l), max(hi, l)
	}
	if hi-lo > 20 {
		t.Fatalf("unbalanced loads %v", loads)
	}
	if got := Shards(units, 0); len(got) != 1 || len(got[0]) != len(units) {
		t.Fatalf("zero workers should mean one group, got %d groups", len(got))
	}
}

func TestSourceRootOf(t *testing.T) {
	cases := map[string]string{
		"/r/app/src/debug/kotlin/a/b/C.kt":  "/r/app/src/debug/kotlin",
		"/r/app/src/main/java/a/C.kt":       "/r/app/src/main/java",
		"/r/tools/scripts/build.gradle.kts": "/r/tools/scripts",
		"/r/lib/src/test/kotlin/x/y/z/Z.kt": "/r/lib/src/test/kotlin",
	}
	for in, want := range cases {
		if got := sourceRootOf(filepath.FromSlash(in)); got != filepath.FromSlash(want) {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
	}
}

// fixtureModel builds a two-project model on disk: :lib (built) and :app depending on it.
func fixtureModel(t *testing.T) (*gradlemodel.Model, string) {
	t.Helper()
	root := t.TempDir()
	write := func(rel string) string {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("class X"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	write("lib/src/main/kotlin/L.kt")
	write("lib/src/test/kotlin/LTest.kt")
	write("app/src/main/kotlin/A.kt")
	libJar := write("lib/build/intermediates/compile_library_classes_jar/release/bundleLibCompileToJarRelease/classes.jar")
	_ = libJar
	set := func(project, name, kind string, deps ...string) gradlemodel.SourceSet {
		return gradlemodel.SourceSet{
			Name: name, Kind: kind, Platform: "android",
			SourceDirs:  []string{filepath.Join(root, project, "src", map[string]string{"main": "main", "test": "test"}[kind], "kotlin")},
			ProjectDeps: deps,
		}
	}
	model := &gradlemodel.Model{Projects: []gradlemodel.Project{
		{Path: ":lib", Dir: filepath.Join(root, "lib"), SourceSets: []gradlemodel.SourceSet{set("lib", "release", "main"), set("lib", "releaseUnitTest", "test")}},
		{Path: ":app", Dir: filepath.Join(root, "app"), SourceSets: []gradlemodel.SourceSet{set("app", "release", "main", ":lib")}},
	}}
	return model, root
}

func TestPlanTurnsProjectDependenciesIntoBuiltJarsAndDropsEdges(t *testing.T) {
	model, root := fixtureModel(t)
	units, missing := Plan(model, root, []string{root})
	if len(units) != 3 || missing != 0 {
		t.Fatalf("got %d units, %d missing jars", len(units), missing)
	}
	byID := map[string]Unit{}
	for _, u := range units {
		byID[u.Spec.ID] = u
	}
	jar := filepath.Join(root, "lib/build/intermediates/compile_library_classes_jar/release/bundleLibCompileToJarRelease/classes.jar")
	app := byID[":app:release"].Spec
	if len(app.DependsOn) != 0 || !contains(app.Classpath, jar) {
		t.Errorf("app should see :lib as a jar and have no module edges: %+v", app)
	}
	test := byID[":lib:releaseUnitTest"].Spec
	if !contains(test.Classpath, jar) || !contains(test.Friends, jar) {
		t.Errorf("a test module should see its main jar on the classpath and as a friend: %+v", test)
	}
	if len(Shards(units, 2)) != 2 {
		t.Error("expected two shards")
	}
}

func TestPlanLimitsUnitsToTheScanPaths(t *testing.T) {
	model, root := fixtureModel(t)
	units, _ := Plan(model, root, []string{filepath.Join(root, "app")})
	if len(units) != 1 || units[0].Spec.ID != ":app:release" {
		t.Fatalf("got %+v", units)
	}
}

func TestPlanCountsMissingDependencyJars(t *testing.T) {
	model, root := fixtureModel(t)
	if err := os.RemoveAll(filepath.Join(root, "lib", "build")); err != nil {
		t.Fatal(err)
	}
	_, missing := Plan(model, root, []string{root})
	if missing != 1 {
		t.Fatalf("expected 1 missing dependency jar, got %d", missing)
	}
}

func TestMergeKeepsFirstUnitsFileAndUnionsDependencies(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, files, deps map[string]any) {
		data, _ := json.Marshal(map[string]any{"version": 1, "kotlinVersion": "2.4", "files": files, "dependencies": deps})
		if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("k1", map[string]any{"/a/A.kt": map[string]any{"package": "first"}, "/a/B.kt": map[string]any{"package": "b"}}, map[string]any{"x.X": map[string]any{}})
	write("k2", map[string]any{"/a/A.kt": map[string]any{"package": "second"}}, map[string]any{"y.Y": map[string]any{}})
	out := filepath.Join(dir, "oracle.json")
	units := []Unit{{Spec: firchecks.ModuleSpec{ID: "one"}}, {Spec: firchecks.ModuleSpec{ID: "two"}}, {Spec: firchecks.ModuleSpec{ID: "failed"}}}
	covered, err := merge(Input{CacheDir: dir, OutputPath: out}, units, []string{"k1", "k2", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(covered) != 2 || !covered["/a/A.kt"] || !covered["/a/B.kt"] {
		t.Fatalf("covered %v", covered)
	}
	var merged struct {
		Files        map[string]struct{ Package string } `json:"files"`
		Dependencies map[string]json.RawMessage          `json:"dependencies"`
	}
	data, _ := os.ReadFile(out)
	if err := json.Unmarshal(data, &merged); err != nil {
		t.Fatalf("merged file is not valid JSON: %v\n%s", err, data)
	}
	if merged.Files["/a/A.kt"].Package != "first" || len(merged.Dependencies) != 2 {
		t.Fatalf("unexpected merge: %+v", merged)
	}
}

func TestUnitKeyChangesWithSourceContentAndClasspath(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	_ = os.MkdirAll(src, 0o755)
	file := filepath.Join(src, "A.kt")
	_ = os.WriteFile(file, []byte("class A"), 0o644)
	jar := filepath.Join(root, "dep.jar")
	_ = os.WriteFile(jar, []byte("1"), 0o644)
	unit := Unit{Spec: firchecks.ModuleSpec{ID: "m", SourceRoots: []string{src}, Classpath: []string{jar}}}
	key := func() string { k, _ := unitKey(unit, jar, newIdentities(), nil); return k }
	k1 := key()
	if k1 != key() {
		t.Fatal("key is not stable")
	}
	_ = os.WriteFile(file, []byte("class B"), 0o644)
	k2 := key()
	if k2 == k1 {
		t.Error("a source edit must change the key")
	}
	_ = os.WriteFile(jar, []byte("22"), 0o644)
	if key() == k2 {
		t.Error("a changed dependency jar must change the key")
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestPruneRemovesOldUnreferencedFactsOnly(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-30 * 24 * time.Hour)
	write := func(name string, mtime time.Time) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	write("live.json", old)
	write("live.check.json", old)
	write("stale-old.json", old)
	write("stale-old.check.json", old)
	write("stale-new.json", time.Now())
	write("worker-0.log", old)
	prune(dir, []string{"live"}, 7*24*time.Hour)
	for name, want := range map[string]bool{"live.json": true, "live.check.json": true, "stale-old.json": false,
		"stale-old.check.json": false, "stale-new.json": true, "worker-0.log": true} {
		_, err := os.Stat(filepath.Join(dir, name))
		if (err == nil) != want {
			t.Errorf("%s: exists=%v, want %v", name, err == nil, want)
		}
	}
}

func TestWorkerHeapStaysWithinBounds(t *testing.T) {
	for _, workers := range []int{1, 4, 8, 14, 64} {
		if h := workerHeapMB(workers); h < 1536 || h > 5120 {
			t.Errorf("workers=%d: heap %dMB outside [1536, 5120]", workers, h)
		}
	}
	if workerHeapMB(0) != 5120 {
		t.Error("no workers should fall back to the default cap")
	}
}

func TestPlanCompilesUnbuiltDependenciesFromSourceAsContext(t *testing.T) {
	model, root := fixtureModel(t)
	if err := os.RemoveAll(filepath.Join(root, "lib", "build")); err != nil {
		t.Fatal(err)
	}
	units, _ := Plan(model, root, []string{root})
	for _, u := range units {
		if u.Spec.ID != ":app:release" {
			continue
		}
		libSources := filepath.Join(root, "lib", "src", "main", "kotlin")
		if !contains(u.Spec.ContextSourceRoots, libSources) {
			t.Fatalf("app should compile :lib's sources as context, got %v", u.Spec.ContextSourceRoots)
		}
		if contains(u.Spec.SourceRoots, libSources) {
			t.Fatal("context roots must not become the module's own roots")
		}
		return
	}
	t.Fatal("no app unit")
}

func TestMergeCachedReusesTheOutputUntilUnitsOrOutputChange(t *testing.T) {
	dir := t.TempDir()
	data, _ := json.Marshal(map[string]any{"version": 1, "kotlinVersion": "2.4", "files": map[string]any{"/a/A.kt": map[string]any{}}, "dependencies": map[string]any{}})
	if err := os.WriteFile(filepath.Join(dir, "k1.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	in := Input{CacheDir: dir, OutputPath: filepath.Join(dir, "oracle.json")}
	units := []Unit{{Spec: firchecks.ModuleSpec{ID: "one"}}}
	if _, err := mergeCached(in, units, []string{"k1"}); err != nil {
		t.Fatal(err)
	}
	first, _ := os.Stat(in.OutputPath)
	covered, err := mergeCached(in, units, []string{"k1"})
	if err != nil || !covered["/a/A.kt"] {
		t.Fatalf("cached merge: %v %v", covered, err)
	}
	if again, _ := os.Stat(in.OutputPath); !again.ModTime().Equal(first.ModTime()) {
		t.Error("an unchanged merge must not rewrite the output")
	}
	// A different key set, or an output replaced by something else, merges again.
	if err := os.WriteFile(in.OutputPath, []byte(`{"files":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	covered, err = mergeCached(in, units, []string{"k1"})
	if err != nil || !covered["/a/A.kt"] {
		t.Fatalf("replaced output should be rebuilt: %v %v", covered, err)
	}
	if body, _ := os.ReadFile(in.OutputPath); !json.Valid(body) || len(body) < 40 {
		t.Errorf("output not rebuilt: %s", body)
	}
}

func TestWorkerJVMGivesARecordingToOneWorkerAndACacheToAll(t *testing.T) {
	cache := workerJVM{aot: []string{"-XX:AOTCache=/c"}}
	for shard := 0; shard < 3; shard++ {
		if got := cache.argsFor(shard); len(got) != 1 {
			t.Errorf("a ready cache should apply to worker %d, got %v", shard, got)
		}
	}
	recording := workerJVM{aot: []string{"-XX:AOTMode=record"}, recording: true}
	if got := recording.argsFor(0); len(got) != 1 {
		t.Errorf("the first worker should record, got %v", got)
	}
	if got := recording.argsFor(1); len(got) != 0 {
		t.Errorf("only one worker may record, got %v", got)
	}
	if got := (workerJVM{}).argsFor(0); len(got) != 0 {
		t.Errorf("no AOT arguments expected, got %v", got)
	}
}

func TestPreferAOTJavaFallsBackWhenNoNewerJDKIsNamed(t *testing.T) {
	t.Setenv("KRIT_WORKER_JAVA", "")
	t.Setenv("JAVA_HOME", t.TempDir())
	got := preferAOTJava("/fallback/java")
	if got == "" {
		t.Fatal("expected some java path")
	}
}

func TestCopyFileLeavesNoPartialFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.json")
	dst := filepath.Join(dir, "dst.json")
	if err := os.WriteFile(src, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dst); string(got) != `{"ok":true}` {
		t.Fatalf("dst = %q", got)
	}
	missing := filepath.Join(dir, "never.json")
	if err := copyFile(filepath.Join(dir, "absent.json"), missing); err == nil {
		t.Fatal("copy of a missing source should fail")
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("failed copy left a destination file")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.tmp.*")); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func TestUnitKeyChangesWithClasspathDirectoryContents(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	_ = os.MkdirAll(src, 0o755)
	_ = os.WriteFile(filepath.Join(src, "A.kt"), []byte("class A"), 0o644)
	classes := filepath.Join(root, "classes", "pkg")
	_ = os.MkdirAll(classes, 0o755)
	class := filepath.Join(classes, "Dep.class")
	_ = os.WriteFile(class, []byte("v1"), 0o644)
	unit := Unit{Spec: firchecks.ModuleSpec{ID: "m", SourceRoots: []string{src}, Classpath: []string{filepath.Join(root, "classes")}}}
	key := func() string { k, _ := unitKey(unit, "", newIdentities(), nil); return k }
	k1 := key()
	if k1 != key() {
		t.Fatal("key is not stable for an unchanged directory")
	}
	_ = os.WriteFile(class, []byte("version2"), 0o644)
	k2 := key()
	if k2 == k1 {
		t.Error("editing a class inside a classpath directory must change the key")
	}
	_ = os.WriteFile(filepath.Join(classes, "New.class"), []byte("x"), 0o644)
	if key() == k2 {
		t.Error("adding a class inside a classpath directory must change the key")
	}
}
