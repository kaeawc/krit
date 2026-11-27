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
