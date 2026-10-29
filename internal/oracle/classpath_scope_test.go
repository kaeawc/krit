package oracle

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/buildid"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/store"
)

func TestClasspathChangesStoreScopeAndTypesFreshness(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "dep.jar")
	types := filepath.Join(dir, "types.json")
	if err := os.WriteFile(jar, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(types, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	before := NewStoreScope(BackendFIR, "", []string{jar})
	if err := RecordTypesFacts(types, false, before); err != nil {
		t.Fatal(err)
	}
	if !TypesJSONSatisfies(types, false, before, buildid.Token()) {
		t.Fatal("fresh types rejected")
	}
	time.Sleep(time.Millisecond)
	if err := os.WriteFile(jar, []byte("replaced jar"), 0644); err != nil {
		t.Fatal(err)
	}
	after := NewStoreScope(BackendFIR, "", []string{jar})
	if before.version == after.version {
		t.Fatal("store version unchanged")
	}
	if TypesJSONSatisfies(types, false, after, buildid.Token()) {
		t.Fatal("stale types accepted")
	}
}

// A settings-only change must change the store scope: the
// facts were computed under another language level or opt-in set, with no
// source, classpath or jar change to show for it.
func TestKotlinSettingsChangeStoreScope(t *testing.T) {
	base := gradlemodel.KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0", OptIns: []string{"a.A"}, Flags: []string{"-Xcontext-parameters"}}
	variants := map[string]gradlemodel.KotlinSettings{
		"language":    {LanguageVersion: "1.9", APIVersion: "2.0", OptIns: base.OptIns, Flags: base.Flags},
		"api":         {LanguageVersion: "2.0", APIVersion: "1.9", OptIns: base.OptIns, Flags: base.Flags},
		"opt-in":      {LanguageVersion: "2.0", APIVersion: "2.0", OptIns: []string{"a.A", "b.B"}, Flags: base.Flags},
		"flag":        {LanguageVersion: "2.0", APIVersion: "2.0", OptIns: base.OptIns, Flags: []string{"-Xexplicit-backing-fields"}},
		"progressive": {LanguageVersion: "2.0", APIVersion: "2.0", OptIns: base.OptIns, Flags: base.Flags, Progressive: true},
		"none":        {},
	}
	for _, backend := range []Backend{BackendFIR} {
		for _, target := range []string{"", "17"} {
			ref := NewStoreScopeWithSettings(backend, "", nil, target, base)
			if ref.KotlinSettings == "" {
				t.Fatalf("%s: scope does not record the settings identity", backend)
			}
			for name, other := range variants {
				got := NewStoreScopeWithSettings(backend, "", nil, target, other)
				if got.version == ref.version {
					t.Errorf("%s target=%q: %s change did not change the store scope", backend, target, name)
				}
			}
			// Opt-in order and duplicates are not semantic.
			reordered := base
			reordered.OptIns = []string{"a.A", "a.A"}
			if NewStoreScopeWithSettings(backend, "", nil, target, reordered).version != ref.version {
				t.Errorf("%s target=%q: equivalent settings produced another scope", backend, target)
			}
		}
		// The scope with no settings is exactly the pre-settings scope.
		if NewStoreScopeWithSettings(backend, "", nil, "17", gradlemodel.KotlinSettings{}).version != NewStoreScopeWithTarget(backend, "", nil, "17").version {
			t.Errorf("%s: zero settings changed the existing scope identity", backend)
		}
	}
	// Settings and the JVM target are independent dimensions of the scope.
	a := NewStoreScopeWithSettings(BackendFIR, "", nil, "11", base)
	b := NewStoreScopeWithSettings(BackendFIR, "", nil, "17", base)
	if a.version == b.version {
		t.Fatal("JVM target stopped affecting the scope once settings were present")
	}
}

// Kotlin settings no longer bypass the persistent daemon: the daemon registry
// is keyed on them instead. A run with settings and no JVM target keeps the
// daemon path.
func TestKotlinSettingsKeepTheDaemonPath(t *testing.T) {
	t.Setenv("KRIT_DAEMON_CACHE", "on")
	opts := InvocationOptions{Backend: BackendFIR, Kotlin: gradlemodel.KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0"}}
	if use, reason := oneShotMissAnalysisReason(1, 1, opts); use {
		t.Fatalf("settings forced a one-shot compile (%q); the daemon must serve them", reason)
	}
	// The FIR JVM-target rule is unchanged.
	opts.JvmTarget = "17"
	if use, _ := oneShotMissAnalysisReason(1, 1, opts); !use {
		t.Fatal("FIR with a JVM target still needs the target-scoped one-shot")
	}
}

// A daemon launched for one set of Kotlin settings is never reused for another,
// and the settings are passed at launch.
func TestDaemonRegistryIsKeyedOnKotlinSettings(t *testing.T) {
	jar, dirs := "/jars/krit-fir.jar", []string{"/src"}
	older := gradlemodel.KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0"}
	newer := gradlemodel.KotlinSettings{LanguageVersion: "2.2", APIVersion: "2.2"}
	if daemonRegistryKeyKT(jar, dirs, gradlemodel.KotlinSettings{}) != daemonRegistryKey(jar, dirs) {
		t.Fatal("zero settings must keep the existing registry key")
	}
	keys := map[string]bool{}
	for _, kt := range []gradlemodel.KotlinSettings{{}, older, newer, {LanguageVersion: "2.0", APIVersion: "2.0", Flags: []string{"-Xwhen-guards"}}} {
		key := daemonRegistryKeyKT(jar, dirs, kt, "/a.jar")
		if keys[key] {
			t.Fatalf("settings %+v collide on registry key %q", kt, key)
		}
		keys[key] = true
	}
	d := &Daemon{sourcesHash: daemonRegistryKeyKT(jar, dirs, older, "/a.jar"), kotlin: older}
	if !d.MatchesRepoSettings(jar, dirs, older, "/a.jar") || !d.MatchesRepo(jar, dirs, "/a.jar") {
		t.Fatal("daemon must match the settings it was launched with")
	}
	if d.MatchesRepoSettings(jar, dirs, newer, "/a.jar") || d.MatchesRepoSettings(jar, dirs, gradlemodel.KotlinSettings{}, "/a.jar") {
		t.Fatal("daemon launched for other settings must not match")
	}
	pool := &DaemonPool{Members: []*Daemon{d}}
	if !pool.MatchesRepoSettings(jar, dirs, older, "/a.jar") || pool.MatchesRepoSettings(jar, dirs, newer, "/a.jar") {
		t.Fatal("pool must require every member to carry the settings")
	}
	args := appendDaemonJarArgsKT(nil, jar, dirs, nil, older, "--daemon")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--kotlin-arg -language-version=2.0") || !strings.Contains(joined, "--kotlin-arg -api-version=2.0") {
		t.Fatalf("launch args lack the settings: %v", args)
	}
	if strings.Contains(strings.Join(appendDaemonJarArgs(nil, jar, dirs, nil, "--daemon"), " "), "--kotlin-arg") {
		t.Fatal("no settings must add no launch flags")
	}
}

func TestCompileContextArgsCarryKotlinSettings(t *testing.T) {
	opts := InvocationOptions{
		Backend: BackendFIR, Classpath: []string{"/a.jar"},
		Kotlin: gradlemodel.KotlinSettings{LanguageVersion: "1.9", APIVersion: "1.8", OptIns: []string{"x.Y"}, Flags: []string{"-Xcontext-parameters"}},
	}
	got := compileContextArgs("/jars/x.jar", opts)
	want := []string{
		"--classpath", "/a.jar",
		"--kotlin-arg", "-language-version=1.9", "--kotlin-arg", "-api-version=1.8",
		"--kotlin-arg", "-opt-in=x.Y", "--kotlin-arg", "-Xcontext-parameters",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v want %v", got, want)
	}
	opts.JvmTarget = "17"
	if got := compileContextArgs("/jars/x.jar", opts); !strings.Contains(strings.Join(got, " "), "--jvm-target 17") || !strings.Contains(strings.Join(got, " "), "--kotlin-arg -language-version=1.9") {
		t.Fatalf("fir args = %v", got)
	}
	if got := compileContextArgs("/jars/x.jar", InvocationOptions{Backend: BackendFIR}); len(got) != 0 {
		t.Fatalf("no settings must add no arguments: %v", got)
	}
}

func TestJvmTargetChangesFIRStoreScope(t *testing.T) {
	a := NewStoreScopeWithTarget(BackendFIR, "", nil, "11")
	b := NewStoreScopeWithTarget(BackendFIR, "", nil, "17")
	if a.version == b.version {
		t.Fatal("JVM target did not affect FIR store scope")
	}
}

// A changed dependency jar must move the legacy (no .krit/store) entry
// directory so unchanged files miss instead of reusing stale facts (#827).
func TestLegacyEntryDirIsScopedByClasspath(t *testing.T) {
	tmp := t.TempDir()
	base, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(tmp, "dep.jar")
	if err := os.WriteFile(jar, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	kt := gradlemodel.KotlinSettings{}
	if LegacyEntryDir(base, kt, nil) != base {
		t.Fatal("empty classpath must keep the original directory")
	}
	before := LegacyEntryDir(base, kt, []string{jar})
	if before == base {
		t.Fatal("classpath must scope the entry directory")
	}
	if again := LegacyEntryDir(base, kt, []string{jar}); again != before {
		t.Fatalf("unchanged classpath moved the directory: %q vs %q", before, again)
	}
	if err := os.WriteFile(jar, []byte("replaced jar"), 0644); err != nil {
		t.Fatal(err)
	}
	if after := LegacyEntryDir(base, kt, []string{jar}); after == before {
		t.Fatal("changed jar reused the old entry directory")
	}
	withKt := gradlemodel.KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0"}
	if LegacyEntryDir(base, withKt, []string{jar}) == LegacyEntryDir(base, kt, []string{jar}) {
		t.Fatal("settings must still separate directories under one classpath")
	}
}

// legacyClasspathRepo seeds a valid FIR entry for one file in the legacy dir
// scoped by kt and classpath.
func legacyClasspathRepo(t *testing.T, kt gradlemodel.KotlinSettings, classpath []string) (repo, srcDir, cacheDir, file string) {
	t.Helper()
	repo = t.TempDir()
	srcDir = filepath.Join(repo, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file = writeTempFile(t, srcDir, "A.kt", "package demo\nfun a() = 1\n")
	var err error
	if cacheDir, err = CacheDir(repo); err != nil {
		t.Fatal(err)
	}
	// Stamped with the compilation the run will compute, so a hit needs no
	// whole-compilation refresh.
	opts := InvocationOptions{Backend: BackendFIR, Classpath: classpath}
	compilation := wholeCompilationFingerprint(opts, []string{srcDir}, "", opts.tracker())
	if _, err := WriteFreshEntries(prepareLegacyEntryDir(cacheDir, kt, classpath),
		&Data{Version: 1, Files: map[string]*File{file: {}}},
		&CacheDepsFile{Version: 1, Approximation: ApproximationFIRWholeCompilation, CompilationFingerprint: compilation, Files: map[string]*CacheDepsEntry{file: {DepPaths: nil}}},
	); err != nil {
		t.Fatal(err)
	}
	return repo, srcDir, cacheDir, file
}

func writeTestJar(t *testing.T, dir, content string) string {
	t.Helper()
	jar := filepath.Join(dir, "dep.jar")
	if err := os.WriteFile(jar, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return jar
}

// InvokeCachedWithOptions must read the entry dir scoped by opts.Classpath.
func TestInvokeCachedUsesTheClasspathScopedLegacyDir(t *testing.T) {
	jarDir := t.TempDir()
	jar := writeTestJar(t, jarDir, "one")
	kt := gradlemodel.KotlinSettings{}
	repo, srcDir, _, _ := legacyClasspathRepo(t, kt, []string{jar})
	run := func(name string, cp []string) error {
		_, err := InvokeCachedWithOptions("", []string{srcDir}, repo, filepath.Join(repo, name), "", false, nil,
			InvocationOptions{Backend: BackendFIR, Kotlin: kt, Classpath: cp})
		return err
	}
	if err := run("a.json", []string{jar}); err != nil {
		t.Fatalf("entries seeded under the run's classpath must be served from cache: %v", err)
	}
	if err := run("b.json", nil); err == nil {
		t.Fatal("a classpath-free run reused classpath-scoped entries")
	}
	// A jar rewritten in place (new size) is a different classpath: miss.
	writeTestJar(t, jarDir, "replaced jar contents")
	if err := run("c.json", []string{jar}); err == nil {
		t.Fatal("a rewritten jar reused the old entries")
	}
}

func TestExpandStaleOraclePathsReadsTheClasspathScopedLegacyDir(t *testing.T) {
	jar := writeTestJar(t, t.TempDir(), "one")
	kt := gradlemodel.KotlinSettings{}
	_, _, cacheDir, a := legacyClasspathRepo(t, kt, []string{jar})
	if got := ExpandStaleOraclePaths(nil, cacheDir, []string{a}, nil, ExpansionScope{Classpath: []string{jar}}); len(got) != 0 {
		t.Fatalf("entries under the run's classpath must resolve (nothing seeded): %v", got)
	}
	if got := ExpandStaleOraclePaths(nil, cacheDir, []string{a}, nil, ExpansionScope{}); len(got) != 1 {
		t.Fatalf("an empty-classpath scope must not see the entries (file is seeded unresolved): %v", got)
	}
}

// Classpath-scoped directories share the pruning budget.
func TestLegacyClasspathScopesArePruned(t *testing.T) {
	tmp := t.TempDir()
	base, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for i := 0; i < maxLegacyScopes+3; i++ {
		jar := writeTestJar(t, t.TempDir(), strings.Repeat("x", i+1))
		dir := prepareLegacyEntryDir(base, gradlemodel.KotlinSettings{}, []string{jar})
		stamp := time.Now().Add(-3*time.Hour + time.Duration(i)*time.Minute)
		if err := os.Chtimes(dir, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, dir)
	}
	entries, err := os.ReadDir(filepath.Join(base, legacyScopesDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != maxLegacyScopes {
		t.Fatalf("kept %d classpath scopes, want %d", len(entries), maxLegacyScopes)
	}
	if _, err := os.Stat(dirs[len(dirs)-1]); err != nil {
		t.Fatalf("most recent scope was pruned: %v", err)
	}
}

// With a store in use the scope lives in the store keys: no empty legacy
// scope directory may be created to take an LRU slot.
func TestInvokeCachedWithStoreCreatesNoLegacyScopeDir(t *testing.T) {
	jar := writeTestJar(t, t.TempDir(), "one")
	repo := t.TempDir()
	srcDir := filepath.Join(repo, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTempFile(t, srcDir, "A.kt", "package demo\nfun a() = 1\n")
	cacheDir, err := CacheDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	s := store.New(filepath.Join(repo, "store"))
	// The run itself may fail (no JVM jar); only the directory side effect matters.
	_, _ = InvokeCachedWithOptions("", []string{srcDir}, repo, filepath.Join(repo, "o.json"), "", false, s,
		InvocationOptions{Backend: BackendFIR, Classpath: []string{jar}})
	if _, err := os.Stat(filepath.Join(cacheDir, legacyScopesDir)); err == nil {
		t.Fatal("a store run created a legacy scope directory")
	}
}

// The FIR compilation fingerprint independently catches a classpath-only
// change while the legacy entry dir is held fixed.
func TestFIRCompilationRefreshOnClasspathOnlyChange(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	a := writeTempFile(t, srcDir, "A.kt", "package demo\nfun a() = 1\n")
	fir := writeTestJar(t, filepath.Join(dir), "fir")
	cpDir := t.TempDir()
	jar := writeTestJar(t, cpDir, "one")
	opts := InvocationOptions{Backend: BackendFIR, Classpath: []string{jar}}
	before := wholeCompilationFingerprint(opts, []string{srcDir}, fir, opts.tracker())
	hits := []*CacheEntry{{FilePath: a, CompilationFingerprint: before}}
	if got := requestCompilationRefresh(opts, hits, nil, before, opts.tracker(), false); len(got) != 0 {
		t.Fatalf("unchanged classpath requested a refresh: %v", got)
	}
	writeTestJar(t, cpDir, "replaced jar contents")
	after := wholeCompilationFingerprint(opts, []string{srcDir}, fir, opts.tracker())
	if after == before {
		t.Fatal("classpath change left the compilation fingerprint unchanged")
	}
	if got := requestCompilationRefresh(opts, hits, nil, after, opts.tracker(), false); len(got) != 1 || got[0] != a {
		t.Fatalf("classpath-only change must refresh through %s, got %v", a, got)
	}
}
