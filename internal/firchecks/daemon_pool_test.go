package firchecks

import (
	"path/filepath"
	"strings"
	"testing"
)

// With KRIT_FIR_DAEMON_POOL set, check daemons are keyed by the pool slot, so
// projects with different sources, classpaths and targets share one daemon; the
// requests retarget its session. Oracle daemons keep their per-project key.
func TestFirDaemonPoolSharesTheCheckDaemon(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "krit-fir.jar")
	appSources := []string{filepath.Join(dir, "app", "src")}
	libSources := []string{filepath.Join(dir, "lib", "src")}
	appTarget := CompileTarget{JvmTarget: "17"}
	libTarget := CompileTarget{JvmTarget: "21"}

	unpooledApp := firRegistryKeyFor(firCheckRole, jar, appSources, []string{"a.jar"}, appTarget)
	unpooledLib := firRegistryKeyFor(firCheckRole, jar, libSources, []string{"b.jar"}, libTarget)
	if unpooledApp == unpooledLib {
		t.Fatal("without a pool, projects must not share a check daemon")
	}

	t.Setenv(FirDaemonPoolEnv, "build-1234-0")
	app := firRegistryKeyFor(firCheckRole, jar, appSources, []string{"a.jar"}, appTarget)
	lib := firRegistryKeyFor(firCheckRole, jar, libSources, []string{"b.jar"}, libTarget)
	if app != lib {
		t.Fatalf("pooled check daemons differ: %s vs %s", app, lib)
	}
	if !strings.HasPrefix(app, firRegistryFamilyPrefix(firCheckRole, jar, libSources, nil)+"@") {
		t.Fatalf("pooled key %s does not extend its family prefix", app)
	}
	if oracleApp, oracleLib := firRegistryKeyFor("", jar, appSources, nil), firRegistryKeyFor("", jar, libSources, nil); oracleApp == oracleLib {
		t.Fatal("oracle requests carry no sources, so oracle daemons must stay per project")
	}

	t.Setenv(FirDaemonPoolEnv, "build-1234-1")
	if other := firRegistryKeyFor(firCheckRole, jar, appSources, []string{"a.jar"}, appTarget); other == app {
		t.Fatal("pool slots must not share a daemon: concurrent runs would find it busy")
	}
}
