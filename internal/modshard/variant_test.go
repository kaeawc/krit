package modshard

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

// variantModel is an Android project whose model exports the debug variant: main+debug and
// test+testDebug. The release, flavor and testRelease directories exist on disk only.
func variantModel(t *testing.T, extra ...string) (*gradlemodel.Model, string) {
	t.Helper()
	root := t.TempDir()
	for _, rel := range append([]string{
		"src/main/kotlin/Api.kt", "src/debug/kotlin/Flags.kt", "src/release/kotlin/Flags.kt",
		"src/free/kotlin/Free.kt", "src/freeRelease/java/FreeRelease.java",
		"src/test/kotlin/T.kt", "src/testDebug/kotlin/Fake.kt", "src/testRelease/kotlin/Fake.kt",
		"src/testFixtures/kotlin/Fixture.kt",
	}, extra...) {
		p := filepath.Join(root, "app", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("class X"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dirs := func(sets ...string) []string {
		var out []string
		for _, s := range sets {
			out = append(out, filepath.Join(root, "app", "src", s, "kotlin"))
		}
		return out
	}
	gen := filepath.Join(root, "app", "build", "generated", "source", "buildConfig", "debug")
	model := &gradlemodel.Model{Projects: []gradlemodel.Project{{Path: ":app", Dir: filepath.Join(root, "app"), SourceSets: []gradlemodel.SourceSet{
		{Name: "debug", Kind: "main", Platform: "android", Variant: "debug", SourceDirs: dirs("main", "debug"), GeneratedSourceDirs: []string{gen}},
		{Name: "debugUnitTest", Kind: "test", Platform: "android", Variant: "debug", SourceDirs: dirs("test", "testDebug")},
	}}}}
	return model, root
}

func appFile(root, rel string) string {
	return filepath.Join(root, "app", "src", filepath.FromSlash(rel))
}

func TestVariantUnitsCompileEachUnexportedSetWithItsVariantsBase(t *testing.T) {
	model, root := variantModel(t)
	units, _ := Plan(model, root, []string{root})
	dir := func(set, lang string) string { return filepath.Join(root, "app", "src", set, lang) }
	gated := []string{
		appFile(root, "release/kotlin/Flags.kt"), appFile(root, "freeRelease/java/FreeRelease.java"),
		appFile(root, "testRelease/kotlin/Fake.kt"), appFile(root, "testFixtures/kotlin/Fixture.kt"),
		filepath.Join(root, "elsewhere", "src", "main", "kotlin", "B.kt"),
	}
	got, beyond := variantUnits(model, units, nil, gated)
	byID := map[string]Unit{}
	var ids []string
	for _, u := range got {
		ids = append(ids, u.Spec.ID)
		byID[u.Spec.ID] = u
	}
	if !slices.Equal(ids, []string{":app:variant-freeRelease", ":app:variant-release", ":app:variant-testRelease"}) || len(beyond) != 0 {
		t.Fatalf("one unit per unexported set with a template, got %v (beyond %v)", ids, beyond)
	}

	release := byID[":app:variant-release"].Spec
	if release.Kind != "main" || !slices.Equal(release.SourceRoots, []string{dir("release", "kotlin")}) {
		t.Errorf("release compiles its own directory as a main set: %+v", release)
	}
	gen := filepath.Join(root, "app", "build", "generated", "source", "buildConfig", "debug")
	if !slices.Equal(release.ContextSourceRoots, []string{dir("main", "kotlin"), gen}) {
		t.Errorf("release sees src/main and the generated sources but never src/debug: %v", release.ContextSourceRoots)
	}

	combined := byID[":app:variant-freeRelease"].Spec
	if !slices.Equal(combined.SourceRoots, []string{dir("freeRelease", "java")}) ||
		!slices.Equal(combined.ContextSourceRoots, []string{dir("main", "kotlin"), dir("free", "kotlin"), dir("release", "kotlin"), gen}) {
		t.Errorf("freeRelease sees the flavor and build type it combines: %+v", combined)
	}

	test := byID[":app:variant-testRelease"].Spec
	if test.Kind != "test" || !slices.Contains(test.ContextSourceRoots, dir("test", "kotlin")) || slices.Contains(test.ContextSourceRoots, dir("testDebug", "kotlin")) {
		t.Errorf("testRelease sees src/test but never src/testDebug: %+v", test)
	}
	if sources := byID[":app:variant-testRelease"].Sources; !slices.Equal(sources, []string{appFile(root, "testRelease/kotlin/Fake.kt")}) {
		t.Errorf("a variant unit lists only its own sources: %v", sources)
	}
}

func TestVariantUnitsSkipProjectsThatKeepMergedVerdicts(t *testing.T) {
	model, root := variantModel(t)
	units, _ := Plan(model, root, []string{root})
	gated := []string{appFile(root, "release/kotlin/Flags.kt")}
	if got, _ := variantUnits(model, units, []string{":app"}, gated); len(got) != 0 {
		t.Fatalf("a skipped project's variant sets keep their merged verdicts: %+v", got)
	}
	for i := range units {
		units[i].ContextDropped = true
	}
	if got, _ := variantUnits(model, units, nil, gated); len(got) != 0 {
		t.Fatalf("a template without its dependency context is not reused: %+v", got)
	}
	if got, _ := variantUnits(model, nil, nil, gated); len(got) != 0 {
		t.Fatalf("a project without an exported unit of the kind has no template: %+v", got)
	}
}

func TestVariantUnitsStopAtTheLimit(t *testing.T) {
	var extra, gated []string
	for i := range maxVariantUnits + 2 {
		extra = append(extra, fmt.Sprintf("src/flavor%02d/kotlin/F.kt", i))
	}
	model, root := variantModel(t, extra...)
	for _, rel := range extra {
		gated = append(gated, filepath.Join(root, "app", filepath.FromSlash(rel)))
	}
	units, _ := Plan(model, root, []string{root})
	got, beyond := variantUnits(model, units, nil, gated)
	if len(got) != maxVariantUnits || len(beyond) != 2 || !strings.Contains(beyond[0], filepath.Join("src", "flavor16")) || !strings.HasSuffix(beyond[0], "(1 gated files)") {
		t.Fatalf("variant sets beyond the limit are listed, not compiled: %d units, beyond %v", len(got), beyond)
	}
}

func TestNameComponents(t *testing.T) {
	if got := nameComponents("freeStagingRelease", false); !slices.Equal(got, []string{"free", "freeStaging", "staging", "stagingRelease", "release"}) {
		t.Errorf("got %v", got)
	}
	if got := nameComponents("debug", true); !slices.Equal(got, []string{"debug"}) {
		t.Errorf("got %v", got)
	}
	if got := nameComponents("Release", false); len(got) != 0 {
		t.Errorf("a single word has no other components: %v", got)
	}
	for set, want := range map[string]string{"testRelease": "test", "test": "test", "androidTestFree": "androidTest", "testFixturesDebug": "", "release": "main", "testing": "main"} {
		if kind, _ := variantKind(set); kind != want {
			t.Errorf("variantKind(%q) = %q, want %q", set, kind, want)
		}
	}
}

// A class each build type declares gates both copies in the merged compile. The exported debug copy
// was rechecked before #1023; the release copy, outside the model, is now rechecked too.
func TestCheckIsolatedRechecksVariantSetsOutsideTheModel(t *testing.T) {
	t.Setenv("KRIT_WORKER_AOT", "0")
	model, root := variantModel(t)
	writeModel(t, root, model)
	debug, release, api := appFile(root, "debug/kotlin/Flags.kt"), appFile(root, "release/kotlin/Flags.kt"), appFile(root, "main/kotlin/Api.kt")
	request, _ := json.Marshal(map[string]any{
		"id": 2, "files": []map[string]string{{"path": api}, {"path": debug}, {"path": release}}, "rules": []string{"R"},
	})
	java := fakeWorker(t, []string{":app:debug", ":app:debugUnitTest", ":app:variant-release"}, []string{release})
	resp, rechecked, err := CheckIsolated(context.Background(), Input{
		ScanPaths: []string{root}, JarPath: filepath.Join(root, "krit-fir.jar"), JavaPath: java, Workers: 1,
		CacheDir: filepath.Join(t.TempDir(), "cache"), CheckRequest: request,
	}, []string{debug, release})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(rechecked, debug) || !slices.Contains(rechecked, release) {
		t.Fatalf("the release copy should be rechecked, got %v", rechecked)
	}
	if !strings.Contains(string(resp), release) {
		t.Fatalf("the response should carry the release file's verdict: %s", resp)
	}
}
