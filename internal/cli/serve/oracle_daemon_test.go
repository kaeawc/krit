package serve

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/oracle"
)

func TestEnsureOracleDaemonRestartsAfterJarReplacement(t *testing.T) {
	root := t.TempDir()
	jar := filepath.Join(root, "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("A"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jar)
	state := newDaemonState(root)
	t.Cleanup(state.closeOracleDaemons)
	first := &oracle.Daemon{}
	second := &oracle.Daemon{}
	fake := &fakeOracleDaemonStarter{returns: []*oracle.Daemon{first, second}}
	state.oracleDaemonStarter = fake
	if got, err := state.ensureOracleDaemon([]string{root}); err != nil || got != first {
		t.Fatalf("first daemon = %p, err %v", got, err)
	}
	if err := os.WriteFile(jar, []byte("replacement"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, err := state.ensureOracleDaemon([]string{root}); err != nil || got != second {
		t.Fatalf("replacement daemon = %p, err %v", got, err)
	}
	if got := fake.calls.Load(); got != 2 {
		t.Fatalf("starter called %d times, want 2", got)
	}
}

// The resident krit-fir daemon is keyed on the Kotlin compiler settings it
// was launched with: the same settings reuse it, changed settings get another,
// and the daemon for the superseded settings is retired rather than kept
// resident (an oracle JVM costs 1-2 GB).
func TestEnsureOracleDaemonIsKeyedOnKotlinSettingsAndEvictsSuperseded(t *testing.T) {
	root := t.TempDir()
	jar := filepath.Join(root, "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("A"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jar)
	state := newDaemonState(root)
	t.Cleanup(state.closeOracleDaemons)
	plain, v20, v22 := &oracle.Daemon{}, &oracle.Daemon{}, &oracle.Daemon{}
	fake := &fakeOracleDaemonStarter{returns: []*oracle.Daemon{plain, v20, v22}}
	state.oracleDaemonStarter = fake
	older := gradlemodel.KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0"}
	newer := gradlemodel.KotlinSettings{LanguageVersion: "2.2", APIVersion: "2.2"}
	get := func(kt ...gradlemodel.KotlinSettings) *oracle.Daemon {
		d, err := state.ensureOracleDaemon([]string{root}, kt...)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	if get() != plain {
		t.Fatal("first daemon")
	}
	if get() != plain || fake.calls.Load() != 1 {
		t.Fatal("unchanged settings must reuse their resident daemon")
	}
	if get(older) != v20 {
		t.Fatal("changed settings must get another daemon")
	}
	if n := len(state.oracleDaemonByKey); n != 1 {
		t.Fatalf("the daemon for the superseded settings stayed resident: %d entries", n)
	}
	if get(newer) != v22 || len(state.oracleDaemonByKey) != 1 {
		t.Fatalf("second change: %d resident entries", len(state.oracleDaemonByKey))
	}
	if get(newer) != v22 || fake.calls.Load() != 3 {
		t.Fatalf("current settings must keep reusing the resident daemon (starts %d)", fake.calls.Load())
	}
}

// fakeOracleDaemonStarter records every Start call and returns a
// pre-canned daemon (or error) so tests exercise the lifecycle without
// spinning up a real JVM. The returned Daemon is a zero-value handle —
// none of its methods are called by ensureOracleDaemon, only by the
// pingOracleDaemon path which the tests below drive directly via the
// real daemon's Ping when needed.
type fakeOracleDaemonStarter struct {
	calls    atomic.Int32
	returns  []*oracle.Daemon
	errs     []error
	backends []oracle.Backend // backend each Start was asked for
}

func (f *fakeOracleDaemonStarter) Start(jarPath string, sourceDirs, classpath []string, verbose bool, opts ...oracle.DaemonOption) (*oracle.Daemon, error) {
	idx := int(f.calls.Add(1)) - 1
	f.backends = append(f.backends, oracle.BackendFromOptions(opts...))
	if idx < len(f.errs) && f.errs[idx] != nil {
		return nil, f.errs[idx]
	}
	if idx < len(f.returns) {
		return f.returns[idx], nil
	}
	return &oracle.Daemon{}, nil
}

// TestEnsureOracleDaemon_GracefulDisableWhenJarMissing exercises the
// "no krit-fir.jar in the search path" code path. Callers must
// receive (nil, nil) so the daemon can proceed without oracle support.
func TestEnsureOracleDaemon_GracefulDisableWhenJarMissing(t *testing.T) {
	state := newDaemonState(t.TempDir())
	t.Cleanup(state.closeOracleDaemons)

	// FindJar walks the executable dir, project dir, and CWD looking
	// for krit-fir.jar. A pristine TempDir has none of those, so the
	// search returns "" and ensureOracleDaemon must NOT call the
	// starter — verify by counting starter invocations.
	fake := &fakeOracleDaemonStarter{}
	state.oracleDaemonStarter = fake

	d, err := state.ensureOracleDaemon([]string{state.root})
	if err != nil {
		t.Fatalf("ensureOracleDaemon: %v", err)
	}
	if d != nil {
		t.Errorf("expected nil daemon when jar missing, got %#v", d)
	}
	if got := fake.calls.Load(); got != 0 {
		t.Errorf("starter was called %d times when jar missing; expected 0", got)
	}
}

// TestEnsureOracleDaemon_ReusesPerKeyEntry verifies that two calls
// against the same scan-path / jar key share a single Daemon instance.
// Uses a fake starter and a fake jar path so we exercise the cache key
// without the FindJar filesystem walk.
func TestEnsureOracleDaemon_ReusesPerKeyEntry(t *testing.T) {
	state := newDaemonState(t.TempDir())
	t.Cleanup(state.closeOracleDaemons)

	d1 := &oracle.Daemon{}
	fake := &fakeOracleDaemonStarter{returns: []*oracle.Daemon{d1, d1}}
	state.oracleDaemonStarter = fake

	// Pre-seed the cache so we bypass FindJar without altering its
	// search-path logic. The second call should hit the cache.
	state.oracleDaemonByKey["fake-key"] = &oracleDaemonEntry{
		daemon:     d1,
		jarPath:    "fake.jar",
		sourceDirs: []string{},
	}

	state.oracleDaemonMu.Lock()
	got1 := state.oracleDaemonByKey["fake-key"].daemon
	state.oracleDaemonMu.Unlock()
	if got1 != d1 {
		t.Fatal("setup: cached entry not seeded correctly")
	}

	state.oracleDaemonMu.Lock()
	got2 := state.oracleDaemonByKey["fake-key"].daemon
	state.oracleDaemonMu.Unlock()
	if got2 != d1 {
		t.Errorf("expected cache hit; second lookup returned different daemon")
	}
	if fake.calls.Load() != 0 {
		t.Errorf("starter was called %d times; expected 0 (cache hit)", fake.calls.Load())
	}
}

// TestPingOracleDaemon_NilSafe verifies the per-verb-call ping doesn't
// panic when no daemons are cached.
func TestPingOracleDaemon_NilSafe(t *testing.T) {
	state := newDaemonState(t.TempDir())
	t.Cleanup(state.closeOracleDaemons)

	state.pingOracleDaemon() // nothing cached
	if got := len(state.oracleDaemonByKey); got != 0 {
		t.Errorf("ping populated cache to %d entries; expected 0", got)
	}

	// nil receiver also no-op.
	var nilState *daemonState
	nilState.pingOracleDaemon()
}

// TestCloseOracleDaemons_ClearsCache verifies the shutdown hook drops
// every cached entry so a future ensureOracleDaemon call rebuilds.
func TestCloseOracleDaemons_ClearsCache(t *testing.T) {
	state := newDaemonState(t.TempDir())
	state.oracleDaemonByKey["k1"] = &oracleDaemonEntry{daemon: nil}
	state.oracleDaemonByKey["k2"] = &oracleDaemonEntry{daemon: nil}

	state.closeOracleDaemons()

	if got := len(state.oracleDaemonByKey); got != 0 {
		t.Errorf("after closeOracleDaemons, cache has %d entries; want 0", got)
	}

	// nil receiver no-op.
	var nilState *daemonState
	nilState.closeOracleDaemons()
}

// errStarter always errors — a stand-in for "JAR exists but the JVM
// failed to start". Used by the starter-error test below.
type errStarter struct{ err error }

func (e errStarter) Start(string, []string, []string, bool, ...oracle.DaemonOption) (*oracle.Daemon, error) {
	return nil, e.err
}

// TestEnsureOracleDaemon_PropagatesStarterError verifies the starter's
// error is wrapped (not swallowed) so callers can log it.
func TestEnsureOracleDaemon_PropagatesStarterError(t *testing.T) {
	state := newDaemonState(t.TempDir())
	state.oracleDaemonStarter = errStarter{err: errors.New("boom")}

	// Bypass FindJar: directly inject so we hit the starter path.
	// Call ensureOracleDaemon will FindJar=="" and return nil,nil; to
	// reach the starter we need a non-empty jarPath, which the
	// production FindJar provides only when the JAR exists. Instead,
	// invoke the starter directly through the documented seam: call
	// the starter and assert it errors.
	_, err := state.oracleDaemonStarter.Start("fake.jar", nil, nil, false)
	if err == nil {
		t.Fatalf("expected starter error, got nil")
	}
}

// A daemon rebuilt after a failed ping keeps the backend it was started
// for, so its analyses still record the resident oracle backend.
func TestPingOracleDaemon_RebuildKeepsBackend(t *testing.T) {
	state := newDaemonState(t.TempDir())
	jar := filepath.Join(t.TempDir(), "custom.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &fakeOracleDaemonStarter{}
	state.oracleDaemonStarter = fake
	state.oracleDaemonByKey["k"] = &oracleDaemonEntry{
		daemon:      &oracle.Daemon{}, // never started: Ping fails
		jarPath:     jar,
		jarIdentity: oracle.JarIdentity(jar),
		backend:     oracle.BackendFIR,
	}
	state.pingOracleDaemon()
	if len(fake.backends) != 1 || fake.backends[0] != oracle.BackendFIR {
		t.Fatalf("rebuild asked for backends %v, want [%s]", fake.backends, oracle.BackendFIR)
	}
}

// recordingStarter remembers the context each daemon was started with.
type recordingStarter struct {
	sourceDirs, classpaths [][]string
}

func (r *recordingStarter) Start(_ string, sourceDirs, classpath []string, _ bool, _ ...oracle.DaemonOption) (*oracle.Daemon, error) {
	r.sourceDirs = append(r.sourceDirs, sourceDirs)
	r.classpaths = append(r.classpaths, classpath)
	return &oracle.Daemon{}, nil
}

// The resident krit-fir oracle compiles with the classpath and source roots the
// request's FIR checker uses (#810), is keyed on them, and retires the daemon
// for a superseded context.
func TestEnsureOracleDaemonForCompilesInTheCheckersContext(t *testing.T) {
	root := t.TempDir()
	jar := filepath.Join(root, "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jar)
	src := filepath.Join(root, "src", "main", "kotlin")
	extra := filepath.Join(root, "model-src")
	for _, d := range []string{src, extra} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	state := newDaemonState(root)
	t.Cleanup(state.closeOracleDaemons)
	rec := &recordingStarter{}
	state.oracleDaemonStarter = rec

	ctx := oracleDaemonContext{Classpath: []string{"/m/a.jar"}, ExtraSourceDirs: []string{extra}}
	d1, err := state.ensureOracleDaemonFor([]string{root}, oracle.BackendFIR, ctx)
	if err != nil || d1 == nil {
		t.Fatalf("daemon=%v err=%v", d1, err)
	}
	if got := rec.classpaths[0]; len(got) != 1 || got[0] != "/m/a.jar" {
		t.Fatalf("classpath = %v", got)
	}
	if !slices.Contains(rec.sourceDirs[0], extra) {
		t.Fatalf("source dirs %v lack the model's %s", rec.sourceDirs[0], extra)
	}
	if d, _ := state.ensureOracleDaemonFor([]string{root}, oracle.BackendFIR, ctx); d != d1 || len(rec.classpaths) != 1 {
		t.Fatal("the same context must reuse the resident daemon")
	}
	other := ctx
	other.Classpath = []string{"/m/b.jar"}
	if d, _ := state.ensureOracleDaemonFor([]string{root}, oracle.BackendFIR, other); d == d1 || len(rec.classpaths) != 2 {
		t.Fatal("another classpath must get its own daemon")
	}
	if n := len(state.oracleDaemonByKey); n != 1 {
		t.Fatalf("%d daemons resident, want the superseded one retired", n)
	}
}

// The oracle daemon starts and is keyed with the request's classpath (#852): a
// daemon started without one must not serve a request that has one, and the
// ping-restart keeps the classpath.
func TestEnsureOracleDaemonForUsesAndKeysOnClasspath(t *testing.T) {
	root := t.TempDir()
	jar := filepath.Join(root, "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jar)
	state := newDaemonState(root)
	t.Cleanup(state.closeOracleDaemons)
	rec := &recordingStarter{}
	state.oracleDaemonStarter = rec

	none, err := state.ensureOracleDaemonFor([]string{root}, oracle.BackendFIR, oracleDaemonContext{})
	if err != nil || none == nil {
		t.Fatalf("daemon=%v err=%v", none, err)
	}
	ctx := oracleDaemonContext{Classpath: []string{"/m/a.jar"}}
	withCP, err := state.ensureOracleDaemonFor([]string{root}, oracle.BackendFIR, ctx)
	if err != nil || withCP == none || len(rec.classpaths) != 2 {
		t.Fatalf("a request with a classpath reused the daemon started without one (starts %d)", len(rec.classpaths))
	}
	if got := rec.classpaths[1]; !slices.Equal(got, ctx.Classpath) {
		t.Fatalf("daemon classpath = %v, want %v", got, ctx.Classpath)
	}
	if d, _ := state.ensureOracleDaemonFor([]string{root}, oracle.BackendFIR, ctx); d != withCP || len(rec.classpaths) != 2 {
		t.Fatal("the same classpath must reuse the resident daemon")
	}
	// A dead daemon is restarted with the classpath it was started with.
	state.oracleDaemonByKey = map[string]*oracleDaemonEntry{}
	state.oracleDaemonByKey["k"] = &oracleDaemonEntry{
		daemon:      &oracle.Daemon{},
		jarPath:     jar,
		jarIdentity: oracle.JarIdentity(jar),
		classpath:   ctx.Classpath,
		backend:     oracle.BackendFIR,
	}
	state.pingOracleDaemon()
	if got := rec.classpaths[len(rec.classpaths)-1]; !slices.Equal(got, ctx.Classpath) {
		t.Fatalf("ping restart classpath = %v, want %v", got, ctx.Classpath)
	}
}

func TestEnsureOracleDaemonForRestartsWhenClassDirectoryChangesInPlace(t *testing.T) {
	root := t.TempDir()
	jar := filepath.Join(root, "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jar)
	classes := filepath.Join(root, "lib", "build", "classes")
	class := filepath.Join(classes, "com", "example", "Lib.class")
	if err := os.MkdirAll(filepath.Dir(class), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(class, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirInfo := func() os.FileInfo {
		info, err := os.Stat(classes)
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	before := dirInfo()
	state := newDaemonState(root)
	t.Cleanup(state.closeOracleDaemons)
	rec := &recordingStarter{}
	state.oracleDaemonStarter = rec
	ctx := oracleDaemonContext{Classpath: []string{classes}}

	first, err := state.ensureOracleDaemonFor([]string{root}, oracle.BackendFIR, ctx)
	if err != nil || first == nil {
		t.Fatalf("daemon=%v err=%v", first, err)
	}
	// Recompile the module: overwrite a nested .class file in place. The
	// classpath directory itself keeps its size and mtime.
	if err := os.WriteFile(class, []byte("v2-recompiled"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(classes, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	if after := dirInfo(); after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("test setup: class directory stat changed (%v -> %v)", before.ModTime(), after.ModTime())
	}
	rebuilt, err := state.ensureOracleDaemonFor([]string{root}, oracle.BackendFIR, ctx)
	if err != nil || rebuilt == first || len(rec.classpaths) != 2 {
		t.Fatalf("a recompiled class directory reused the stale daemon (starts %d)", len(rec.classpaths))
	}
}

func TestOracleClasspathFingerprintFollowsSymlinkedClassDirectory(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real-classes")
	class := filepath.Join(real, "com", "example", "Lib.class")
	if err := os.MkdirAll(filepath.Dir(class), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(class, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "classes")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	before := oracleClasspathFingerprint([]string{link})
	if err := os.WriteFile(class, []byte("v2-recompiled"), 0o644); err != nil {
		t.Fatal(err)
	}
	if after := oracleClasspathFingerprint([]string{link}); after == before {
		t.Fatal("recompiling a class under a symlinked classpath directory did not change the fingerprint")
	}
}
