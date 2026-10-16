package parity_test

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
)

// wcBackend describes one compiler-backed oracle for the warm-vs-clean matrix.
// Each backend is compared with its own clean baseline; the two backends are
// never compared with each other.
type wcBackend struct {
	name    string
	backend oracle.Backend
	jarEnv  string
	// jar returns the executable helper jar, or "" when it is missing.
	jar func(root string) string
	// classpath is the backend's base oracle classpath.
	classpath func() ([]string, string)
}

func wcBackends() []wcBackend {
	return []wcBackend{
		{
			name:    "fir",
			backend: oracle.BackendFIR,
			jarEnv:  oracle.BackendFIR.JarEnvVar(),
			jar: func(root string) string {
				jar := firchecks.FindFirJar([]string{root})
				if jar == "" || !isExecutableJar(jar) {
					return ""
				}
				return jar
			},
			classpath: func() ([]string, string) { return nil, "" },
		},
		{
			name:    "kaa",
			backend: oracle.BackendKAA,
			jarEnv:  oracle.BackendKAA.JarEnvVar(),
			jar: func(root string) string {
				jar := oracle.FindJar([]string{root})
				if jar == "" || !isExecutableKAAJar(jar) {
					return ""
				}
				return jar
			},
			// kotlin.Deprecated lives in the stdlib, which krit-types does not
			// bundle into its analysis classpath.
			classpath: func() ([]string, string) {
				stdlib := findKotlinStdlib()
				if stdlib == "" {
					return nil, "kotlin-stdlib jar not found in Gradle cache; set KOTLIN_STDLIB_JAR"
				}
				return []string{stdlib}, ""
			},
		},
	}
}

// TestWarmCleanMutationFIR and TestWarmCleanMutationKAA drive the real helper
// jars through the oracle cache (one-shot and persistent-daemon paths, absolute
// and relative scan roots). They follow the oracle-backend-parity job's
// contract: a missing jar skips the top-level test, and that job fails the run
// on a top-level skip, so the matrix cannot silently stop running in CI.
func TestWarmCleanMutationFIR(t *testing.T) { runWarmCleanOracleMatrix(t, wcBackends()[0]) }

func TestWarmCleanMutationKAA(t *testing.T) { runWarmCleanOracleMatrix(t, wcBackends()[1]) }

func runWarmCleanOracleMatrix(t *testing.T, be wcBackend) {
	wcRequireMatrix(t)
	root := repoRoot(t)
	jar := be.jar(root)
	if jar == "" {
		t.Skipf("%s executable helper jar not found; build tools/krit-%s with `./gradlew shadowJar`", be.name, map[string]string{"fir": "fir", "kaa": "types"}[be.name])
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	baseCP, skipWhy := be.classpath()
	if skipWhy != "" {
		t.Skip(skipWhy)
	}
	shared, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mx := &wcMatrix{
		be: be, jar: jar, baseCP: baseCP, shared: shared, home: filepath.Join(shared, "home"),
		altJar: wcIdentityJar(t, jar, filepath.Join(shared, "identity-"+filepath.Base(jar))),
		seeds:  map[string]*wcSeed{}, cleans: map[string]wcClean{},
	}
	if err := os.MkdirAll(mx.home, 0o755); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	defer func() { t.Logf("%s matrix wall time: %s", be.name, time.Since(started).Round(time.Second)) }()

	// The relative-root rows are the #805 bug class (a warm run that spells
	// paths relative to the working directory). They cover the mutations that
	// move the affected set most: an ABI edit, a new file, a rename, an import.
	relRows := map[string]bool{"return-type": true, "file-add": true, "file-rename": true, "import-change": true}

	for _, spelling := range []string{"abs", "rel"} {
		for _, mode := range []string{"one-shot", "daemon"} {
			for _, m := range wcMutations() {
				if m.structuralOnly || (spelling == "rel" && !relRows[m.name]) {
					continue
				}
				if m.skip != "" && (spelling != "abs" || mode != "one-shot") {
					continue
				}
				t.Run(fmt.Sprintf("%s/%s/%s", mode, spelling, m.name), func(t *testing.T) {
					if m.skip != "" {
						t.Skip(m.skip)
					}
					if m.classpath && mx.greeter == "" {
						mx.greeter = wcClasspathJar(t, filepath.Join(shared, "greeter.jar"))
					}
					c := &wcOracleCase{mx: mx, mode: mode, rel: spelling == "rel", m: m}
					c.run(t)
				})
			}
		}
	}
}

// wcMatrix holds what the rows of one backend share. A one-shot row's warm
// state is only on-disk cache, so its seed is built once per project shape and
// restored byte-for-byte for each mutation. Daemon rows seed a fresh daemon
// each, since a resident session cannot be restored. Clean baselines depend
// only on the final tree and the inputs, so rows that mutate the same way
// share one fresh one-shot run.
type wcMatrix struct {
	be      wcBackend
	jar     string
	altJar  string
	greeter string
	baseCP  []string
	shared  string
	home    string
	seeds   map[string]*wcSeed
	cleans  map[string]wcClean
}

type wcSeed struct {
	root, backup string
	snap         wcSnapshot
	stats        wcRunStats
}

type wcClean struct {
	snap     wcSnapshot
	stats    wcRunStats
	identity string
}

type wcOracleCase struct {
	mx   *wcMatrix
	mode string
	rel  bool
	m    wcMutation
}

// wcRunStats is what a run reports about how it used the cache.
type wcRunStats struct {
	hits, misses int
	classified   int
}

type wcInputs struct {
	jar       string
	classpath []string
}

func (in wcInputs) String() string {
	return fmt.Sprintf("jar=%s classpath=%v", filepath.Base(filepath.Dir(in.jar))+"/"+filepath.Base(in.jar), wcBaseNames(in.classpath))
}

func wcBaseNames(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
	}
	return out
}

func (c *wcOracleCase) run(t *testing.T) {
	// A private daemon registry per case: the daemons this case starts are
	// its own and die with it, so the matrix never accumulates JVMs.
	home := c.mx.home
	if c.mode == "daemon" {
		home = t.TempDir()
		t.Cleanup(func() { killRegisteredDaemons(t, home) })
	}
	// One-shot rows start no daemon, so they share one HOME (and its JVM
	// class-data archives) instead of paying an archive build per case.
	t.Setenv("HOME", home)
	if c.mode == "daemon" {
		t.Setenv("KRIT_DAEMON_CACHE", "on")
	} else {
		t.Setenv("KRIT_DAEMON_CACHE", "off")
	}
	problem, detail := c.exercise(t)
	wcResolveRow(t, fmt.Sprintf("%s/%s/%s/%s", c.mx.be.name, c.mode, c.spellingKey(), c.m.name), problem, detail)
}

// exercise runs the seed / control / mutate / warm / clean sequence and
// returns the first thing that is wrong ("" when nothing is) with a full
// report. Rows listed in wcKnownStale turn a problem into a skip, so every
// check that can be tripped by a known product bug returns instead of failing.
// This harness calls the oracle with a nil store and no call filter, as the
// default CLI flow does; it does not model users of a .krit/store.
func (c *wcOracleCase) exercise(t *testing.T) (problem, detail string) {
	mx, be := c.mx, c.mx.be
	seedIn := wcInputs{jar: mx.jar, classpath: mx.baseCP}
	warmIn := seedIn
	if c.m.classpath {
		warmIn.classpath = append(append([]string(nil), mx.baseCP...), mx.greeter)
	}
	if c.m.jarIdentity {
		warmIn.jar = mx.altJar
	}

	// 1. Seed: a cold run that must populate the cache.
	warmRoot, seed, seedStats := c.seed(t, seedIn)
	if seedStats.classified == 0 || seedStats.hits != 0 {
		t.Fatalf("seed run is not cold: %+v", seedStats)
	}
	if !wcSeedUsable(seed) {
		return "the seed run returned no facts", fmt.Sprintf("%s %s path, %s scan root: the seed run is unusable: %s", be.name, c.mode, c.spellingKey(), wcSeedFailure(seed))
	}
	if got := c.storedEntries(t, warmRoot, seedIn); got != seedStats.classified {
		return "the seed run did not populate the cache", fmt.Sprintf("seed run populated %d cache entries for %d classified files; a warm run would prove nothing", got, seedStats.classified)
	}

	// 2. No-op control: must be served from the cache and change nothing.
	control, controlStats := c.invoke(t, warmRoot, "control", seedIn)
	if controlStats.misses != 0 || controlStats.hits != seedStats.classified {
		return "the no-op control missed the cache", fmt.Sprintf("no-op control did not hit the cache: %+v (seeded %d files)", controlStats, seedStats.classified)
	}
	if diff := wcDiff(control, seed); diff != "" {
		return "the cache-served control differs from the seed run", "no-op control changed the result relative to the seed run:\n" + diff
	}

	// 3. Mutate, then run warm on the same cache/session.
	if c.m.apply != nil {
		c.m.apply(t, warmRoot)
	}
	warm, warmStats := c.invoke(t, warmRoot, "warm", warmIn)

	// 4. A genuinely fresh session: a copy of the final tree with its own
	//    cache directory, in a fresh process.
	clean := c.clean(t, warmRoot, warmIn)
	if clean.stats.hits != 0 {
		t.Fatalf("clean baseline was not cold: %+v", clean.stats)
	}
	t.Logf("cache use: seed=%+v control=%+v warm=%+v clean=%+v", seedStats, controlStats, warmStats, clean.stats)

	changed := wcSnapshotsDiffer(clean.snap, seed)
	switch c.m.effect {
	case wcEffectChanges:
		if !changed {
			t.Errorf("mutation %s had no effect on the clean result; the case no longer tests anything", c.m.name)
		}
	case wcEffectSame:
		if changed {
			t.Errorf("mutation %s changed the clean result but was expected to be a no-op:\n%s", c.m.name, wcDiff(clean.snap, seed))
		}
	}
	if c.m.jarIdentity && warmStats.hits != 0 {
		t.Errorf("a byte-different helper jar still hit %d cache entries; helper identity must invalidate the store", warmStats.hits)
	}

	if diff := wcDiff(warm, clean.snap); diff != "" {
		return "the warm result differs from a clean scan", fmt.Sprintf(
			"STALE WARM RESULT\n  mutation: %s\n  backend:  %s (%s path, %s scan root)\n  seed inputs:  %s\n  warm inputs:  %s\n  clean inputs: %s (fresh one-shot, isolated cache)\n  warm tree:  %s\n  clean tree: %s\n  warm cache: hits=%d misses=%d (clean: misses=%d)\n  differing keys (warm vs clean):\n%s",
			c.m.name, be.name, c.mode, c.spellingKey(), seedIn, warmIn, warmIn,
			wcTreeIdentity(warmRoot), clean.identity,
			warmStats.hits, warmStats.misses, clean.stats.misses, diff)
	}
	return "", ""
}

// seed returns a project root holding a freshly seeded cache and the seed
// run's snapshot. One-shot rows restore a shared, once-built seed; daemon rows
// seed a fresh project (and so a fresh daemon).
func (c *wcOracleCase) seed(t *testing.T, in wcInputs) (string, wcSnapshot, wcRunStats) {
	t.Helper()
	mx := c.mx
	if c.mode == "daemon" {
		root := wcCanonicalDir(t)
		wcWriteProject(t, root, c.m)
		snap, stats := c.invoke(t, root, "seed", in)
		return root, snap, stats
	}
	extras := make([]string, 0, len(c.m.extra))
	for rel := range c.m.extra {
		extras = append(extras, rel)
	}
	sort.Strings(extras)
	key := fmt.Sprintf("%v|%v", c.rel, extras)
	seed := mx.seeds[key]
	if seed == nil {
		// Build the seed, retrying once if the helper errored or returned no
		// facts (a loaded runner can do that to a cold JVM). A seed that never
		// comes out usable is not shared: every later row would restore it and
		// fail for the same transient reason.
		built := &wcSeed{root: wcCanonicalDirIn(t, mx.shared), backup: wcCanonicalDirIn(t, mx.shared)}
		usable := false
		for attempt := 1; attempt <= 2 && !usable; attempt++ {
			for _, dir := range []string{"src", ".krit"} {
				if err := os.RemoveAll(filepath.Join(built.root, dir)); err != nil {
					t.Fatal(err)
				}
			}
			wcWriteProject(t, built.root, c.m)
			built.snap, built.stats = c.invoke(t, built.root, "seed", in)
			usable = wcSeedUsable(built.snap)
			if !usable {
				t.Logf("seed attempt %d unusable: %s", attempt, wcSeedFailure(built.snap))
			}
		}
		if !usable {
			return built.root, built.snap, built.stats
		}
		wcCopyTreeKeepCache(t, built.root, built.backup)
		mx.seeds[key] = built
		seed = built
	}
	// Restore the seeded project and cache exactly, whatever the previous
	// row left behind.
	for _, dir := range []string{"src", ".krit"} {
		if err := os.RemoveAll(filepath.Join(seed.root, dir)); err != nil {
			t.Fatal(err)
		}
	}
	wcCopyTreeKeepCache(t, seed.backup, seed.root)
	return seed.root, seed.snap, seed.stats
}

// clean returns the fresh-session baseline for the final tree in root: a
// one-shot run over a copy with an empty cache. Rows that reach the same
// final tree with the same inputs share it.
func (c *wcOracleCase) clean(t *testing.T, root string, in wcInputs) wcClean {
	t.Helper()
	mx := c.mx
	identity := wcTreeIdentity(root)
	key := fmt.Sprintf("%s|%s|%s", c.m.name, in, identity)
	if got, ok := mx.cleans[key]; ok {
		return got
	}
	cleanRoot := wcCanonicalDirIn(t, mx.shared)
	wcCopyTree(t, root, cleanRoot)
	// A fresh one-shot process, whatever path the warm row exercised, from
	// the absolute spelling.
	t.Setenv("KRIT_DAEMON_CACHE", "off")
	fresh := &wcOracleCase{mx: mx, mode: "one-shot", m: c.m}
	snap, stats := fresh.invoke(t, cleanRoot, "clean", in)
	t.Setenv("KRIT_DAEMON_CACHE", map[string]string{"daemon": "on", "one-shot": "off"}[c.mode])
	got := wcClean{snap: snap, stats: stats, identity: wcTreeIdentity(cleanRoot)}
	mx.cleans[key] = got
	return got
}

func wcCanonicalDir(t *testing.T) string { return wcCanonicalDirIn(t, t.TempDir()) }

func wcCanonicalDirIn(t *testing.T, parent string) string {
	t.Helper()
	dir, err := os.MkdirTemp(parent, "wc")
	if err != nil {
		t.Fatal(err)
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// spellingKey is the scan-root spelling as it appears in wcKnownStale keys.
func (c *wcOracleCase) spellingKey() string {
	if c.rel {
		return "rel"
	}
	return "abs"
}

// invoke runs the cached oracle path once over root and returns the
// normalized snapshot.
func (c *wcOracleCase) invoke(t *testing.T, root, label string, in wcInputs) (wcSnapshot, wcRunStats) {
	t.Helper()
	t.Setenv(c.mx.be.jarEnv, in.jar)
	srcDirs := []string{filepath.Join(root, "src")}
	if c.rel {
		restore := wcChdir(t, root)
		defer restore()
		srcDirs = []string{"src"}
	}
	out := filepath.Join(t.TempDir(), label+".json")
	tracker := perf.New(true)
	opts := oracle.InvocationOptions{Backend: c.mx.be.backend, Classpath: in.classpath, Tracker: tracker}
	_, runErr := oracle.InvokeCachedWithOptions(in.jar, srcDirs, root, out, "", false, nil, opts)

	snap := wcSnapshot{}
	if runErr != nil {
		snap["run-error"] = wcScrub(runErr.Error(), root)
	} else {
		raw, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		var data oracle.Data
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Fatalf("%s: parse: %v", label, err)
		}
		for path, file := range data.Files {

			snap["facts|"+wcNormPath(root, path)] = wcCanonical(root, file)
		}
		declared := wcDeclaredFQNs(&data)
		for fqn, cls := range data.Dependencies {
			// krit-fir lists the project's own source classes among the
			// dependencies of a fresh run, while a run served from the cache
			// reassembles dependencies from the stored per-file entries and
			// omits them. Those classes are already compared as per-file
			// declarations, so only library dependencies are compared here.
			if cls != nil && cls.JARPath == "" && declared(fqn) {
				continue
			}
			snap["dep|"+fqn] = wcCanonical(root, cls)
		}
	}
	files, err := oracle.CollectKtFiles(srcDirs)
	if err != nil {
		t.Fatalf("%s: collect: %v", label, err)
	}
	stats := wcRunStats{classified: len(files)}
	if entry, ok := wcFindTiming(tracker.GetTimings(), "cacheClassify"); ok {
		stats.hits = int(entry.Metrics["hits"])
		stats.misses = int(entry.Metrics["misses"])
	} else {
		t.Fatalf("%s: oracle cache was bypassed (no cacheClassify timing)", label)
	}
	c.addEntries(t, snap, root, in, files)
	return snap, stats
}

// addEntries adds the stored cache entries (dependency closures and, for a
// whole-compilation backend, the compilation fingerprint's presence) for the
// scanned files to snap.
func (c *wcOracleCase) addEntries(t *testing.T, snap wcSnapshot, root string, in wcInputs, files []string) {
	t.Helper()
	cacheDir, err := oracle.CacheDir(root)
	if err != nil {
		t.Fatal(err)
	}
	hits, _ := oracle.ClassifyFilesScopedV3(cacheDir, files, "", "", c.mx.be.backend.CacheApproximation())
	for _, h := range hits {
		deps := wcNormPaths(root, h.Closure.DepPaths)
		prop := wcNormPaths(root, h.Closure.PropagatingDepPaths)
		perFile := make([]string, 0, len(h.PerFileDeps))
		for fqn := range h.PerFileDeps {
			perFile = append(perFile, fqn)
		}
		sort.Strings(perFile)
		snap["entry|"+wcNormPath(root, h.FilePath)] = fmt.Sprintf("closure=%v propagating=%v perFileDeps=%v crashed=%v wholeCompilationStamp=%v",
			deps, prop, perFile, h.Crashed, h.CompilationFingerprint != "")
	}
}

// storedEntries counts the cache entries the backend would reuse for root.
func (c *wcOracleCase) storedEntries(t *testing.T, root string, in wcInputs) int {
	t.Helper()
	t.Setenv(c.mx.be.jarEnv, in.jar)
	srcDirs := []string{filepath.Join(root, "src")}
	if c.rel {
		defer wcChdir(t, root)()
		srcDirs = []string{"src"}
	}
	files, err := oracle.CollectKtFiles(srcDirs)
	if err != nil {
		t.Fatal(err)
	}
	cacheDir, err := oracle.CacheDir(root)
	if err != nil {
		t.Fatal(err)
	}
	hits, _ := oracle.ClassifyFilesScopedV3(cacheDir, files, "", "", c.mx.be.backend.CacheApproximation())
	n := 0
	for _, h := range hits {
		if c.mx.be.backend.ReturnsWholeCompilation() && h.CompilationFingerprint == "" {
			continue
		}
		n++
	}
	return n
}

// wcDeclaredFQNs returns a predicate for FQNs declared by the project's own
// source files (or nested inside such a declaration).
func wcDeclaredFQNs(data *oracle.Data) func(string) bool {
	declared := map[string]bool{}
	for _, f := range data.Files {
		for _, cls := range f.Declarations {
			declared[cls.FQN] = true
		}
	}
	return func(fqn string) bool {
		for name := fqn; name != ""; {
			if declared[name] {
				return true
			}
			i := strings.LastIndex(name, ".")
			if i < 0 {
				break
			}
			name = name[:i]
		}
		return false
	}
}

func wcFindTiming(entries []perf.TimingEntry, name string) (perf.TimingEntry, bool) {
	for _, e := range entries {
		if e.Name == name {
			return e, true
		}
		if got, ok := wcFindTiming(e.Children, name); ok {
			return got, true
		}
	}
	return perf.TimingEntry{}, false
}

func wcChdir(t *testing.T, dir string) func() {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(old); err != nil {
			t.Fatal(err)
		}
	}
}

// wcNormPath renders a path the oracle reported (absolute, or in the caller's
// relative spelling) relative to the project root.
func wcNormPath(root, path string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	if rel, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return path
}

func wcNormPaths(root string, paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = wcNormPath(root, p)
	}
	sort.Strings(out)
	return out
}

func wcScrub(s, root string) string {
	return strings.ReplaceAll(s, root, "<root>")
}

// wcCanonical marshals v (maps sort their keys) with the project root
// scrubbed, so facts from two project copies compare equal.
func wcCanonical(root string, v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "<unmarshalable: " + err.Error() + ">"
	}
	return wcScrub(string(raw), root)
}

// wcIdentityJar writes a copy of jar whose bytes differ (a changed archive
// comment) while every entry, including the manifest, is carried over
// unchanged. A helper that is byte-different but behaviorally identical is
// the cheapest way to exercise helper-identity invalidation.
func wcIdentityJar(t *testing.T, jar, dst string) string {
	t.Helper()
	zr, err := zip.OpenReader(jar)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	f, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, entry := range zr.File {
		w, err := zw.CreateRaw(&entry.FileHeader)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(entry.Name, "/") {
			continue
		}
		r, err := entry.OpenRaw()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(w, r); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.SetComment("krit warm/clean identity probe"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return dst
}

// wcClasspathJar compiles a one-class Java library (lib.Greeter) with javac
// and packs it into a jar, the artifact the classpath-change case adds.
func wcClasspathJar(t *testing.T, dst string) string {
	t.Helper()
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Fatalf("javac not found (the parity job installs a JDK): %v", err)
	}
	work := t.TempDir()
	src := filepath.Join(work, "lib", "Greeter.java")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, src, "package lib;\n\npublic class Greeter {\n    public static String greet() { return \"hi\"; }\n}\n")
	out := filepath.Join(work, "classes")
	if outb, err := exec.Command(javac, "-d", out, src).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, outb)
	}
	class, err := os.ReadFile(filepath.Join(out, "lib", "Greeter.class"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("lib/Greeter.class")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(class); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return dst
}

func wcHasFacts(snap wcSnapshot) bool {
	for k := range snap {
		if strings.HasPrefix(k, "facts|") {
			return true
		}
	}
	return false
}

// wcSeedUsable reports whether a seed run produced facts and did not error.
func wcSeedUsable(snap wcSnapshot) bool {
	_, failed := snap["run-error"]
	return !failed && wcHasFacts(snap)
}

// wcSeedFailure says why a seed run is unusable: the helper's error when it
// had one, else that it returned no facts.
func wcSeedFailure(snap wcSnapshot) string {
	if err, failed := snap["run-error"]; failed {
		return "oracle run error: " + err
	}
	return "the oracle returned no facts"
}
