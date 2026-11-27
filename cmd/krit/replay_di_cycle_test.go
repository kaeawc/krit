package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A warm run that re-dispatches only the edited file still runs the
// project-scope rules on the whole project: DiCycleDetection reports a
// four-file cycle on Alpha.kt whichever file of the cycle the edit touched,
// and drops it once the cycle breaks. Each warm run must match a fresh one,
// and the body-only edits must take the replay (#1833).
func TestWarmReplayKeepsDICycleAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("App.kt", "package digraph\n\nannotation class Inject\nannotation class Component\n\n@Component\ninterface AppComponent {\n    fun alpha(): Alpha\n}\n")
	write("Alpha.kt", "package digraph\n\nclass Alpha @Inject constructor(val beta: Beta) {\n    fun f(): Int = 1\n}\n")
	write("Beta.kt", "package digraph\n\nclass Beta @Inject constructor(val gamma: Gamma) {\n    fun g(): Int = 1\n}\n")
	write("Gamma.kt", "package digraph\n\nclass Gamma @Inject constructor(val alpha: Alpha)\n")

	scan := func(extra ...string) (string, string) {
		t.Helper()
		args := append([]string{"--no-fir", "--no-daemon", "--no-type-oracle", "--enable-rules", "DiCycleDetection", "-f", "json", "-q", "--perf"}, extra...)
		stdout, stderr, code := runKrit(t, append(args, dir)...)
		if code > 1 {
			t.Fatalf("krit %v exited %d: %s", extra, code, stderr)
		}
		var report struct {
			Findings []scopeFinding    `json:"findings"`
			Perf     []json.RawMessage `json:"perfTiming"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("invalid JSON for %v: %v", extra, err)
		}
		var keys []string
		for _, f := range report.Findings {
			if f.Rule == "DiCycleDetection" {
				keys = append(keys, fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line))
			}
		}
		sort.Strings(keys)
		return strings.Join(keys, " "), dispatchPathIn(report.Perf)
	}
	scan()
	steps := []struct {
		name, file, content string
		wantPath            string // "" when any path will do
	}{
		{"body edit in Alpha.kt", "Alpha.kt", "package digraph\n\nclass Alpha @Inject constructor(val beta: Beta) {\n    fun f(): Int = 2\n}\n", "cacheHitDeltaBundle"},
		{"body edit in Beta.kt", "Beta.kt", "package digraph\n\nclass Beta @Inject constructor(val gamma: Gamma) {\n    fun g(): Int = 2\n}\n", "cacheHitDeltaBundle"},
		{"cycle broken in Gamma.kt", "Gamma.kt", "package digraph\n\nclass Gamma @Inject constructor()\n", ""},
	}
	for _, s := range steps {
		write(s.file, s.content)
		warm, path := scan()
		fresh, _ := scan("--no-cache")
		if warm != fresh {
			t.Fatalf("%s: warm DiCycleDetection [%s] (dispatch %s), fresh [%s]", s.name, warm, path, fresh)
		}
		if s.wantPath != "" && path != s.wantPath {
			t.Fatalf("%s: dispatch path %q, want %q; the test no longer exercises the replay", s.name, path, s.wantPath)
		}
	}
}

// dispatchPathIn returns the "path" of the dispatchPath perf entry.
func dispatchPathIn(entries []json.RawMessage) string {
	for _, raw := range entries {
		var e struct {
			Name       string            `json:"name"`
			Attributes map[string]string `json:"attributes"`
			Children   []json.RawMessage `json:"children"`
		}
		if json.Unmarshal(raw, &e) != nil {
			continue
		}
		if e.Name == "dispatchPath" {
			return e.Attributes["path"]
		}
		if p := dispatchPathIn(e.Children); p != "" {
			return p
		}
	}
	return ""
}
