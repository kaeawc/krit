package rules

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/rules/api"
)

// The resource rules that read build files beyond the res files (lookupManifestGradleInfo, readManifestGradleInfo,
// resourceModuleMinSdk, moduleIdentityForRes, moduleVectorSettingsForRes, buildResourceGraph) declare it, so the
// resource caches key on what they read (#1883). A new reader must join this list and implement
// ResourceBuildInfoReader.
func TestResourceBuildInfoReaders(t *testing.T) {
	want := []string{
		"LabelForResource", "LibraryCustomView", "RelativeClassResolution", "TranslucentOrientation",
		"UnusedAttributeResource", "UnusedIds", "UnusedResources", "VectorDrawableCompat",
	}
	var got []string
	for _, r := range api.Registry {
		if _, ok := r.Implementation.(ResourceBuildInfoReader); !ok {
			continue
		}
		if scope := RuleScope(r); scope != api.ScopeResource {
			t.Errorf("%s implements ResourceBuildInfoReader but runs in scope %v, not ScopeResource", r.ID, scope)
		}
		got = append(got, r.ID)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResourceBuildInfoReader rules = %v, want %v", got, want)
	}
	ids, _ := ResourceBuildInfoReadersOf(api.Registry)
	sort.Strings(ids)
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ResourceBuildInfoReadersOf(registry) = %v, want %v", ids, want)
	}
}

// Each lookup a rule declares searches the build script the rule reads: the first of ManifestBuildScriptCandidates
// that exists is the module's own script, for the res directory of any source set.
func TestResourceBuildInfoLookupsReachTheModuleScript(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct{ resDir, script string }{
		{"lib/src/main/res", "lib/build.gradle.kts"},
		{"lib/src/debug/res", "lib/build.gradle.kts"},
	} {
		script := filepath.Join(root, test.script)
		if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(script, []byte("plugins {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		resDir := filepath.Join(root, test.resDir)
		_, readers := ResourceBuildInfoReadersOf(api.Registry)
		for _, reader := range readers {
			for _, lookup := range reader.ResourceBuildInfoReads(resDir).Lookups {
				if got := ManifestBuildScript(lookup); got != script {
					t.Errorf("%T lookup %s for %s reads %q, want %q", reader, lookup, test.resDir, got, script)
				}
			}
		}
	}
}

// VectorDrawableCompat names the module by the nearest settings script above it, so its reads list every settings
// path searched up to that one: creating a nearer one changes the name.
func TestGradleSettingsCandidates(t *testing.T) {
	root := t.TempDir()
	module := filepath.Join(root, "features", "lib")
	settings := filepath.Join(root, "settings.gradle.kts")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("include(\":features:lib\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(root, "features", "settings.gradle.kts"), filepath.Join(root, "features", "settings.gradle"),
		settings,
	}
	if got := gradleSettingsCandidates(module); !reflect.DeepEqual(got, want) {
		t.Fatalf("gradleSettingsCandidates = %v, want %v", got, want)
	}
	if got := gradleModulePath(module); got != ":features:lib"+string(filepath.Separator) {
		t.Fatalf("gradleModulePath = %q", got)
	}
}
