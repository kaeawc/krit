package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Regression tests for the findings-bundle cache ignoring XML inputs: layouts,
// manifests and navigation graphs feed class references into the cross-file
// index, but the bundle fingerprint hashed only Kotlin/Java content, so an
// XML-only edit replayed the prior bundle's findings.

const (
	bundleXMLHelperKt = "app/src/main/java/com/example/Helper.kt"
	bundleXMLLayout   = "app/src/main/res/layout/main.xml"

	bundleXMLWithHelper    = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<fragment xmlns:android=\"http://schemas.android.com/apk/res/android\"\n    android:name=\"com.example.Helper\" />\n"
	bundleXMLWithoutHelper = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<FrameLayout xmlns:android=\"http://schemas.android.com/apk/res/android\" />\n"
)

// bundleXMLDaemon wires the host the way internal/cli/serve does for the
// pieces that matter here: the resident XML loader, the stats-clean memo, the
// resident bundle slot, the watcher-reported dirty set and the prior manifest
// facts, all backed by one WorkspaceState.
type bundleXMLDaemon struct {
	t         *testing.T
	dir       string
	cacheRoot string
	ws        *WorkspaceState
	rule      *api.Rule
	calls     *int64
	started   bool
	tick      int
}

func newBundleXMLDaemon(t *testing.T, files map[string]string) *bundleXMLDaemon {
	t.Helper()
	d := &bundleXMLDaemon{t: t, dir: t.TempDir(), cacheRoot: t.TempDir(), calls: new(int64)}
	calls := d.calls
	d.ws = NewWorkspaceState(d.dir)
	d.write(bundleXMLHelperKt, "package com.example\n\nclass Helper\n")
	for name, body := range files {
		d.write(name, body)
	}
	d.rule = api.FakeRule("DeadHelper", api.WithNeeds(api.NeedsCrossFile), api.WithCheck(func(ctx *api.Context) {
		atomic.AddInt64(calls, 1)
		for _, sym := range ctx.CodeIndex.SymbolsNamed("Helper") {
			if !ctx.CodeIndex.IsReferencedOutsideFile("Helper", sym.File) {
				ctx.Emit(scanner.Finding{File: sym.File, Line: 1, Col: 1, Message: "unused"})
			}
		}
	}))
	return d
}

func (d *bundleXMLDaemon) write(name, body string) {
	d.t.Helper()
	path := filepath.Join(d.dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		d.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		d.t.Fatal(err)
	}
	// Distinct mtimes: same-size back-to-back writes can share a kernel tick,
	// and the content-hash memo is keyed on (size, mtime).
	d.tick++
	mtime := time.Unix(1_700_000_000+int64(d.tick), 0)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		d.t.Fatal(err)
	}
}

// editXML rewrites an XML file and does what the daemon's file watcher does
// for an .xml event.
func (d *bundleXMLDaemon) editXML(name, body string) {
	d.t.Helper()
	d.write(name, body)
	d.ws.Touch(filepath.Join(d.dir, name))
	d.ws.BumpSourceMTimeVersion()
	d.ws.BumpXMLFilesVersion()
}

func (d *bundleXMLDaemon) run() ProjectResult {
	d.t.Helper()
	host := ProjectHostState{
		ResidentFiles:           d.ws,
		XMLFilesLoader:          d.ws.XMLFiles,
		FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
		FindingsBundleCacheRoot: d.cacheRoot,
		DaemonCaches: DaemonCaches{
			BundleStatsClean:     d.ws.BundleStatsClean,
			MarkBundleStatsClean: d.ws.MarkBundleStatsClean,
			SourceMTimeVersion:   d.ws.SourceMTimeVersion,
			ResidentBundle:       d.ws.ResidentBundle,
			StoreResidentBundle:  d.ws.StoreResidentBundle,
		},
	}
	manifestKey := scanner.FindingsBundleManifestKey(d.cacheRoot, []string{d.dir})
	if m, ok := scanner.LoadFindingsBundleManifest(d.cacheRoot, manifestKey); ok {
		host.PriorContentHashes = m.ContentHashes
		host.PriorStructuralFPs = m.StructuralFPs
		host.PriorOracleBlobHashes = m.OracleBlobHashes
		host.PriorAbiHashes = m.AbiHashes
		host.PriorFileStats = m.FileStats
	}
	if d.started {
		host.SourceSetDirty = d.ws.DrainDirty()
		if host.SourceSetDirty == nil {
			host.SourceSetDirty = []string{}
		}
	}
	d.started = true
	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{d.dir},
			ActiveRules: []*api.Rule{d.rule},
			Format:      "json",
			Version:     "test",
		},
		Host: host,
	})
	if err != nil {
		d.t.Fatalf("RunProject: %v", err)
	}
	return res
}

// wantFindings runs the daemon-style host and checks the count against both
// want and a cache-free run over the same tree.
func (d *bundleXMLDaemon) wantFindings(label string, want int) {
	d.t.Helper()
	got := d.run().FindingsCount
	clean, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{d.dir},
			ActiveRules: []*api.Rule{d.rule},
			Format:      "json",
			Version:     "test",
		},
	})
	if err != nil {
		d.t.Fatalf("clean RunProject: %v", err)
	}
	if clean.FindingsCount != want {
		d.t.Fatalf("%s: clean findings = %d, want %d (test premise)", label, clean.FindingsCount, want)
	}
	if got != want {
		d.t.Errorf("%s: daemon findings = %d, want %d (clean run agrees)", label, got, want)
	}
}

func TestRunProject_BundleReplayHonorsXMLEdit_Daemon(t *testing.T) {
	d := newBundleXMLDaemon(t, map[string]string{bundleXMLLayout: bundleXMLWithHelper})
	d.wantFindings("cold", 0)
	before := atomic.LoadInt64(d.calls)
	d.wantFindings("warm unchanged", 0)
	// wantFindings also does a cache-free run, which calls the rule once.
	if got := atomic.LoadInt64(d.calls) - before; got != 1 {
		t.Errorf("unchanged XML must still be a full bundle hit; rule calls = %d, want 1 (the clean run)", got)
	}

	d.editXML(bundleXMLLayout, bundleXMLWithoutHelper)
	d.wantFindings("after reference removed", 1)
	d.wantFindings("warm again", 1)

	d.editXML(bundleXMLLayout, bundleXMLWithHelper)
	d.wantFindings("after reference restored", 0)
	d.wantFindings("warm after restore", 0)
}

// xmlInputsFingerprint must move for every XML input class the bundle's rules
// read (class-reference layouts, manifests, values resources) and stay put
// while they are unchanged.
func TestXMLInputsFingerprint_TracksXMLInputs(t *testing.T) {
	dir := t.TempDir()
	// Each write gets a distinct mtime: the content-hash memo is keyed on
	// (size, mtime), and back-to-back same-size writes can share a kernel tick.
	tick := 0
	write := func(rel, body string) string {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		tick++
		mtime := time.Unix(1_700_000_000+int64(tick), 0)
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
		return path
	}
	kt := write("app/src/main/java/com/example/A.kt", "package com.example\n\nclass A\n")
	manifest := write("app/src/main/AndroidManifest.xml", "<manifest package=\"a\"/>")
	write("app/src/main/res/layout/main.xml", bundleXMLWithoutHelper)
	write("app/src/main/res/values/strings.xml", "<resources><string name=\"a\">A</string></resources>")
	project := &android.Project{
		ManifestPaths: []string{manifest},
		ResDirs:       []string{filepath.Join(dir, "app/src/main/res")},
	}
	host := ProjectHostState{}
	fp := func() string { return xmlInputsFingerprint(host, project, []string{kt}) }

	base := fp()
	if base == "" {
		t.Fatal("fingerprint must be non-empty for a project with XML")
	}
	if again := fp(); again != base {
		t.Fatal("fingerprint changed without an XML edit")
	}
	for _, tc := range []struct{ name, path, body string }{
		{"layout", "app/src/main/res/layout/main.xml", bundleXMLWithHelper},
		{"manifest", "app/src/main/AndroidManifest.xml", "<manifest package=\"b\"/>"},
		{"values resource", "app/src/main/res/values/strings.xml", "<resources><string name=\"b\">B</string></resources>"},
	} {
		write(tc.path, tc.body)
		if got := fp(); got == base {
			t.Errorf("%s edit did not change the XML fingerprint", tc.name)
		}
		// Restore the original content for the next case.
		switch tc.name {
		case "layout":
			write(tc.path, bundleXMLWithoutHelper)
		case "manifest":
			write(tc.path, "<manifest package=\"a\"/>")
		default:
			write(tc.path, "<resources><string name=\"a\">A</string></resources>")
		}
		if got := fp(); got != base {
			t.Errorf("%s: fingerprint did not return to base after restoring content", tc.name)
		}
	}
	if got := xmlInputsFingerprint(host, nil, []string{kt}); got == "" {
		t.Error("layout XML alone must still produce a fingerprint")
	}
	if got := xmlInputsFingerprint(host, nil, nil); got != "" {
		t.Errorf("no Kotlin roots and no Android project: fingerprint = %q, want empty", got)
	}
}
