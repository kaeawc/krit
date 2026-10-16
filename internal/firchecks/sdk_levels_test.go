package firchecks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func writeBuildGradle(t *testing.T, path string, minSdk, targetSdk int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("android {\n    defaultConfig {\n        minSdkVersion %d\n        targetSdkVersion %d\n    }\n}\n", minSdk, targetSdk)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The pass resolves each requested file's SDK levels with the lookup the Go
// rules run on file.Path (android.ResolveSDKLevels on the scan's spelling)
// and sends them keyed by the requested path. A file under no Android module
// has no entry.
func TestRunPassSendsSDKLevels(t *testing.T) {
	p := newVerdictProject(t, map[string]string{
		"app/src/main/kotlin/A.kt": "package a\n",
		"plain/src/P.kt":           "package p\n",
	}, nil)
	writeBuildGradle(t, "app/build.gradle", 21, 33)
	checker := NewFakeFirChecker()
	RunPass(PassOptions{
		Enabled:     true,
		Checker:     checker,
		ActiveRules: []*api.Rule{{ID: verdictRule}},
		KotlinPaths: []string{"app/src/main/kotlin/A.kt", "plain/src/P.kt"},
	}, nil)
	if len(checker.CalledSDKLevels) != 1 {
		t.Fatalf("Check calls = %d, want 1", len(checker.CalledSDKLevels))
	}
	goRuleLevels := android.ResolveSDKLevels("app/src/main/kotlin/A.kt")
	if goRuleLevels != (android.SDKLevels{MinSdk: 21, TargetSdk: 33}) {
		t.Fatalf("ResolveSDKLevels = %+v", goRuleLevels)
	}
	want := map[string]android.SDKLevels{p.abs("app/src/main/kotlin/A.kt"): goRuleLevels}
	if got := checker.CalledSDKLevels[0]; !reflect.DeepEqual(got, want) {
		t.Fatalf("sdkLevels = %v, want %v (a file with no known level needs no entry)", got, want)
	}
}

func TestCheckSendsSDKLevelsOnTheWire(t *testing.T) {
	d, requests := fakeFirDaemon(t, `{"id":1,"succeeded":2,"skipped":0,"findings":[],"rules":[],"crashed":{}}`)
	files := []fileRef{{Path: "/p/app/src/A.kt"}, {Path: "/p/lib/src/L.kt"}}
	facts := FileFacts{SDKLevels: map[string]android.SDKLevels{
		"/p/app/src/A.kt": {MinSdk: 21, TargetSdk: 34},
		"/p/lib/src/L.kt": {TargetSdk: 30},
	}}
	if _, err := d.Check(files, nil, nil, []string{"AddJavascriptInterface"}, nil, facts); err != nil {
		t.Fatalf("Check: %v", err)
	}
	raw := <-requests
	var sent struct {
		SDKLevels map[string]map[string]int `json:"sdkLevels"`
	}
	if err := json.Unmarshal(raw, &sent); err != nil {
		t.Fatalf("request is not JSON: %v\n%s", err, raw)
	}
	want := map[string]map[string]int{
		"/p/app/src/A.kt": {"minSdk": 21, "targetSdk": 34},
		"/p/lib/src/L.kt": {"targetSdk": 30},
	}
	if !reflect.DeepEqual(sent.SDKLevels, want) {
		t.Fatalf("sdkLevels = %v, want %v\n%s", sent.SDKLevels, want, raw)
	}

	d, requests = fakeFirDaemon(t, `{"id":1,"succeeded":1,"skipped":0,"findings":[],"rules":[],"crashed":{}}`)
	if _, err := d.Check(files[:1], nil, nil, []string{"AddJavascriptInterface"}, nil, FileFacts{}); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if raw := <-requests; strings.Contains(string(raw), "sdkLevels") {
		t.Fatalf("sdkLevels must be omitted when no file has a known level: %s", raw)
	}
}

func TestFileFactsForFilesKeepsRequestedSDKLevels(t *testing.T) {
	facts := FileFacts{SDKLevels: map[string]android.SDKLevels{
		"/a/A.kt": {MinSdk: 21, TargetSdk: 34},
		"/a/B.kt": {MinSdk: 23},
	}}
	got := facts.forFiles([]string{"/a/B.kt", "/a/C.kt"})
	want := FileFacts{SDKLevels: map[string]android.SDKLevels{"/a/B.kt": {MinSdk: 23}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("forFiles = %+v, want %+v", got, want)
	}
}

func TestFirInvocationFingerprintIncludesSDKLevels(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	rules := []string{"ImplicitPendingIntent"}
	levels := func(minSdk, targetSdk int) FileFacts {
		return FileFacts{SDKLevels: map[string]android.SDKLevels{"/p/A.kt": {MinSdk: minSdk, TargetSdk: targetSdk}}}
	}
	none := FirInvocationFingerprint(nil, jar, rules, nil, FileFacts{})
	base := FirInvocationFingerprint(nil, jar, rules, nil, levels(21, 30))
	if none == base {
		t.Fatal("a known SDK level must change the fingerprint")
	}
	if base == FirInvocationFingerprint(nil, jar, rules, nil, levels(21, 31)) {
		t.Fatal("a different targetSdk must change the fingerprint")
	}
	if base == FirInvocationFingerprint(nil, jar, rules, nil, levels(23, 30)) {
		t.Fatal("a different minSdk must change the fingerprint")
	}
	if base != FirInvocationFingerprint(nil, jar, rules, nil, levels(21, 30)) {
		t.Fatal("the same SDK levels must keep the fingerprint")
	}
}

// build.gradle is not a compiled source, so nothing else in the fingerprint
// notices a targetSdk edit: the levels the pass resolves must miss the cache.
func TestInvokeCached_TargetSdkChangeMisses(t *testing.T) {
	tmp := t.TempDir()
	ktFile := filepath.Join(tmp, "app", "src", "main", "kotlin", "Intents.kt")
	buildFile := filepath.Join(tmp, "app", "build.gradle")
	writeBuildGradle(t, buildFile, 21, 30)
	if err := os.MkdirAll(filepath.Dir(ktFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ktFile, []byte("val intent = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := ContentHash(ktFile)
	if err != nil {
		t.Fatal(err)
	}
	rules := []string{"ImplicitPendingIntent"}
	files := []string{ktFile}
	facts := fileFactsOf(files, nil)
	if got := facts.SDKLevels[ktFile]; got != (android.SDKLevels{MinSdk: 21, TargetSdk: 30}) {
		t.Fatalf("resolved levels = %+v", got)
	}
	cacheDir, _ := CacheDir(tmp)
	if err := WriteCacheEntry(cacheDir, &FirCacheEntry{
		V: FirCacheVersion, ContentHash: hash, FilePath: ktFile, Rules: rules,
		ClosureFingerprint: CheckCacheFingerprint(nil, files, nil, "", rules, nil, facts),
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := InvokeCached("", files, nil, nil, rules, nil, fileFactsOf(files, nil), tmp, false, false); err != nil {
		t.Fatalf("unchanged build.gradle must hit the cache: %v", err)
	}

	writeBuildGradle(t, buildFile, 21, 31)
	edited := fileFactsOf(files, nil)
	if got := edited.SDKLevels[ktFile].TargetSdk; got != 31 {
		t.Fatalf("targetSdk after the edit = %d, want 31", got)
	}
	// A miss needs the jar, which "" cannot provide: the error proves the miss.
	_, err = InvokeCached("", files, nil, nil, rules, nil, edited, tmp, false, false)
	if err == nil || !strings.Contains(err.Error(), "krit-fir.jar not found") {
		t.Fatalf("a targetSdk edit must miss the cache, got err=%v", err)
	}

	writeBuildGradle(t, buildFile, 21, 30)
	if _, err := InvokeCached("", files, nil, nil, rules, nil, fileFactsOf(files, nil), tmp, false, false); err != nil {
		t.Fatalf("restoring the targetSdk must hit the cache again: %v", err)
	}
}
