package firchecks

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
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
	t.Setenv("JAVA_HOME", "/jdk/21")
	app := firRegistryKeyFor(firCheckRole, jar, appSources, []string{"a.jar"}, appTarget)
	lib := firRegistryKeyFor(firCheckRole, jar, libSources, []string{"b.jar"}, appTarget)
	if app != lib {
		t.Fatalf("pooled check daemons for the same settings differ: %s vs %s", app, lib)
	}
	// krit-fir keeps a session's JVM target, Kotlin args and compiler plugins when a
	// request leaves them empty, so projects that differ in them must not share.
	if other := firRegistryKeyFor(firCheckRole, jar, libSources, []string{"b.jar"}, libTarget); other == app {
		t.Fatal("a different JVM target must not share the pooled daemon")
	}
	withPlugin := CompileTarget{JvmTarget: "17", Kotlin: gradlemodel.KotlinSettings{OptIns: []string{"kotlin.RequiresOptIn"}}}
	if other := firRegistryKeyFor(firCheckRole, jar, libSources, nil, withPlugin); other == app {
		t.Fatal("different Kotlin settings must not share the pooled daemon")
	}
	t.Setenv("JAVA_HOME", "/jdk/25")
	if other := firRegistryKeyFor(firCheckRole, jar, appSources, []string{"a.jar"}, appTarget); other == app {
		t.Fatal("a different JDK must not share the pooled daemon")
	}
	t.Setenv("JAVA_HOME", "/jdk/21")
	if !strings.HasPrefix(app, firRegistryFamilyPrefix(firCheckRole, jar, libSources, nil, appTarget)+"@") {
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
