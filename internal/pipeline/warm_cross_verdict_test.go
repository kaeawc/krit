package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// The parse phase asks for the warm cross-findings verdict again; a plan that
// recorded one answers without re-running the check (#1850). This host has
// no cache directories, so a re-run would refuse.
func TestWarmCrossVerdictReusesThePlansVerdict(t *testing.T) {
	args := ProjectArgs{ActiveRules: []*api.Rule{api.FakeRule("Cross", api.WithNeeds(api.NeedsCrossFile))}}
	cols := &scanner.FindingColumns{}
	recorded := warmAnalysisCachePlan{crossChecked: true, crossAllowed: true, cross: cols}
	if ok, got := warmCrossVerdict(args, ProjectHostState{}, recorded); !ok || got != cols {
		t.Fatalf("recorded verdict: got (%v, %p), want (true, %p)", ok, got, cols)
	}
	if ok, _ := warmCrossVerdict(args, ProjectHostState{}, warmAnalysisCachePlan{}); ok {
		t.Fatal("without a recorded verdict the check runs, and this host can only refuse")
	}
}

// The parallel hashing gives what hashing each file in turn does, an
// unreadable file included.
func TestHashSourceFilesMatchesSerial(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for i := 0; i < 50; i++ {
		p := filepath.Join(dir, "F"+string(rune('a'+i%26))+string(rune('a'+i/26))+".java")
		if err := os.WriteFile(p, []byte("class F"+p+" {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	paths = append(paths, filepath.Join(dir, "Missing.java"))
	got := hashSourceFiles(paths)
	if len(got) != len(paths) {
		t.Fatalf("hashed %d of %d paths", len(got), len(paths))
	}
	for _, p := range paths {
		if want := hashSourceFile(p); got[p] != want {
			t.Fatalf("%s: parallel %q, serial %q", p, got[p], want)
		}
	}
	if got[filepath.Join(dir, "Missing.java")] != "unreadable-source-file" {
		t.Fatal("an unreadable file must hash to the refusing sentinel")
	}
}

// A warm run checks cross-findings reuse once, refused or not (#1850): the
// plan, the parse-skip gate and the parse phase used to run the check three
// times, each hashing every Java file outside the per-file set.
func TestRunProject_WarmCrossFindingsCheckedOncePerRun(t *testing.T) {
	h := newWarmCrossHarness(t, "CrossRule", map[string]string{
		"A.kt":   "package test\n\nclass A\n",
		"B.java": "package test;\n\nclass B {}\n",
	})
	h.javaPaths = []string{filepath.Join(h.dir, "B.java")}
	var checks int
	testHookLoadWarmCrossFindings = func() { checks++ }
	t.Cleanup(func() { testHookLoadWarmCrossFindings = nil })

	h.wantFindings("cold", 2, false)
	checks = 0
	h.wantFindings("warm", 2, true)
	if checks != 1 {
		t.Fatalf("warm run checked cross-findings reuse %d times, want 1", checks)
	}
	// A Java edit leaves every Kotlin file a per-file cache hit but refuses
	// reuse: the index no longer matches B.java.
	h.write("B.java", "package test;\n\nclass B { int x; }\n")
	checks = 0
	h.wantFindings("java edit", 2, false)
	if checks != 1 {
		t.Fatalf("refusing run checked cross-findings reuse %d times, want 1", checks)
	}
}
