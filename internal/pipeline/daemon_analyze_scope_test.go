package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
)

type stubDaemon struct {
	allCalls, filesCalls int
}

func (s *stubDaemon) AnalyzeAll() (*oracle.Data, error) {
	s.allCalls++
	return &oracle.Data{Version: 1, Files: map[string]*oracle.File{}}, nil
}

func (s *stubDaemon) AnalyzeFiles([]string) (*oracle.Data, error) {
	s.filesCalls++
	return &oracle.Data{Version: 1, Files: map[string]*oracle.File{}}, nil
}

// daemonAnalyze is where the scope gate is enforced: a types.json stamped under
// other Kotlin settings must not take a partial merge, the run falls back to a
// full analysis and stamps the replacement, and a types.json from this scope
// keeps the partial path.
func TestDaemonAnalyzeGatesThePartialPathOnTheRunsScope(t *testing.T) {
	root := t.TempDir()
	jar := filepath.Join(root, "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jar)
	scanPaths := []string{root}
	typesPath := oracle.CachePath(scanPaths)
	if err := os.MkdirAll(filepath.Dir(typesPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := oracle.WriteTypesJSON(typesPath, &oracle.Data{Version: 1, Files: map[string]*oracle.File{}}); err != nil {
		t.Fatal(err)
	}
	run := func(kt gradlemodel.KotlinSettings) IndexInput {
		return IndexInput{
			OracleBackend:    oracle.BackendFIR,
			OracleKotlin:     kt,
			StaleOraclePaths: []string{filepath.Join(root, "A.kt")},
		}
	}
	scopeFor := func(in IndexInput) oracle.StoreScope {
		s, ok := oracleTypesScope(in, scanPaths)
		if !ok {
			t.Fatal("no scope for the test jar")
		}
		return s
	}
	old := run(gradlemodel.KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0"})
	cur := run(gradlemodel.KotlinSettings{LanguageVersion: "2.2", APIVersion: "2.2"})
	recordTypesScope(typesPath, scopeFor(old), true, scanPaths, t.Logf)

	d := &stubDaemon{}
	if got := daemonAnalyze(cur, d, scanPaths, perf.New(false)); got == nil {
		t.Fatal("no data")
	}
	if d.filesCalls != 0 || d.allCalls != 1 {
		t.Fatalf("settings-only change took the partial path: files=%d all=%d", d.filesCalls, d.allCalls)
	}
	if !partialMergeAllowed(typesPath, scopeFor(cur), true, scanPaths) {
		t.Fatal("the full fallback result was not stamped with the run's scope")
	}

	d = &stubDaemon{}
	if got := daemonAnalyze(cur, d, scanPaths, perf.New(false)); got == nil {
		t.Fatal("no data")
	}
	if d.filesCalls != 1 || d.allCalls != 0 {
		t.Fatalf("in-scope types.json lost the partial path: files=%d all=%d", d.filesCalls, d.allCalls)
	}
}
