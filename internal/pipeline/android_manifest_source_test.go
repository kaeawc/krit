package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	msManifestRel = "app/src/main/AndroidManifest.xml"
	msSourceRel   = "app/src/main/java/com/example/Marker.kt"

	msManifestWithService    = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" package=\"com.example\">\n    <application>\n        <service android:name=\"com.example.Marker\" />\n    </application>\n</manifest>\n"
	msManifestWithoutService = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" package=\"com.example\">\n    <application />\n</manifest>\n"

	msSourceMarker = "package com.example\n\nclass Marker {\n    fun run() = Unit\n}\n"
	msSourcePlain  = "package com.example\n\nclass Plain {\n    fun run() = Unit\n}\n"
)

// manifestSourceJoinRule reports every source file that declares a class the
// manifests register as a <service>: its findings depend on both inputs.
func manifestSourceJoinRule(calls *int64) *api.Rule {
	r := api.FakeRule("ManifestSourceJoin", api.WithNeeds(api.NeedsManifest|api.NeedsParsedFiles), api.WithCheck(func(ctx *api.Context) {
		atomic.AddInt64(calls, 1)
		services := map[string]bool{}
		for _, m := range ctx.Manifests {
			if m.Root == nil {
				continue
			}
			for _, app := range m.Root.ChildrenByTag("application") {
				for _, svc := range app.ChildrenByTag("service") {
					name := svc.AttrValue("android:name")
					services[name[strings.LastIndex(name, ".")+1:]] = true
				}
			}
		}
		for _, f := range ctx.ParsedFiles {
			for name := range services {
				if strings.Contains(string(f.Content), "class "+name+" ") {
					ctx.Emit(scanner.Finding{File: f.Path, Line: 3, Col: 1, Message: "registered service " + name})
				}
			}
		}
	}))
	r.Languages = []scanner.Language{scanner.LangKotlin, scanner.LangJava}
	return r
}

type manifestSourceProject struct {
	t    *testing.T
	dir  string
	tick int
}

func newManifestSourceProject(t *testing.T, manifestBody, sourceBody string) *manifestSourceProject {
	p := &manifestSourceProject{t: t, dir: t.TempDir()}
	p.write(msManifestRel, manifestBody)
	p.write(msSourceRel, sourceBody)
	return p
}

func (p *manifestSourceProject) path(rel string) string { return filepath.Join(p.dir, rel) }

func (p *manifestSourceProject) write(rel, body string) {
	p.t.Helper()
	path := p.path(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		p.t.Fatal(err)
	}
	// Distinct mtimes: the content-hash memo is keyed on (size, mtime).
	p.tick++
	mtime := time.Unix(1_700_000_000+int64(p.tick), 0)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		p.t.Fatal(err)
	}
}

// runPhase runs AndroidPhase with the given source files and cache dir, the
// way runAndroidPhase does.
func (p *manifestSourceProject) runPhase(rule *api.Rule, cacheDir string, parsed bool) AndroidResult {
	p.t.Helper()
	var files []*scanner.File
	if parsed {
		files = []*scanner.File{parseKotlinForTest(p.t, p.path(msSourceRel), readFile(p.t, p.path(msSourceRel)))}
	}
	writer := scanner.NewAndroidCacheWriter(2)
	res, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
		Project:        &android.Project{ManifestPaths: []string{p.path(msManifestRel)}},
		ActiveRules:    []*api.Rule{rule},
		Dispatcher:     rules.NewDispatcher([]*api.Rule{rule}, nil),
		SourceFiles:    files,
		SourcePaths:    []string{p.path(msSourceRel)},
		RuleHash:       "test-rule-hash",
		LibraryFactsFP: "test-lf",
		CacheDir:       cacheDir,
		CacheWriter:    writer,
	})
	if err != nil {
		p.t.Fatalf("AndroidPhase.Run: %v", err)
	}
	if err := writer.Close(); err != nil {
		p.t.Fatalf("flush writer: %v", err)
	}
	return res
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestManifestSourcePass_CachesAndInvalidatesOnEitherInput(t *testing.T) {
	p := newManifestSourceProject(t, msManifestWithService, msSourceMarker)
	cacheDir := filepath.Join(p.dir, ".krit", "android-findings-cache")
	var calls int64
	rule := manifestSourceJoinRule(&calls)

	step := func(label string, parsed bool, wantFindings int, wantCalls int64) {
		t.Helper()
		hashutil.ResetDefault()
		before := atomic.LoadInt64(&calls)
		res := p.runPhase(rule, cacheDir, parsed)
		if got := res.Findings.Len(); got != wantFindings {
			t.Fatalf("%s: findings = %d, want %d", label, got, wantFindings)
		}
		if got := atomic.LoadInt64(&calls) - before; got != wantCalls {
			t.Fatalf("%s: rule calls = %d, want %d", label, got, wantCalls)
		}
	}

	step("cold", true, 1, 1)
	step("warm unchanged", true, 1, 0)
	// A warm run whose parse phase skipped every source file still serves
	// the cached result keyed on the on-disk hashes.
	step("warm unparsed", false, 1, 0)

	p.write(msManifestRel, msManifestWithoutService)
	step("manifest edit", true, 0, 1)
	step("manifest edit warm", true, 0, 0)

	p.write(msManifestRel, msManifestWithService)
	step("manifest restored", true, 1, 0)

	p.write(msSourceRel, msSourcePlain)
	step("source edit", true, 0, 1)
	// The parse phase left the edited file unparsed: the pass parses it
	// on demand rather than running with a partial source set.
	p.write(msSourceRel, msSourceMarker+"\n")
	step("source edit unparsed", false, 1, 1)
}

// TestManifestSourcePass_InvalidatesOnGroovyBuildScriptEdit pins #1625 review: ProviderReadPermissionOnly reads the
// app's Groovy build.gradle (not a source file) to decide whether a library provider reaches the merged manifest, so
// the manifest-source key must cover the project's build scripts. Every warm result must equal a cache-free run.
func TestManifestSourcePass_InvalidatesOnGroovyBuildScriptEdit(t *testing.T) {
	const (
		appBuildRel = "app/build.gradle"
		libBuildRel = "lib/build.gradle.kts"
		settingsRel = "settings.gradle.kts"
		appManRel   = "app/src/main/AndroidManifest.xml"
		libManRel   = "lib/src/main/AndroidManifest.xml"
		providerRel = "lib/src/main/java/com/example/lib/NotesProvider.kt"
		depends     = "plugins { id 'com.android.application' }\n\ndependencies {\n    implementation project(':lib')\n}\n"
		independent = "plugins { id 'com.android.application' }\n"
	)
	p := &manifestSourceProject{t: t, dir: t.TempDir()}
	p.write(settingsRel, "include(\":app\", \":lib\")\n")
	p.write(appBuildRel, depends)
	p.write(libBuildRel, "plugins { id(\"com.android.library\") }\n")
	p.write(appManRel, "<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" package=\"com.example.app\">\n    <application />\n</manifest>\n")
	p.write(libManRel, "<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" package=\"com.example.lib\">\n    <application>\n        <provider android:name=\".NotesProvider\" android:authorities=\"a\" android:readPermission=\"com.example.READ\" />\n    </application>\n</manifest>\n")
	p.write(providerRel, `package com.example.lib

import android.content.ContentProvider
import android.content.ContentValues
import android.net.Uri

class NotesProvider : ContentProvider() {
    override fun insert(uri: Uri, values: ContentValues?): Uri? = uri
}
`)
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "ProviderReadPermissionOnly" {
			rule = r
		}
	}
	if rule == nil {
		t.Fatal("ProviderReadPermissionOnly is not registered")
	}
	cacheDir := filepath.Join(p.dir, ".krit", "android-findings-cache")
	run := func(cacheDir string) int {
		t.Helper()
		hashutil.ResetDefault()
		writer := scanner.NewAndroidCacheWriter(2)
		res, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
			Project: &android.Project{
				ManifestPaths: []string{p.path(appManRel), p.path(libManRel)},
				GradlePaths:   []string{p.path(settingsRel), p.path(appBuildRel), p.path(libBuildRel)},
			},
			ActiveRules:    []*api.Rule{rule},
			Dispatcher:     rules.NewDispatcher([]*api.Rule{rule}, nil),
			SourceFiles:    []*scanner.File{parseKotlinForTest(t, p.path(providerRel), readFile(t, p.path(providerRel)))},
			SourcePaths:    []string{p.path(providerRel)},
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
	step := func(label string, want int) {
		t.Helper()
		if got := run(""); got != want {
			t.Fatalf("%s: cache-free findings = %d, want %d", label, got, want)
		}
		if got := run(cacheDir); got != want {
			t.Fatalf("%s: warm findings = %d, cache-free %d", label, got, want)
		}
	}
	step("cold", 1)
	step("warm unchanged", 1)
	p.write(appBuildRel, independent)
	step("dependency removed", 0)
	p.write(appBuildRel, depends)
	step("dependency restored", 1)
}

func TestManifestSourceRules_SkipTheCrossFilePhase(t *testing.T) {
	var calls int64
	rule := manifestSourceJoinRule(&calls)
	serial, concurrent := splitConcurrentCrossRules([]*api.Rule{rule})
	if len(serial)+len(concurrent) != 0 {
		t.Fatalf("manifest-source rule must not run in the cross-file phase (serial=%d concurrent=%d)", len(serial), len(concurrent))
	}
	if _, hasParsed := (CrossFilePhase{}).classifyCrossFileNeeds([]*api.Rule{rule}); hasParsed {
		t.Fatal("manifest-source rule must not request a cross-file parsed-files pass")
	}
}

// TestRunProject_ManifestSourceRuleTracksEdits_Daemon drives RunProject with
// the daemon host wiring and checks every warm result against a cache-free run
// after manifest and source edits, including a body-only source edit that
// takes the delta path.
func TestRunProject_ManifestSourceRuleTracksEdits_Daemon(t *testing.T) {
	d := newBundleXMLDaemon(t, map[string]string{
		msManifestRel: msManifestWithService,
		msSourceRel:   msSourceMarker,
	})
	var calls int64
	d.rule = manifestSourceJoinRule(&calls)
	// editSource does what the daemon's file watcher does for a .kt write.
	editSource := func(body string) {
		path := filepath.Join(d.dir, msSourceRel)
		d.write(msSourceRel, body)
		d.ws.BumpSourceMTimeVersion()
		d.ws.Invalidate(path)
		d.ws.InvalidateCodeIndex()
		d.ws.InvalidateDependents()
		d.ws.InvalidateResolver()
		d.ws.Touch(path)
	}

	d.wantFindings("cold", 1)
	d.wantFindings("warm unchanged", 1)

	d.editXML(msManifestRel, msManifestWithoutService)
	d.wantFindings("manifest edit", 0)
	d.wantFindings("manifest edit warm", 0)

	d.editXML(msManifestRel, msManifestWithService)
	d.wantFindings("manifest restored", 1)

	// Body-only edit: same declarations, different body.
	editSource(strings.Replace(msSourceMarker, "= Unit", "= println()", 1))
	d.wantFindings("body edit", 1)

	editSource(msSourcePlain)
	d.wantFindings("source edit", 0)
	d.wantFindings("source edit warm", 0)

	editSource(msSourceMarker)
	d.wantFindings("source restored", 1)
}

// TestRunProject_ManifestSourceRuleTracksEdits_CLICaches repeats the edit
// sequence with the CLI's on-disk caches (analysis, cross-file, Android).
func TestRunProject_ManifestSourceRuleTracksEdits_CLICaches(t *testing.T) {
	p := newManifestSourceProject(t, msManifestWithService, msSourceMarker)
	cacheRoot := t.TempDir()
	var calls int64
	rule := manifestSourceJoinRule(&calls)
	run := func(label string, want int) {
		t.Helper()
		hashutil.ResetDefault()
		writer := scanner.NewAndroidCacheWriter(2)
		res, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{
				Config:      config.NewConfig(),
				Paths:       []string{p.dir},
				ActiveRules: []*api.Rule{rule},
				Format:      "json",
				Version:     "test",
			},
			Host: ProjectHostState{
				AndroidCacheDir:    filepath.Join(cacheRoot, "android"),
				AndroidCacheWriter: writer,
				CrossFileCacheDir:  filepath.Join(cacheRoot, "cross"),
			},
		})
		if err != nil {
			t.Fatalf("%s: RunProject: %v", label, err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if res.FindingsCount != want {
			t.Fatalf("%s: findings = %d, want %d", label, res.FindingsCount, want)
		}
	}
	run("cold", 1)
	run("warm", 1)
	p.write(msManifestRel, msManifestWithoutService)
	run("manifest edit", 0)
	p.write(msManifestRel, msManifestWithService)
	run("manifest restored", 1)
	p.write(msSourceRel, msSourcePlain)
	run("source edit", 0)
	p.write(msSourceRel, msSourceMarker)
	run("source restored", 1)
}
