package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/buildid"
	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// runCachedRtlProject runs the RTL rules through the Android phase with the findings cache on and a fixed rule hash,
// so two calls differ only in what is on disk.
func runCachedRtlProject(t *testing.T, root, cacheDir, ruleHash string) rtlCounts {
	t.Helper()
	var active []*api.Rule
	for _, id := range []string{"RtlHardcodedResource", "RtlSymmetryResource", "RtlCompatResource", "RtlEnabledManifest"} {
		active = append(active, findV2RuleForTest(t, id))
	}
	writer := scanner.NewAndroidCacheWriter(2)
	result, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
		Project:     android.DetectProject([]string{root}),
		ActiveRules: active,
		Dispatcher:  rules.NewDispatcher(active, nil),
		RuleHash:    ruleHash,
		CacheDir:    cacheDir,
		CacheWriter: writer,
	})
	if err != nil {
		t.Fatalf("AndroidPhase.Run: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	var c rtlCounts
	for _, f := range result.Findings.Findings() {
		switch f.Rule {
		case "RtlHardcodedResource":
			c.hardcoded++
		case "RtlSymmetryResource":
			c.symmetry++
		case "RtlCompatResource":
			c.compat++
		case "RtlEnabledManifest":
			switch {
			case strings.Contains(f.Message, "You must set"):
				c.enabled = append(c.enabled, "target")
			case strings.Contains(f.Message, "does not explicitly"):
				c.enabled = append(c.enabled, "proj")
			}
		}
	}
	sort.Strings(c.enabled)
	return c
}

func writeRtlApp(t *testing.T, root, supportsRtl string, target, min int) {
	t.Helper()
	writeRtlFile(t, filepath.Join(root, "settings.gradle.kts"), "include(\":app\")\n")
	writeRtlFile(t, filepath.Join(root, "app", "build.gradle.kts"), "plugins { id(\"com.android.application\") }\n")
	writeRtlFile(t, filepath.Join(root, "app", "src", "main", "AndroidManifest.xml"), rtlManifestBody(supportsRtl, target, min, true))
	writeRtlFile(t, filepath.Join(root, "app", "src", "main", "res", "layout", "main.xml"), rtlMatrixLayout)
}

// TestAndroidCache_ProjectLevelInputsInvalidate changes manifest facts that decide layout findings without touching
// the layout, and expects the warm cache to follow the change instead of replaying the first scan.
func TestAndroidCache_ProjectLevelInputsInvalidate(t *testing.T) {
	steps := []struct {
		name        string
		supportsRtl string
		target, min int
	}{
		{"rtl off", "false", 21, 21},
		{"rtl on", "true", 21, 21},
		{"rtl on, target below 17", "true", 16, 14},
		{"rtl unset, target 21", "unset", 21, 21},
	}
	root := t.TempDir()
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")
	for _, step := range steps {
		writeRtlApp(t, root, step.supportsRtl, step.target, step.min)
		warm := runCachedRtlProject(t, root, cacheDir, "fixed-rule-hash")
		// Run twice more: the second cached run must agree with a scan that has no cache at all.
		again := runCachedRtlProject(t, root, cacheDir, "fixed-rule-hash")
		fresh := runRtlProject(t, root)
		if warm.String() != fresh.String() || again.String() != fresh.String() {
			t.Fatalf("%s: cached scan %s / warm replay %s, uncached scan %s", step.name, warm, again, fresh)
		}
	}
	// The steps must actually differ, or the test proves nothing.
	writeRtlApp(t, root, "false", 21, 21)
	off := runCachedRtlProject(t, root, cacheDir, "fixed-rule-hash")
	writeRtlApp(t, root, "true", 21, 21)
	on := runCachedRtlProject(t, root, cacheDir, "fixed-rule-hash")
	if off.String() == on.String() {
		t.Fatalf("supportsRtl change did not change findings: %s", on)
	}
}

// TestAndroidCache_DifferentRuleHashMisses shows that a rule hash from another build never reads a previous
// build's entries: the stale entries are planted with a different hash and must not be replayed.
func TestAndroidCache_DifferentRuleHashMisses(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")
	writeRtlApp(t, root, "true", 21, 21)
	old := runCachedRtlProject(t, root, cacheDir, "build-a")
	if old.hardcoded == 0 {
		t.Fatalf("fixture produced no RTL findings: %s", old)
	}
	// Rewrite the project so the old build's cache entries no longer describe it, then drop
	// the layout fingerprint memo by using the new build's hash.
	writeRtlApp(t, root, "false", 21, 21)
	fresh := runCachedRtlProject(t, root, cacheDir, "build-b")
	if fresh.hardcoded != 0 {
		t.Fatalf("build-b replayed build-a findings: %s", fresh)
	}
}

const keyboardClickableLayout = `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <View android:id="@+id/tap" android:layout_width="10dp" android:layout_height="10dp" android:clickable="true"/>
</LinearLayout>
`

func writeSdkModule(t *testing.T, root, name string, minSdk int) {
	t.Helper()
	writeRtlFile(t, filepath.Join(root, name, "build.gradle.kts"), fmt.Sprintf("plugins { id(\"com.android.library\") }\nandroid {\n    defaultConfig {\n        minSdk = %d\n    }\n}\n", minSdk))
	writeRtlFile(t, filepath.Join(root, name, "src", "main", "AndroidManifest.xml"), rtlManifestBody("unset", 0, 0, false))
}

func runCachedKeyboardProject(t *testing.T, root, cacheDir string) int {
	t.Helper()
	rule := findV2RuleForTest(t, "KeyboardInaccessibleWidget")
	project := android.DetectProject([]string{root})
	facts := librarymodel.FactsForProfile(librarymodel.ProfileFromGradlePaths(project.GradlePaths))
	writer := scanner.NewAndroidCacheWriter(2)
	result, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
		Project:        project,
		ActiveRules:    []*api.Rule{rule},
		Dispatcher:     rules.NewDispatcher([]*api.Rule{rule}, nil),
		RuleHash:       "fixed-rule-hash",
		LibraryFactsFP: facts.Fingerprint(),
		CacheDir:       cacheDir,
		CacheWriter:    writer,
	})
	if err != nil {
		t.Fatalf("AndroidPhase.Run: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	return result.Findings.Len()
}

// TestAndroidCache_ModuleMinSdkChangeInvalidatesLayoutFindings changes one module's minSdk, which gates a layout
// rule, while the layout and every other module stay byte-identical.
func TestAndroidCache_ModuleMinSdkChangeInvalidatesLayoutFindings(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")
	writeRtlFile(t, filepath.Join(root, "settings.gradle.kts"), "include(\":a\")\ninclude(\":b\")\n")
	writeSdkModule(t, root, "a", 21)
	writeSdkModule(t, root, "b", 21)
	writeRtlFile(t, filepath.Join(root, "b", "src", "main", "res", "layout", "tap.xml"), keyboardClickableLayout)

	if got := runCachedKeyboardProject(t, root, cacheDir); got != 1 {
		t.Fatalf("minSdk 21: got %d findings, want 1", got)
	}
	if got := runCachedKeyboardProject(t, root, cacheDir); got != 1 {
		t.Fatalf("warm replay: got %d findings, want 1", got)
	}
	writeSdkModule(t, root, "b", 26)
	if got := runCachedKeyboardProject(t, root, cacheDir); got != 0 {
		t.Fatalf("minSdk 26 in module b: got %d findings, want 0 (stale cache replay)", got)
	}
	writeSdkModule(t, root, "b", 21)
	if got := runCachedKeyboardProject(t, root, cacheDir); got != 1 {
		t.Fatalf("minSdk back to 21: got %d findings, want 1", got)
	}
}

func countCacheEntries(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".bin") {
			n++
		}
		return nil
	})
	return n
}

// TestAndroidCache_BuildIdentityForcesMiss plays two krit builds against one cache: the same build reuses the
// entries it wrote, a different build writes its own instead of reading them.
func TestAndroidCache_BuildIdentityForcesMiss(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")
	writeRtlApp(t, root, "true", 21, 21)
	hash := func() string { return cache.ComputeCacheKeyHash([]string{"RtlHardcodedResource"}, nil, false) }

	buildid.OverrideTokenForTest(t, "rev:build-a")
	hashA := hash()
	runCachedRtlProject(t, root, cacheDir, hashA)
	written := countCacheEntries(t, cacheDir)
	if written == 0 {
		t.Fatal("first scan wrote no cache entries")
	}
	runCachedRtlProject(t, root, cacheDir, hashA)
	if got := countCacheEntries(t, cacheDir); got != written {
		t.Fatalf("second scan by the same build wrote %d new entries, want a full cache hit", got-written)
	}

	buildid.OverrideTokenForTest(t, "rev:build-b")
	hashB := hash()
	if hashA == hashB {
		t.Fatal("rule hash does not depend on the build identity")
	}
	runCachedRtlProject(t, root, cacheDir, hashB)
	if got := countCacheEntries(t, cacheDir); got != 2*written {
		t.Fatalf("scan by another build left %d entries, want %d: it must miss and write its own", got, 2*written)
	}
}

const rtlStartLayout = `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:textAlignment="viewStart"/>
</LinearLayout>
`

// runCachedRtlEnabledManifests returns the manifests RtlEnabledManifest reports, scanning with the cache on.
func runCachedRtlEnabledManifests(t *testing.T, root, cacheDir string) []string {
	t.Helper()
	rule := findV2RuleForTest(t, "RtlEnabledManifest")
	writer := scanner.NewAndroidCacheWriter(2)
	result, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
		Project:     android.DetectProject([]string{root}),
		ActiveRules: []*api.Rule{rule},
		Dispatcher:  rules.NewDispatcher([]*api.Rule{rule}, nil),
		RuleHash:    "fixed-rule-hash",
		CacheDir:    cacheDir,
		CacheWriter: writer,
	})
	if err != nil {
		t.Fatalf("AndroidPhase.Run: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	var files []string
	for _, f := range result.Findings.Findings() {
		rel, _ := filepath.Rel(root, f.File)
		files = append(files, filepath.ToSlash(rel))
	}
	sort.Strings(files)
	return files
}

// TestAndroidCache_IdenticalManifestsKeepTheirOwnFindings scans two modules whose manifests are byte-identical stubs
// but whose layouts differ. A manifest's findings depend on its path and on the res/ files beside it, so neither
// module may read the other's cached result, and a layout edit alone must change the manifest findings.
func TestAndroidCache_IdenticalManifestsKeepTheirOwnFindings(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")
	writeRtlFile(t, filepath.Join(root, "settings.gradle.kts"), "include(\":app\")\ninclude(\":a\")\ninclude(\":b\")\n")
	writeRtlFile(t, filepath.Join(root, "app", "build.gradle.kts"), "plugins { id(\"com.android.application\") }\ndependencies {\n  implementation(project(\":a\"))\n  implementation(project(\":b\"))\n}\n")
	writeRtlFile(t, filepath.Join(root, "app", "src", "main", "AndroidManifest.xml"), rtlManifestBody("unset", 21, 21, true))
	for _, mod := range []string{"a", "b"} {
		writeRtlFile(t, filepath.Join(root, mod, "build.gradle.kts"), "plugins { id(\"com.android.library\") }\n")
		writeRtlFile(t, filepath.Join(root, mod, "src", "main", "AndroidManifest.xml"), "<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" />\n")
	}
	layout := func(mod, body string) string {
		return filepath.Join(root, mod, "src", "main", "res", "layout", "main.xml")
	}
	writeRtlFile(t, layout("a", ""), rtlStartLayout)
	writeRtlFile(t, layout("b", ""), rtlEmptyLayout)

	want := []string{"a/src/main/AndroidManifest.xml"}
	for _, step := range []string{"cold", "warm"} {
		if got := runCachedRtlEnabledManifests(t, root, cacheDir); !slices.Equal(got, want) {
			t.Fatalf("%s scan reports %v, want %v", step, got, want)
		}
	}

	writeRtlFile(t, layout("b", ""), rtlStartLayout)
	want = []string{"a/src/main/AndroidManifest.xml", "b/src/main/AndroidManifest.xml"}
	if got := runCachedRtlEnabledManifests(t, root, cacheDir); !slices.Equal(got, want) {
		t.Fatalf("after editing only b's layout, scan reports %v, want %v", got, want)
	}
}
