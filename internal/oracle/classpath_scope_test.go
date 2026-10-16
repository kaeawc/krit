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

// A settings-only change must change the store scope of either backend: the
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
	for _, backend := range []Backend{BackendFIR, BackendKAA} {
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
// is keyed on them instead. A KAA run with settings keeps the daemon path.
func TestKotlinSettingsKeepTheDaemonPathForKAA(t *testing.T) {
	t.Setenv("KRIT_DAEMON_CACHE", "on")
	opts := InvocationOptions{Backend: BackendKAA, Kotlin: gradlemodel.KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0"}}
	if use, reason := oneShotMissAnalysisReason(1, 1, opts); use {
		t.Fatalf("settings forced a one-shot compile (%q); the daemon must serve them", reason)
	}
	// The pre-existing FIR JVM-target rule is unchanged.
	opts.Backend, opts.JvmTarget = BackendFIR, "17"
	if use, _ := oneShotMissAnalysisReason(1, 1, opts); !use {
		t.Fatal("FIR with a JVM target still needs the target-scoped one-shot")
	}
}

// A daemon launched for one set of Kotlin settings is never reused for another,
// and the settings are passed at launch.
func TestDaemonRegistryIsKeyedOnKotlinSettings(t *testing.T) {
	jar, dirs := "/jars/krit-types.jar", []string{"/src"}
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
		Backend: BackendKAA, Classpath: []string{"/a.jar"}, JvmTarget: "17",
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
	opts.Backend = BackendFIR
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
