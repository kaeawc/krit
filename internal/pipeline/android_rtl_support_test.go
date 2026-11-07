package pipeline

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// rtlProject lays out a Gradle build and returns the manifest and build script paths the scan would collect.
func rtlProject(t *testing.T, settings string, files map[string]string) (dir string, manifests, gradle []string) {
	t.Helper()
	dir = t.TempDir()
	writeRtlFile(t, filepath.Join(dir, "settings.gradle.kts"), settings)
	for rel, body := range files {
		path := filepath.Join(dir, rel)
		writeRtlFile(t, path, body)
		switch filepath.Base(rel) {
		case "AndroidManifest.xml":
			manifests = append(manifests, path)
		case "build.gradle.kts":
			gradle = append(gradle, path)
		}
	}
	return dir, manifests, gradle
}

func TestProjectRtlMainsFollowModuleDependencies(t *testing.T) {
	// A plugin-driven settings file lists no modules, so the resolver must
	// also work from the scanned build scripts alone.
	settings := map[string]string{
		"static includes":  "include(\":app\", \":tool\", \":shared\", \":applib\", \":toollib\", \":orphan\", \":testonly\")\n",
		"partial includes": "include(\":tool\")\n",
		"plugin-driven":    "plugins { id(\"x\") }\n",
	}
	appMain := api.RtlMainProject{HasApplication: true, MinSdk: 21, TargetSdk: 34}
	toolMain := api.RtlMainProject{HasApplication: true, RtlOff: true, MinSdk: 21, TargetSdk: 34}
	for name, settingsBody := range settings {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{
				"app/src/main/AndroidManifest.xml":  rtlManifestBody("unset", 34, 21, true),
				"tool/src/main/AndroidManifest.xml": rtlManifestBody("false", 34, 21, true),
				"app/build.gradle.kts":              "dependencies { implementation(project(\":applib\"))\n implementation(project(\":shared\"))\n testImplementation(project(\":testonly\")) }\n",
				"tool/build.gradle.kts":             "dependencies { implementation(project(\":toollib\"))\n api(project(\":shared\")) }\n",
			}
			for _, m := range []string{"applib", "toollib", "shared", "orphan", "testonly"} {
				files[m+"/build.gradle.kts"] = "\n"
			}
			dir, manifests, gradle := rtlProject(t, settingsBody, files)
			resolve, _ := projectRtlMains(manifests, gradle)
			cases := map[string][]api.RtlMainProject{
				"app/src/main/res/layout/a.xml":      {appMain},
				"applib/src/main/res/layout/a.xml":   {appMain},
				"tool/src/main/res/layout/a.xml":     {toolMain},
				"toollib/src/main/res/layout/a.xml":  {toolMain},
				"shared/src/main/res/layout/a.xml":   {appMain, toolMain},
				"orphan/src/main/res/layout/a.xml":   nil,
				"testonly/src/main/res/layout/a.xml": nil,
			}
			for rel, want := range cases {
				got, known := resolve(filepath.Join(dir, rel))
				if !known {
					t.Errorf("%s: apps unknown", rel)
				}
				if !sameMains(got, want) {
					t.Errorf("%s: got %v, want %v", rel, got, want)
				}
			}
		})
	}
}

func sameMains(got, want []api.RtlMainProject) bool {
	if len(got) == 0 && len(want) == 0 {
		return true
	}
	key := func(m api.RtlMainProject) string { return fmt.Sprintf("%v", m) }
	sorted := func(in []api.RtlMainProject) []string {
		out := make([]string, 0, len(in))
		for _, m := range in {
			out = append(out, key(m))
		}
		sort.Strings(out)
		return out
	}
	return reflect.DeepEqual(sorted(got), sorted(want))
}

func TestProjectRtlMainsWithoutGradleGraph(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "app", "src", "main", "AndroidManifest.xml")
	writeRtlFile(t, app, rtlManifestBody("false", 34, 21, true))
	lib := filepath.Join(dir, "lib", "src", "main", "AndroidManifest.xml")
	writeRtlFile(t, lib, rtlManifestBody("unset", 34, 21, false))

	resolve, _ := projectRtlMains([]string{app, lib}, nil)
	own, known := resolve(filepath.Join(dir, "app", "src", "main", "res", "layout", "a.xml"))
	if !known || len(own) != 1 || !own[0].RtlOff {
		t.Errorf("an app module is judged by itself: got %v, %v", own, known)
	}
	shared, known := resolve(filepath.Join(dir, "lib", "src", "main", "res", "layout", "a.xml"))
	if !known || len(shared) != 1 || !shared[0].HasApplication {
		t.Errorf("a library module is judged by the apps in the scan: got %v, %v", shared, known)
	}

	resolve, _ = projectRtlMains([]string{lib}, nil)
	if _, known := resolve(filepath.Join(dir, "lib", "src", "main", "res", "layout", "a.xml")); known {
		t.Error("a scan with no application leaves the apps unknown")
	}
	resolve, _ = projectRtlMains(nil, nil)
	if _, known := resolve("res/layout/a.xml"); known {
		t.Error("a scan with no manifests leaves the apps unknown")
	}
}

func TestProjectRtlMainsIgnoreNonMainManifests(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "app", "src", "main", "AndroidManifest.xml")
	debug := filepath.Join(dir, "app", "src", "debug", "AndroidManifest.xml")
	writeRtlFile(t, main, rtlManifestBody("unset", 34, 21, true))
	writeRtlFile(t, debug, rtlManifestBody("false", 34, 21, true))
	resolve, _ := projectRtlMains([]string{main, debug}, nil)
	got, _ := resolve(filepath.Join(dir, "app", "src", "main", "res", "layout", "a.xml"))
	if len(got) != 1 || got[0].RtlOff {
		t.Fatalf("got %v, want only the main manifest", got)
	}
}

func TestProjectRtlMainsFingerprintTracksInputs(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "app", "src", "main", "AndroidManifest.xml")
	writeRtlFile(t, manifest, rtlManifestBody("unset", 34, 21, true))
	_, before := projectRtlMains([]string{manifest}, nil)
	writeRtlFile(t, manifest, rtlManifestBody("false", 34, 21, true))
	_, off := projectRtlMains([]string{manifest}, nil)
	writeRtlFile(t, manifest, rtlManifestBody("unset", 16, 14, true))
	_, old := projectRtlMains([]string{manifest}, nil)
	if before == off || before == old || off == old {
		t.Fatalf("fingerprints must differ: %s %s %s", before, off, old)
	}
}
