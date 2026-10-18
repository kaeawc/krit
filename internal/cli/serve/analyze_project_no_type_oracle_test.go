package serve

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
)

// Issue #855: a request with NoTypeOracle must neither start nor consult the
// resident oracle daemon, while a plain request on the same daemon still does.
func TestBuildProjectInput_NoTypeOracleSkipsOracleDaemon(t *testing.T) {
	tmp := t.TempDir()
	seedJar(t, tmp, ".krit/krit-types.jar")
	state := newDaemonState(tmp)
	t.Cleanup(state.closeOracleDaemons)
	starter := &fakeOracleDaemonStarter{}
	state.oracleDaemonStarter = starter

	off, err := state.buildProjectInput(daemon.AnalyzeProjectArgs{NoFir: true, NoTypeOracle: true}, oracle.BackendKAA)
	if err != nil {
		t.Fatal(err)
	}
	if off.Host.OracleDaemon != nil || off.Args.OracleEnabled || !off.Args.NoTypeOracle {
		t.Fatalf("NoTypeOracle request must run without an oracle: daemon=%v enabled=%v noTypeOracle=%v",
			off.Host.OracleDaemon, off.Args.OracleEnabled, off.Args.NoTypeOracle)
	}
	if n := starter.calls.Load(); n != 0 {
		t.Fatalf("NoTypeOracle request started %d oracle daemon(s), want 0", n)
	}

	// --input-types must not re-enable the oracle either, as in-process.
	off, err = state.buildProjectInput(daemon.AnalyzeProjectArgs{NoFir: true, NoTypeOracle: true, InputTypesPath: "/x/types.json"}, oracle.BackendKAA)
	if err != nil {
		t.Fatal(err)
	}
	if off.Args.OracleEnabled {
		t.Fatal("NoTypeOracle with --input-types must keep the oracle disabled")
	}

	on, err := state.buildProjectInput(daemon.AnalyzeProjectArgs{NoFir: true}, oracle.BackendKAA)
	if err != nil {
		t.Fatal(err)
	}
	if on.Host.OracleDaemon == nil || !on.Args.OracleEnabled || on.Args.NoTypeOracle {
		t.Fatalf("plain request must keep using the oracle: daemon=%v enabled=%v", on.Host.OracleDaemon, on.Args.OracleEnabled)
	}
	if n := starter.calls.Load(); n != 1 {
		t.Fatalf("plain request started %d oracle daemon(s), want 1", n)
	}
}

// Warm state must not cross between oracle-backed and --no-type-oracle
// requests on one daemon: the findings bundle (and the caches keyed on the
// rule hash) are separate per mode, yet each mode still warms on its own.
func TestAnalyzeProject_NoTypeOracleDoesNotShareWarmBundle(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "A.kt", "package demo\n\nclass A {\n    fun a() {}\n}\n")

	call := func(noTypeOracle bool) daemon.AnalyzeProjectResult {
		t.Helper()
		var res daemon.AnalyzeProjectResult
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
			daemon.AnalyzeProjectArgs{NoFir: true, NoTypeOracle: noTypeOracle}, &res); err != nil {
			t.Fatalf("analyze (noTypeOracle=%v): %v", noTypeOracle, err)
		}
		return res
	}

	if r := call(false); r.Stats.FindingsBundleHit {
		t.Fatalf("cold oracle-mode call hit a bundle: %+v", r.Stats)
	}
	if r := call(false); !r.Stats.FindingsBundleHit {
		t.Fatalf("second oracle-mode call must warm-hit: %+v", r.Stats)
	}
	if r := call(true); r.Stats.FindingsBundleHit {
		t.Fatalf("no-type-oracle call was served the oracle-mode bundle: %+v", r.Stats)
	}
	if r := call(true); !r.Stats.FindingsBundleHit {
		t.Fatalf("second no-type-oracle call must warm-hit its own bundle: %+v", r.Stats)
	}
	if r := call(false); !r.Stats.FindingsBundleHit {
		t.Fatalf("oracle-mode bundle must survive an interleaved no-type-oracle call: %+v", r.Stats)
	}
}

// A --no-type-oracle request must not health-check (and so possibly
// restart) a cached oracle JVM: it promises not to touch the oracle at all.
func TestAnalyzeProject_NoTypeOracleSkipsOracleHealthCheck(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "A.kt", "package demo\n\nclass A\n")
	seedJar(t, state.root, ".krit/krit-types.jar")
	jar := filepath.Join(state.root, ".krit", "krit-types.jar")
	starter := &fakeOracleDaemonStarter{}
	state.oracleDaemonMu.Lock()
	state.oracleDaemonStarter = starter
	state.oracleDaemonByKey["dead"] = &oracleDaemonEntry{
		daemon:      &oracle.Daemon{}, // never started: a ping would fail and rebuild it
		jarPath:     jar,
		jarIdentity: oracle.JarIdentity(jar),
	}
	state.oracleDaemonMu.Unlock()

	var res daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{NoFir: true, NoTypeOracle: true}, &res); err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if n := starter.calls.Load(); n != 0 {
		t.Fatalf("no-type-oracle request rebuilt %d oracle daemon(s), want 0", n)
	}
}

// The strict-verify baseline of a --no-type-oracle request runs without the
// oracle too, so it neither starts one nor compares against oracle facts.
func TestRunStrictVerify_NoTypeOracleBaselineSkipsOracle(t *testing.T) {
	root := t.TempDir()
	seedJar(t, root, ".krit/krit-types.jar")
	writeKotlinFile(t, root, "X.kt", "package demo\n\nclass X\n")
	state := newDaemonState(root)
	t.Cleanup(state.closeOracleDaemons)
	state.strictVerify = true
	starter := &fakeOracleDaemonStarter{}
	state.oracleDaemonStarter = starter

	daemonCols := scanner.FindingColumns{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _ = state.runStrictVerify(ctx, daemon.AnalyzeProjectArgs{NoFir: true, NoTypeOracle: true}, &daemonCols)
	if n := starter.calls.Load(); n != 0 {
		t.Fatalf("no-type-oracle strict-verify baseline started %d oracle daemon(s), want 0", n)
	}
}

// Custom Kotlin rule jars run in the krit-types JVM: a --no-type-oracle
// request that carries them keeps a krit-types handle for the plugin RPCs,
// but the type oracle itself stays off, matching an in-process scan.
func TestBuildProjectInput_NoTypeOracleKeepsDaemonForCustomRuleJars(t *testing.T) {
	tmp := t.TempDir()
	seedJar(t, tmp, ".krit/krit-types.jar")
	state := newDaemonState(tmp)
	t.Cleanup(state.closeOracleDaemons)
	starter := &fakeOracleDaemonStarter{}
	state.oracleDaemonStarter = starter

	in, err := state.buildProjectInput(daemon.AnalyzeProjectArgs{
		NoFir:          true,
		NoTypeOracle:   true,
		CustomRuleJars: []string{filepath.Join(tmp, "rules.jar")},
	}, oracle.BackendFIR)
	if err != nil {
		t.Fatal(err)
	}
	if in.Host.OracleDaemon == nil {
		t.Fatal("custom rule jars need a krit-types daemon handle even with --no-type-oracle")
	}
	if in.Args.OracleEnabled {
		t.Fatal("the type oracle must stay disabled")
	}
	if len(starter.backends) != 1 || starter.backends[0] != oracle.BackendKAA {
		t.Fatalf("started backends %v, want [%s]", starter.backends, oracle.BackendKAA)
	}
}
