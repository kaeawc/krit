package modshard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/gradlemodel"
)

func settingsUnit(id, kind string, args, plugins []string, sources ...string) Unit {
	return Unit{Spec: firchecks.ModuleSpec{
		ID: id, Kind: kind, Platform: "jvm", JVMTarget: "17",
		SourceRoots: []string{"/r/" + id + "/src"}, Classpath: []string{"/cp/" + id + ".jar", "/cp/shared.jar"},
		CompilerArgs: args, CompilerPluginClasspath: plugins,
		ContextSourceRoots: []string{"/r/:core:main/src", "/r/:util:main/src"},
	}, Files: len(sources), Sources: sources}
}

func TestGroupBySettingsKeysOnArgsPluginsOptionsAndTarget(t *testing.T) {
	compose := []string{"/p/compose-compiler.jar"}
	a := settingsUnit(":a:main", "main", []string{"-opt-in=x.Y"}, compose, "/r/a/A.kt")
	b := settingsUnit(":b:main", "main", []string{"-opt-in=x.Y"}, compose, "/r/b/B.kt")
	c := settingsUnit(":c:main", "main", nil, nil, "/r/c/C.kt")
	d := settingsUnit(":d:main", "main", []string{"-opt-in=x.Y"}, nil, "/r/d/D.kt")
	e := settingsUnit(":e:main", "main", []string{"-opt-in=x.Y"}, compose, "/r/e/E.kt")
	e.Spec.JVMTarget = "21"
	kmp := settingsUnit(":k:main", "main", nil, nil)
	kmp.Spec.Fragments = []firchecks.ModuleFragment{{Name: "common"}}
	dropped := settingsUnit(":x:main", "main", nil, nil)
	dropped.ContextDropped = true

	groups, left := groupBySettings([]Unit{a, c, b, d, e, kmp, dropped})
	var got [][]string
	for _, g := range groups {
		var ids []string
		for _, u := range g.units {
			ids = append(ids, u.Spec.ID)
		}
		got = append(got, ids)
	}
	want := [][]string{{":a:main", ":b:main"}, {":c:main"}, {":d:main"}, {":e:main"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groups = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(left, []string{":k:main", ":x:main"}) {
		t.Fatalf("left = %v, want the multiplatform and context-limited units", left)
	}
	if label := groups[0].label(); !strings.Contains(label, ":a:main, :b:main") || !strings.Contains(label, "-opt-in=x.Y plugins=compose-compiler.jar") {
		t.Fatalf("label = %q", label)
	}
	if label := groups[1].label(); !strings.Contains(label, "(compiler defaults)") {
		t.Fatalf("label = %q", label)
	}
}

func TestAppliedKeyIgnoresOrderAndTheEmbeddableCompiler(t *testing.T) {
	merged := appliedKey([]string{"-opt-in=b", "-opt-in=a"}, []string{"/p/z.jar", "/p/a.jar"}, nil)
	group := appliedKey([]string{"-opt-in=a", "-opt-in=b"}, []string{"/p/a.jar", "/p/kotlin-compiler-embeddable-2.4.21.jar", "/p/z.jar"}, nil)
	if merged != group {
		t.Fatalf("keys differ:\n%q\n%q", merged, group)
	}
	if appliedKey([]string{"-opt-in=a"}, nil, nil) == appliedKey(nil, nil, nil) {
		t.Fatal("an opt-in must change the key")
	}
	if appliedKey(nil, nil, []string{"plugin:p:k=1"}) == appliedKey(nil, nil, []string{"plugin:p:k=2"}) {
		t.Fatal("a plugin option value must change the key")
	}
}

func TestGroupUnitMergesMembersAndDropsOwnRootsFromContext(t *testing.T) {
	core := settingsUnit(":core:main", "main", []string{"-Xfoo"}, nil, "/r/core/Core.kt")
	app := settingsUnit(":app:main", "main", []string{"-Xfoo"}, nil, "/r/app/App.kt")
	test := settingsUnit(":app:test", "test", []string{"-Xfoo"}, nil, "/r/app/AppTest.kt", "/r/app/App.kt")
	test.Spec.Friends = []string{"/cp/:app:main.jar"}
	groups, _ := groupBySettings([]Unit{core, app, test})
	if len(groups) != 1 {
		t.Fatalf("got %d groups", len(groups))
	}
	u := groupUnit(1, groups[0])
	if u.Spec.ID != ":krit-settings-1:group" || projectOf(u.Spec.ID) != ":krit-settings-1" {
		t.Fatalf("ID = %q", u.Spec.ID)
	}
	if u.Spec.Kind != "main" || !reflect.DeepEqual(u.Spec.CompilerArgs, []string{"-Xfoo"}) || u.Spec.JVMTarget != "17" {
		t.Fatalf("spec = %+v", u.Spec)
	}
	if want := []string{"/r/:core:main/src", "/r/:app:main/src", "/r/:app:test/src"}; !reflect.DeepEqual(u.Spec.SourceRoots, want) {
		t.Fatalf("SourceRoots = %v, want %v", u.Spec.SourceRoots, want)
	}
	// :core:main is compiled as a member, so it is not context too.
	if want := []string{"/r/:util:main/src"}; !reflect.DeepEqual(u.Spec.ContextSourceRoots, want) {
		t.Fatalf("ContextSourceRoots = %v, want %v", u.Spec.ContextSourceRoots, want)
	}
	if want := []string{"/cp/:core:main.jar", "/cp/shared.jar", "/cp/:app:main.jar", "/cp/:app:test.jar"}; !reflect.DeepEqual(u.Spec.Classpath, want) {
		t.Fatalf("Classpath = %v, want %v", u.Spec.Classpath, want)
	}
	if !reflect.DeepEqual(u.Spec.Friends, []string{"/cp/:app:main.jar"}) {
		t.Fatalf("Friends = %v", u.Spec.Friends)
	}
	if want := []string{"/r/app/App.kt", "/r/app/AppTest.kt", "/r/core/Core.kt"}; !reflect.DeepEqual(u.Sources, want) {
		t.Fatalf("Sources = %v, want %v", u.Sources, want)
	}
}

func TestWithoutSharedSourcesDropsGroupsThatShareAFile(t *testing.T) {
	a := settingsUnit(":a:main", "main", []string{"-Xa"}, nil, "/r/shared/S.kt")
	b := settingsUnit(":b:main", "main", []string{"-Xb"}, nil, "/r/shared/S.kt")
	c := settingsUnit(":c:main", "main", []string{"-Xc"}, nil, "/r/c/C.kt")
	groups, _ := groupBySettings([]Unit{a, b, c})
	var logged []string
	kept := withoutSharedSources(groups, func(format string, args ...any) { logged = append(logged, format) })
	if len(kept) != 1 || kept[0].units[0].Spec.ID != ":c:main" || len(logged) != 2 {
		t.Fatalf("kept %d groups, logged %d lines", len(kept), len(logged))
	}
}

// writeSettingsModel writes a model with one main source set per project,
// each with its own source root and Kotlin free compiler args, and returns
// the loaded model and the project directories.
func writeSettingsModel(t *testing.T, projects map[string][]string, shared bool) (*gradlemodel.Model, map[string]string) {
	t.Helper()
	root := t.TempDir()
	dirs := map[string]string{}
	var specs []string
	sharedRoot := filepath.Join(root, "shared", "src")
	if shared {
		if err := os.MkdirAll(sharedRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sharedRoot, "S.kt"), []byte("package s\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	names := make([]string, 0, len(projects))
	for name := range projects {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		dir := filepath.Join(root, name)
		src := filepath.Join(dir, "src", "main", "kotlin")
		if err := os.MkdirAll(src, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(src, strings.ToUpper(name)+".kt"), []byte("package "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		dirs[name] = dir
		roots := []string{src}
		if shared {
			roots = append(roots, sharedRoot)
		}
		rootsJSON, _ := json.Marshal(roots)
		argsJSON, _ := json.Marshal(projects[name])
		specs = append(specs, fmt.Sprintf(`{"path":":%s","dir":%q,"sourceSets":[{"name":"main","kind":"main","platform":"jvm","sourceDirs":%s,"kotlin":{"optIn":[],"freeCompilerArgs":%s}}]}`, name, dir, rootsJSON, argsJSON))
	}
	modelDir := filepath.Join(root, ".krit", "gradle-model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "m.json"), []byte(`{"schema":2,"projects":[`+strings.Join(specs, ",")+`]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	model, _, err := gradlemodel.Load(modelDir)
	if err != nil {
		t.Fatal(err)
	}
	return model, dirs
}

// A scan of one module yields one settings group, which still differs from
// the merged session when another module of the model adds a flag the
// merged session applies.
func TestPlanSettingsSessionsComparesASingleScannedGroup(t *testing.T) {
	model, dirs := writeSettingsModel(t, map[string][]string{"a": {"-Xcontext-sensitive-resolution"}, "b": nil}, false)
	check := &checkContext{kotlinArgs: []string{"-Xcontext-sensitive-resolution"}}
	units, split := planSettingsSessions(Input{ScanPaths: []string{dirs["b"]}}, model, filepath.Dir(dirs["b"]), check, func(string, ...any) {})
	if len(units) != 1 || split.Sessions != 2 || !strings.Contains(split.Labels[0], ":b:main") {
		t.Fatalf("units=%d split=%+v; want one session for :b:main", len(units), split)
	}
	units, _ = planSettingsSessions(Input{ScanPaths: []string{dirs["a"]}}, model, filepath.Dir(dirs["a"]), check, func(string, ...any) {})
	if len(units) != 0 {
		t.Fatalf("a group the merged session compiles exactly needs no session, got %d", len(units))
	}
}

// A group that shares a source file with a group the merged session already
// compiles exactly keeps the merged verdicts.
func TestPlanSettingsSessionsKeepsFilesSharedWithMergedCoveredGroups(t *testing.T) {
	model, dirs := writeSettingsModel(t, map[string][]string{"a": {"-Xcontext-sensitive-resolution"}, "b": nil}, true)
	check := &checkContext{kotlinArgs: []string{"-Xcontext-sensitive-resolution"}}
	root := filepath.Dir(dirs["a"])
	if units, _ := planSettingsSessions(Input{ScanPaths: []string{root}}, model, root, check, func(string, ...any) {}); len(units) != 0 {
		t.Fatalf("got %d sessions; :b:main shares S.kt with :a:main, which the merged session covers", len(units))
	}
}

func TestJvmTargetDiffers(t *testing.T) {
	for _, c := range []struct {
		group, merged string
		want          bool
	}{
		{"", "17", false}, // no declared target compiles at the session's
		{"17", "17", false},
		{"1.8", "8", false},
		{"11", "17", true},
		{"11", "", true}, // the merged session used krit-fir's default
	} {
		if got := jvmTargetDiffers(c.group, c.merged); got != c.want {
			t.Errorf("jvmTargetDiffers(%q, %q) = %v, want %v", c.group, c.merged, got, c.want)
		}
	}
}

// A group that differs from the merged session only in JVM target keeps the
// merged verdicts and is logged; KRIT_FIR_SPLIT_JVM_TARGET=1 gives it a
// session at its own target (#1648).
func TestPlanSettingsSessionsSplitsOnJvmTargetOnlyWhenEnabled(t *testing.T) {
	model, dirs := writeSettingsModel(t, map[string][]string{"a": nil, "b": nil}, false)
	for i := range model.Projects {
		model.Projects[i].SourceSets[0].JvmTarget = map[string]string{":a": "17", ":b": "11"}[model.Projects[i].Path]
	}
	root := filepath.Dir(dirs["a"])
	check := &checkContext{jvmTarget: "17"}
	var log strings.Builder
	logf := func(format string, args ...any) { fmt.Fprintf(&log, format, args...) }

	t.Setenv(SplitJVMTargetEnv, "")
	units, split := planSettingsSessions(Input{ScanPaths: []string{root}}, model, root, check, logf)
	if len(units) != 0 || split.Sessions != 1 {
		t.Fatalf("units=%d split=%+v; want no session of its own", len(units), split)
	}
	if want := "verbose: FIR settings: 1 settings groups differ from the merged session only in JVM target (merged: 17) and keep its verdicts; KRIT_FIR_SPLIT_JVM_TARGET=1 compiles each on its own: :b:main [(compiler defaults) jvm=11]"; !strings.Contains(log.String(), want) {
		t.Fatalf("log lacks %q:\n%s", want, log.String())
	}

	t.Setenv(SplitJVMTargetEnv, "1")
	units, split = planSettingsSessions(Input{ScanPaths: []string{root}}, model, root, check, logf)
	if len(units) != 1 || units[0].Spec.JVMTarget != "11" || split.Sessions != 2 {
		t.Fatalf("units=%+v split=%+v; want one session for :b:main at JVM target 11", units, split)
	}
}
