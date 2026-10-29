package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

// dirDaemon records the incremental directory its full analysis was given.
type dirDaemon struct {
	stubDaemon
	dirs []string
}

func (d *dirDaemon) AnalyzeAllIn(dir string) (*oracle.Data, error) {
	d.dirs = append(d.dirs, dir)
	return d.AnalyzeAll()
}

// #957: an oracle-only full analysis names krit-fir's bundle directory, so a
// restarted daemon can answer an unchanged module without compiling it.
func TestAnalyzeAllPlainNamesTheIncrementalDirOnlyWhenSet(t *testing.T) {
	d := &dirDaemon{}
	if _, err := analyzeAllPlain(IndexInput{FIRIncrementalDir: "/repo/.krit/fir-incremental"}, d); err != nil {
		t.Fatal(err)
	}
	if len(d.dirs) != 1 || d.dirs[0] != "/repo/.krit/fir-incremental" {
		t.Fatalf("dirs = %v, want the incremental dir once", d.dirs)
	}
	if _, err := analyzeAllPlain(IndexInput{}, d); err != nil {
		t.Fatal(err)
	}
	if len(d.dirs) != 1 || d.allCalls != 2 {
		t.Fatalf("without a dir: dirs = %v, allCalls = %d; want the plain request", d.dirs, d.allCalls)
	}

	// A daemon that cannot name the directory still gets the plain request.
	plain := &stubDaemon{}
	if _, err := analyzeAllPlain(IndexInput{FIRIncrementalDir: "/repo/.krit/fir-incremental"}, plain); err != nil {
		t.Fatal(err)
	}
	if plain.allCalls != 1 {
		t.Fatalf("allCalls = %d, want 1", plain.allCalls)
	}
}

var _ incrementalDirAnalyzer = (*oracle.Daemon)(nil)
