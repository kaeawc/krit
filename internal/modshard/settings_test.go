package modshard

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
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
