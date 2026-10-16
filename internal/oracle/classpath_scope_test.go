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

// A compile with settings must not go through the resident daemon registry,
// which is keyed on neither the language level nor the flags.
func TestKotlinSettingsForceSettingsScopedOneShot(t *testing.T) {
	opts := InvocationOptions{Backend: BackendKAA}
	if use, _ := oneShotMissAnalysisReason(4, 1, opts); use {
		t.Skip("baseline already one-shot")
	}
	opts.Kotlin = gradlemodel.KotlinSettings{LanguageVersion: "1.9"}
	for _, backend := range []Backend{BackendKAA, BackendFIR} {
		opts.Backend = backend
		if use, reason := oneShotMissAnalysisReason(4, 1, opts); !use || reason == "" {
			t.Errorf("%s: settings did not force the settings-scoped one-shot (%v %q)", backend, use, reason)
		}
	}
}

func TestCompileContextArgsCarryKotlinSettings(t *testing.T) {
	opts := InvocationOptions{
		Backend: BackendKAA, Classpath: []string{"/a.jar"}, JvmTarget: "17",
		Kotlin: gradlemodel.KotlinSettings{LanguageVersion: "1.9", APIVersion: "1.8", OptIns: []string{"x.Y"}, Flags: []string{"-Xcontext-parameters"}},
	}
	got := compileContextArgs(opts)
	want := []string{
		"--classpath", "/a.jar",
		"--kotlin-arg", "-language-version=1.9", "--kotlin-arg", "-api-version=1.8",
		"--kotlin-arg", "-opt-in=x.Y", "--kotlin-arg", "-Xcontext-parameters",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v want %v", got, want)
	}
	opts.Backend = BackendFIR
	if got := compileContextArgs(opts); !strings.Contains(strings.Join(got, " "), "--jvm-target 17") || !strings.Contains(strings.Join(got, " "), "--kotlin-arg -language-version=1.9") {
		t.Fatalf("fir args = %v", got)
	}
	if got := compileContextArgs(InvocationOptions{Backend: BackendFIR}); len(got) != 0 {
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
