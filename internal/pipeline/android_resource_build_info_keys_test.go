package pipeline

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	rbSettingsRel = "settings.gradle.kts"
	rbBuildRel    = "lib/build.gradle.kts"
	rbResDirRel   = "lib/src/main/res"
	rbLayoutRel   = "lib/src/main/res/layout/view.xml"
	// rbHardcodedNamespace is a layout LibraryCustomView reports in a library module.
	rbHardcodedNamespace = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<LinearLayout xmlns:android=\"http://schemas.android.com/apk/res/android\"\n    xmlns:app=\"http://schemas.android.com/apk/res/com.example.lib\"\n    android:layout_width=\"match_parent\"\n    android:layout_height=\"match_parent\" />\n"
)

func rbCatalogOf(id string) string {
	return "[plugins]\nmod = { id = \"" + id + "\", version = \"8.5.0\" }\n"
}

func rbBuildWithAlias(accessor string) string {
	return "plugins {\n    alias(" + accessor + ".plugins.mod)\n}\n\nandroid {\n    namespace = \"com.example.lib\"\n    defaultConfig {\n        minSdk = 21\n    }\n}\n"
}

// rbEdit is one file write of a cold -> warm sequence. unkeyed marks a file no active rule reads, whose edit must leave
// every key alone.
type rbEdit struct {
	rel, body string
	want      int
	unkeyed   bool
}

// resourceRunKeys are the keys that can replay resource-rule findings: each res directory's resource key, the resource
// bundle key, the Android project bundle fingerprint and the findings bundle's XML inputs.
func resourceRunKeys(t *testing.T, project *android.Project, active []*api.Rule) []string {
	t.Helper()
	hashutil.ResetDefault()
	in := AndroidInput{Project: project, ActiveRules: active, RuleHash: "test-rule-hash", LibraryFactsFP: "test-lf"}
	deps := AndroidPhase{}.androidResourceDeps(active)
	kinds := androidValuesScanKinds(deps)
	fps := newResDirFingerprints("", len(project.ResDirs))
	var keys []string
	for _, resDir := range project.ResDirs {
		keys = append(keys, "resource "+in.resourceKey(resDir, fps.fingerprint(resDir), deps, kinds, false))
	}
	keys = append(keys, "bundle "+in.resourceBundleKey(mergedResourceIndexFingerprintWith(project.ResDirs, &fps), deps, kinds, false))
	projectFP, ok := in.androidProjectBundleFingerprint(classifyAndroidPhaseNeeds(active), deps, kinds, &fps)
	if !ok {
		t.Fatal("android project bundle fingerprint failed")
	}
	keys = append(keys, "project "+projectFP)
	keys = append(keys, "xml "+xmlInputsFingerprint(ProjectHostState{}, project, nil, buildFileReadersOf(active)))
	return keys
}

// runBuildInfoPhase runs the Android phase over project with active, cache-free when cacheDir is "", and returns the
// number of findings.
func runBuildInfoPhase(t *testing.T, project *android.Project, active []*api.Rule, cacheDir string) int {
	t.Helper()
	hashutil.ResetDefault()
	writer := scanner.NewAndroidCacheWriter(2)
	res, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
		Project:        project,
		ActiveRules:    active,
		Dispatcher:     rules.NewDispatcher(active, nil),
		RuleHash:       "test-rule-hash",
		LibraryFactsFP: "test-lf",
		CacheDir:       cacheDir,
		CacheWriter:    writer,
	})
	if err != nil {
		t.Fatalf("AndroidPhase.Run: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return res.Findings.Len()
}

// runResourceEdits runs the cold run, then each edit, checking every warm run against a cache-free one and that the
// keys move exactly for the files the rules read.
func runResourceEdits(t *testing.T, p *manifestSourceProject, project *android.Project, active []*api.Rule, cold int, edits []rbEdit) {
	t.Helper()
	cacheDir := filepath.Join(p.dir, ".krit", "android-findings-cache")
	if got := runBuildInfoPhase(t, project, active, ""); got != cold {
		t.Fatalf("cold cache-free findings = %d, want %d", got, cold)
	}
	if got := runBuildInfoPhase(t, project, active, cacheDir); got != cold {
		t.Fatalf("cold findings = %d, want %d", got, cold)
	}
	for _, e := range edits {
		before := resourceRunKeys(t, project, active)
		p.write(e.rel, e.body)
		after := resourceRunKeys(t, project, active)
		for i := range before {
			if moved := before[i] != after[i]; moved == e.unkeyed {
				t.Errorf("edit of %s: %s key moved = %v, want %v", e.rel, strings.Fields(before[i])[0], moved, !e.unkeyed)
			}
		}
		if got := runBuildInfoPhase(t, project, active, ""); got != e.want {
			t.Fatalf("edit of %s: cache-free findings = %d, want %d", e.rel, got, e.want)
		}
		if got := runBuildInfoPhase(t, project, active, cacheDir); got != e.want {
			t.Fatalf("edit of %s: warm findings = %d, cache-free %d", e.rel, got, e.want)
		}
	}
}

// LibraryCustomView reports a hardcoded namespace only in a library module, which it reads from the module's build
// script through readManifestGradleInfo, resolving plugin aliases through the version catalogs the script's search
// reaches. A catalog-only edit that turns the module into an app must re-run it through every key that replays
// resource findings; catalogs and build files no lookup reads must leave the keys alone (#1883). Edits run forward
// only, so a warm run can't pass by replaying an entry an earlier state wrote.
func TestResourceRules_KeyOnBuildScriptAndCatalogs(t *testing.T) {
	app, library := rbCatalogOf("com.android.application"), rbCatalogOf("com.android.library")
	rule := registeredRule(t, "LibraryCustomView")
	for _, test := range []struct {
		name       string
		noSettings bool
		// catalogRel is the catalog the cold run reads and accessor the name the build script reaches it by.
		catalogRel, accessor string
		edits                []rbEdit
	}{
		{"catalog plugin id", false, "gradle/libs.versions.toml", "libs", []rbEdit{
			{"gradle/libs.versions.toml", app, 0, false},
			{"gradle/libs.versions.toml", library + "# a library again\n", 1, false},
		}},
		{"no settings file, catalog plugin id", true, "gradle/libs.versions.toml", "libs", []rbEdit{
			{"gradle/libs.versions.toml", app, 0, false},
			{"gradle/libs.versions.toml", library + "# a library again\n", 1, false},
		}},
		// The catalog search reads every gradle/*.versions.toml, not only libs.
		{"differently named catalog", false, "gradle/deps.versions.toml", "deps", []rbEdit{
			{"gradle/deps.versions.toml", app, 0, false},
		}},
		{"no settings file, differently named catalog", true, "gradle/deps.versions.toml", "deps", []rbEdit{
			{"gradle/deps.versions.toml", app, 0, false},
		}},
		// The search from the module directory stops at the nearest directory with a catalog.
		{"nearer catalog created", false, "gradle/libs.versions.toml", "libs", []rbEdit{
			{"lib/gradle/libs.versions.toml", app, 0, false},
		}},
		{"build script edit", false, "gradle/libs.versions.toml", "libs", []rbEdit{
			{rbBuildRel, "plugins {\n    id(\"com.android.application\")\n}\n", 0, false},
		}},
		// Files no lookup reads: a catalog the search never reaches, a TOML that isn't a catalog, another module's
		// build script outside the scan.
		{"unrelated files", false, "gradle/libs.versions.toml", "libs", []rbEdit{
			{"tools/gradle/libs.versions.toml", app, 1, true},
			{"gradle/notes.toml", app, 1, true},
			{"other/build.gradle.kts", "plugins {\n    id(\"com.android.application\")\n}\n", 1, true},
			{"gradle/libs.versions.toml", app, 0, false},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := &manifestSourceProject{t: t, dir: t.TempDir()}
			project := &android.Project{ResDirs: []string{p.path(rbResDirRel)}, GradlePaths: []string{p.path(rbBuildRel)}}
			if !test.noSettings {
				p.write(rbSettingsRel, "include(\":lib\")\n")
				project.GradlePaths = append(project.GradlePaths, p.path(rbSettingsRel))
			}
			p.write(rbBuildRel, rbBuildWithAlias(test.accessor))
			p.write(test.catalogRel, library)
			p.write(rbLayoutRel, rbHardcodedNamespace)
			runResourceEdits(t, p, project, []*api.Rule{rule}, 1, test.edits)
		})
	}
}

// UnusedAttributeResource judges an attribute by the minSdk of the layout's module, read from its build script
// (resourceModuleMinSdk), so a minSdk edit there must re-run it (#1883).
func TestResourceRules_KeyOnBuildScriptMinSdk(t *testing.T) {
	const layout = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<LinearLayout xmlns:android=\"http://schemas.android.com/apk/res/android\"\n    android:layout_width=\"match_parent\"\n    android:layout_height=\"match_parent\"\n    android:foreground=\"@null\" />\n"
	buildOf := func(minSdk string) string {
		return "plugins {\n    id(\"com.android.library\")\n}\n\nandroid {\n    defaultConfig {\n        minSdk = " + minSdk + "\n    }\n}\n"
	}
	rule := registeredRule(t, "UnusedAttributeResource")
	p := &manifestSourceProject{t: t, dir: t.TempDir()}
	p.write(rbSettingsRel, "include(\":lib\")\n")
	p.write(rbBuildRel, buildOf("21"))
	p.write(rbLayoutRel, layout)
	project := &android.Project{ResDirs: []string{p.path(rbResDirRel)}, GradlePaths: []string{p.path(rbBuildRel), p.path(rbSettingsRel)}}
	runResourceEdits(t, p, project, []*api.Rule{rule}, 1, []rbEdit{
		{rbBuildRel, buildOf("23"), 0, false},
		{rbBuildRel, buildOf("22"), 1, false},
	})
}

// A run with no active resource rule that reads build files keys none of them: editing the build script or the
// catalog leaves every resource key alone, and the run computes no build-file section for them (#1883).
func TestResourceRules_NoBuildInfoReaderKeysNoBuildFiles(t *testing.T) {
	// HardcodedValuesResource reads only the layouts.
	rule := registeredRule(t, "HardcodedValuesResource")
	if reads := buildFileReadersOf([]*api.Rule{rule}); reads.resources != nil {
		t.Fatalf("buildFileReadersOf(HardcodedValuesResource).resources = %v, want nil", reads.resources)
	}
	const layout = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<LinearLayout xmlns:android=\"http://schemas.android.com/apk/res/android\"\n    android:layout_width=\"match_parent\"\n    android:layout_height=\"match_parent\">\n    <TextView\n        android:layout_width=\"wrap_content\"\n        android:layout_height=\"wrap_content\"\n        android:text=\"Hello\" />\n</LinearLayout>\n"
	p := &manifestSourceProject{t: t, dir: t.TempDir()}
	p.write(rbSettingsRel, "include(\":lib\")\n")
	p.write(rbBuildRel, rbBuildWithAlias("libs"))
	p.write("gradle/libs.versions.toml", rbCatalogOf("com.android.library"))
	p.write(rbLayoutRel, layout)
	// No Gradle paths: the scanned build scripts key the findings bundle and the project bundle for every rule.
	project := &android.Project{ResDirs: []string{p.path(rbResDirRel)}}
	runResourceEdits(t, p, project, []*api.Rule{rule}, 1, []rbEdit{
		{"gradle/libs.versions.toml", rbCatalogOf("com.android.application"), 1, true},
		// The module's SDK levels key every Android unit (sdkLevelsFingerprint), so the edit keeps minSdk.
		{rbBuildRel, strings.Replace(rbBuildWithAlias("libs"), "alias(libs.plugins.mod)", "id(\"com.android.application\")", 1), 1, true},
		{rbSettingsRel, "include(\":lib\", \":app\")\n", 1, true},
	})
}

// Every resource rule that reads build files declares what it reads for a res directory, and each of those reads
// moves the keys of the run: the module build script, the catalog its alias resolves through for a rule that reads
// the module's plugins, and the files a rule reads itself (#1883).
func TestResourceBuildInfoReaders_KeyOnWhatTheyRead(t *testing.T) {
	type edit struct{ rel, body string }
	for _, test := range []struct {
		id string
		// catalog: the rule reads the module through readManifestGradleInfo, which resolves plugin aliases through
		// the version catalogs.
		catalog bool
		extra   []edit
	}{
		{"LibraryCustomView", true, nil},
		{"LabelForResource", true, []edit{{"lib/src/main/AndroidManifest.xml", "<manifest><uses-sdk android:minSdkVersion=\"16\" /></manifest>\n"}}},
		{"UnusedAttributeResource", true, []edit{{"lib/src/main/AndroidManifest.xml", "<manifest><uses-sdk android:minSdkVersion=\"16\" /></manifest>\n"}}},
		{"TranslucentOrientation", true, []edit{{"lib/src/main/AndroidManifest.xml", "<manifest><application /></manifest>\n"}}},
		{"RelativeClassResolution", true, []edit{{"lib/src/main/AndroidManifest.xml", "<manifest package=\"com.other\" />\n"}}},
		{"VectorDrawableCompat", true, nil},
		// The resource graph reads the build scripts above the source set for resource references.
		{"UnusedResources", false, []edit{{"lib/src/build.gradle", "// R.string.a\n"}, {"lib/src/main/AndroidManifest.xml", "<manifest />\n"}}},
		{"UnusedIds", false, []edit{{"lib/src/build.gradle.kts", "// R.id.a\n"}}},
	} {
		t.Run(test.id, func(t *testing.T) {
			rule := registeredRule(t, test.id)
			if reads := buildFileReadersOf([]*api.Rule{rule}); reads.resources == nil {
				t.Fatalf("%s does not declare that it reads build files (rules.ResourceBuildInfoReader)", test.id)
			}
			p := &manifestSourceProject{t: t, dir: t.TempDir()}
			p.write(rbSettingsRel, "include(\":lib\")\n")
			p.write(rbBuildRel, rbBuildWithAlias("libs"))
			p.write("gradle/libs.versions.toml", rbCatalogOf("com.android.library"))
			p.write(rbLayoutRel, rbHardcodedNamespace)
			project := &android.Project{ResDirs: []string{p.path(rbResDirRel)}}
			edits := []edit{{rbBuildRel, rbBuildWithAlias("libs") + "// edited\n"}}
			if test.catalog {
				edits = append(edits, edit{"gradle/libs.versions.toml", rbCatalogOf("com.android.application")})
			}
			for _, e := range append(edits, test.extra...) {
				before := resourceRunKeys(t, project, []*api.Rule{rule})
				p.write(e.rel, e.body)
				after := resourceRunKeys(t, project, []*api.Rule{rule})
				for i := range before {
					if before[i] == after[i] {
						t.Errorf("edit of %s: %s key did not move", e.rel, strings.Fields(before[i])[0])
					}
				}
			}
		})
	}
}
