package pipeline

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// rbTwoModules writes a project of two library modules, a and b, whose build scripts both resolve their plugin
// through the root version catalog and whose layouts LibraryCustomView reports, and returns it with both res
// directories.
func rbTwoModules(t *testing.T) (*manifestSourceProject, *android.Project) {
	t.Helper()
	p := &manifestSourceProject{t: t, dir: t.TempDir()}
	p.write(rbSettingsRel, "include(\":a\", \":b\")\n")
	p.write("gradle/libs.versions.toml", rbCatalogOf("com.android.library"))
	for _, m := range []string{"a", "b"} {
		p.write(m+"/build.gradle.kts", rbBuildWithAlias("libs"))
		p.write(m+"/src/main/AndroidManifest.xml", "<manifest />\n")
		p.write(m+"/src/main/res/layout/view.xml", rbHardcodedNamespace)
	}
	return p, &android.Project{ResDirs: []string{p.path("a/src/main/res"), p.path("b/src/main/res")}}
}

// rbDirEdit is a file write, the modules whose res-dir resource keys it must move, and the findings after it (-1:
// any, as long as the warm run matches a cache-free one).
type rbDirEdit struct {
	rel, body string
	moves     []string
	want      int
}

// runDirKeyEdits runs a cold run, then each edit: the res-dir keys that move are exactly those of edit.moves, the
// whole-project keys move with any of them, and the warm run reports what a cache-free one does.
func runDirKeyEdits(t *testing.T, p *manifestSourceProject, project *android.Project, active []*api.Rule, edits []rbDirEdit) {
	t.Helper()
	cacheDir := filepath.Join(p.dir, ".krit", "android-findings-cache")
	if truth, warm := runBuildInfoPhase(t, project, active, ""), runBuildInfoPhase(t, project, active, cacheDir); warm != truth {
		t.Fatalf("cold findings = %d, cache-free %d", warm, truth)
	}
	modules := []string{"a", "b"}
	for _, e := range edits {
		before := resourceRunKeys(t, project, active)
		p.write(e.rel, e.body)
		after := resourceRunKeys(t, project, active)
		var moved []string
		for i, m := range modules {
			if before[i] != after[i] {
				moved = append(moved, m)
			}
		}
		if !reflect.DeepEqual(moved, e.moves) {
			t.Errorf("edit of %s: res-dir keys of %v moved, want %v", e.rel, moved, e.moves)
		}
		for i := len(modules); i < len(before); i++ {
			if moved := before[i] != after[i]; moved != (len(e.moves) > 0) {
				t.Errorf("edit of %s: %s key moved = %v, want %v", e.rel, strings.Fields(before[i])[0], moved, len(e.moves) > 0)
			}
		}
		truth := runBuildInfoPhase(t, project, active, "")
		if warm := runBuildInfoPhase(t, project, active, cacheDir); warm != truth || (e.want >= 0 && truth != e.want) {
			t.Fatalf("edit of %s: warm findings = %d, cache-free %d, want %d", e.rel, warm, truth, e.want)
		}
	}
}

// A res directory's key covers what the local readers read for it alone, so an edit of one module's build script
// re-runs them for that module only; a file both modules read, the catalog their aliases resolve through, moves both
// (#1943). Edits run forward only, so a warm run can't pass by replaying an entry an earlier state wrote.
func TestResourceDirKeys_LocalReaderKeysOnItsModule(t *testing.T) {
	p, project := rbTwoModules(t)
	app := strings.Replace(rbBuildWithAlias("libs"), "alias(libs.plugins.mod)", "id(\"com.android.application\")", 1)
	runDirKeyEdits(t, p, project, []*api.Rule{registeredRule(t, "LibraryCustomView")}, []rbDirEdit{
		{"a/build.gradle.kts", rbBuildWithAlias("libs") + "// edited\n", []string{"a"}, 2},
		// a becomes an app: its finding goes, b's stays.
		{"a/build.gradle.kts", app, []string{"a"}, 1},
		{"b/build.gradle.kts", rbBuildWithAlias("libs") + "// edited\n", []string{"b"}, 1},
		// Only b resolves its plugin through the catalog now, which makes it an app.
		{"gradle/libs.versions.toml", rbCatalogOf("com.android.application"), []string{"b"}, 0},
		{"a/build.gradle.kts", rbBuildWithAlias("libs"), []string{"a"}, 0},
		{"gradle/libs.versions.toml", rbCatalogOf("com.android.library"), []string{"a", "b"}, 2},
		// A build script no lookup reaches moves nothing.
		{"other/build.gradle.kts", app, nil, 2},
	})
}

// VectorDrawableCompat names a module by the settings script, which every module's res directory reads (#1943).
func TestResourceDirKeys_SettingsMovesEveryModule(t *testing.T) {
	p, project := rbTwoModules(t)
	runDirKeyEdits(t, p, project, []*api.Rule{registeredRule(t, "VectorDrawableCompat")}, []rbDirEdit{
		{"a/build.gradle.kts", rbBuildWithAlias("libs") + "// edited\n", []string{"a"}, -1},
		{rbSettingsRel, "include(\":a\", \":b\", \":c\")\n", []string{"a", "b"}, -1},
	})
}

// UnusedResources reads the files beside every res root for each res directory, so an edit beside one moves every
// res directory's key, alone or beside a local reader (#1943).
func TestResourceDirKeys_ProjectReaderMovesEveryModule(t *testing.T) {
	for _, ids := range [][]string{{"UnusedResources"}, {"UnusedResources", "LibraryCustomView"}} {
		t.Run(strings.Join(ids, "+"), func(t *testing.T) {
			p, project := rbTwoModules(t)
			var active []*api.Rule
			for _, id := range ids {
				active = append(active, registeredRule(t, id))
			}
			runDirKeyEdits(t, p, project, active, []rbDirEdit{
				{"a/src/build.gradle", "// R.string.a\n", []string{"a", "b"}, -1},
				{"a/build.gradle.kts", rbBuildWithAlias("libs") + "// R.string.b\n", []string{"a", "b"}, -1},
			})
		})
	}
}

// The readers that declare their reads local are those whose findings for a res directory come from that directory
// alone; the unused-resource graph spans every res root.
func TestResourceBuildInfoRules_SplitLocalAndProjectReaders(t *testing.T) {
	r := resourceBuildInfoRulesOf(api.Registry)
	if r == nil {
		t.Fatal("no resource rule reads build files")
	}
	ids, readers := rules.ResourceBuildInfoReadersOf(api.Registry)
	var local, project []string
	for i, reader := range readers {
		if _, ok := reader.(rules.ResourceBuildInfoLocalReader); ok {
			local = append(local, ids[i])
		} else {
			project = append(project, ids[i])
		}
	}
	if len(r.local) != len(local) || len(r.project) != len(project) {
		t.Fatalf("split %d local, %d project; want %v and %v", len(r.local), len(r.project), local, project)
	}
	if want := []string{"UnusedIds", "UnusedResources"}; !sameStrings(project, want) {
		t.Fatalf("project-wide readers = %v, want %v", project, want)
	}
}

func sameStrings(a, b []string) bool {
	set := map[string]int{}
	for _, s := range a {
		set[s]++
	}
	for _, s := range b {
		set[s]--
	}
	for _, n := range set {
		if n != 0 {
			return false
		}
	}
	return len(a) == len(b)
}

// One run computes each res directory's build-info key part once and shares it between the copies of its input
// (#1943): the keys it returns are those of an input Run did not prepare.
func TestResourceBuildInfoKeys_SharedAcrossInputCopies(t *testing.T) {
	_, project := rbTwoModules(t)
	hashutil.ResetDefault()
	active := []*api.Rule{registeredRule(t, "LibraryCustomView"), registeredRule(t, "UnusedResources")}
	fresh := AndroidInput{Project: project, ActiveRules: active}
	in := fresh.withResourceBuildInfoKeys()
	copyOf := in
	for _, resDir := range project.ResDirs {
		if got, want := copyOf.resourceDirBuildInfoKey(resDir), fresh.resourceDirBuildInfoKey(resDir); got != want || got == "" {
			t.Fatalf("dir key of %s = %q, want %q", resDir, got, want)
		}
	}
	if got, want := in.resourceBuildInfoKey(), fresh.resourceBuildInfoKey(); got != want || got == "" {
		t.Fatalf("whole key = %q, want %q", got, want)
	}
	if n := len(in.buildInfoKeys.dirs); n != len(project.ResDirs) {
		t.Fatalf("run memo holds %d res-dir keys, want %d", n, len(project.ResDirs))
	}
}

// A resolver shared by a run's fingerprints gives each the value a fresh one does: what it memoizes for one set of
// lookups (a script's catalog search, the globbed directories) does not leak into another's hash (#1943).
func TestGradleInfoResolver_SharedMatchesFresh(t *testing.T) {
	p, _ := rbTwoModules(t)
	p.write("b/build.gradle.kts", rbBuildWithAlias("libs")+"dependencies { implementation(libs.x) }\n")
	p.write("gradle/libs.versions.toml", rbCatalogOf("com.android.library")+"[libraries]\nx = { module = \"a:b\", version = \"1\" }\n")
	sets := [][]string{
		{p.path("a/src/main/AndroidManifest.xml")},
		{p.path("b/src/main/res/layout/view.xml")},
		{p.path("a/src/main/AndroidManifest.xml"), p.path("b/src/main/AndroidManifest.xml")},
		{p.path("nowhere/src/main/AndroidManifest.xml")},
	}
	hashutil.ResetDefault()
	shared := newGradleInfoResolver()
	for _, set := range sets {
		if got, want := shared.fingerprint(set, 1), computeManifestGradleInfoFingerprint(set); got != want {
			t.Errorf("shared resolver fingerprint of %v = %s, fresh %s", set, got, want)
		}
	}
	if a, b := computeManifestGradleInfoFingerprint(sets[0]), computeManifestGradleInfoFingerprint(sets[1]); a == b {
		t.Fatal("the two modules' lookups hash alike")
	}
}
