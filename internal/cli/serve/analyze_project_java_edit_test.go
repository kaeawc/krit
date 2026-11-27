package serve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// With only DeadCode active no rule needs Java before dispatch, so Java sits
// outside the per-file set and a Java-only edit leaves every Kotlin entry a
// cache hit. B.java's call is the only reference to A.kt's helper(); after
// each edit the daemon must report what a fresh, cache-free call does
// (#1843).
func TestAnalyzeProject_JavaEditOutsidePerFileSet(t *testing.T) {
	isolateFromHelperJars(t)
	socket, state := startServerForTest(t)
	root := state.root
	seen := map[string]bool{}
	var cfg strings.Builder
	for _, r := range api.Registry {
		if r.Category != "" && !seen[r.Category] {
			seen[r.Category] = true
			fmt.Fprintf(&cfg, "%s:\n  active: false\n", r.Category)
		}
	}
	const calls = "package p;\n\npublic class B {\n    public int run() {\n        return AKt.helper();\n    }\n}\n"
	const noCall = "package p;\n\npublic class B {\n    public int run() {\n        return 0;\n    }\n}\n"
	write := func(name, content string) string {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write("krit.yml", cfg.String())
	write("A.kt", "package p\n\nfun helper(): Int = 1\n")
	java := write("B.java", calls)

	deadCode := func(noCache bool) string {
		t.Helper()
		var r daemon.AnalyzeProjectResult
		args := daemon.AnalyzeProjectArgs{NoFir: true, EnableRules: "DeadCode", NoCache: noCache}
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject, args, &r); err != nil {
			t.Fatal(err)
		}
		var report struct {
			Findings []struct {
				Rule string `json:"rule"`
				File string `json:"file"`
				Line int    `json:"line"`
			} `json:"findings"`
		}
		if err := json.Unmarshal(r.Findings, &report); err != nil {
			t.Fatalf("findings JSON: %v", err)
		}
		var keys []string
		for _, f := range report.Findings {
			if f.Rule == "DeadCode" {
				keys = append(keys, fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line))
			}
		}
		sort.Strings(keys)
		return strings.Join(keys, " ")
	}
	step := func(name string) {
		t.Helper()
		warm := deadCode(false)
		warmAgain := deadCode(false)
		fresh := deadCode(true)
		if warm != fresh || warmAgain != fresh {
			t.Fatalf("%s: warm DeadCode [%s], second warm [%s], fresh [%s]", name, warm, warmAgain, fresh)
		}
		t.Logf("%s: [%s]", name, warm)
	}
	// Deliver each edit through the watcher's own handler, as fsnotify would.
	fw := &fileWatcher{
		state:          state.workspace,
		root:           root,
		debounce:       map[string]*time.Timer{},
		debounceGen:    map[string]uint64{},
		debounceWindow: 150 * time.Millisecond,
	}
	edit := func(content string) {
		write("B.java", content)
		fw.handle(backendEvent{Path: java, Op: opWrite})
	}
	step("cold")
	edit(noCall)
	step("remove Java call")
	edit(calls)
	step("re-add Java call")
	edit(noCall)
	step("remove Java call again")
}

// A Java edit made while no daemon ran must reach the next daemon's first
// call too. That daemon's stat-drift check covered only the parsed files, and
// with DeadCode alone the parse holds no Java, so B.java kept the content
// hash the prior manifest recorded and the findings bundle hit (#1893).
func TestAnalyzeProject_JavaEditWhileDaemonWasDownIsSeen(t *testing.T) {
	isolateFromHelperJars(t)
	root := t.TempDir()
	seen := map[string]bool{}
	var cfg strings.Builder
	for _, r := range api.Registry {
		if r.Category != "" && !seen[r.Category] {
			seen[r.Category] = true
			fmt.Fprintf(&cfg, "%s:\n  active: false\n", r.Category)
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("krit.yml", cfg.String())
	write("A.kt", "package p\n\nfun helper(): Int = 1\n")
	write("B.java", "package p;\n\npublic class B {\n    public int run() {\n        return AKt.helper();\n    }\n}\n")

	deadCode := func(socket string, noCache bool) string {
		t.Helper()
		var r daemon.AnalyzeProjectResult
		args := daemon.AnalyzeProjectArgs{NoFir: true, EnableRules: "DeadCode", NoCache: noCache}
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject, args, &r); err != nil {
			t.Fatal(err)
		}
		var report struct {
			Findings []struct {
				Rule string `json:"rule"`
				File string `json:"file"`
				Line int    `json:"line"`
			} `json:"findings"`
		}
		if err := json.Unmarshal(r.Findings, &report); err != nil {
			t.Fatalf("findings JSON: %v", err)
		}
		var keys []string
		for _, f := range report.Findings {
			if f.Rule == "DeadCode" {
				keys = append(keys, fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line))
			}
		}
		sort.Strings(keys)
		return strings.Join(keys, " ")
	}
	first, _ := startServerWith(t, root, 5*time.Second)
	cold := deadCode(first, false)
	// The manifest is written in the background; a daemon that stays up long
	// enough to be stopped has persisted it.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if m, _ := filepath.Glob(filepath.Join(root, ".krit", "findings-bundle-cache", "manifests", "*", "*")); len(m) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the first daemon never persisted its bundle manifest; the test would prove nothing")
		}
		time.Sleep(20 * time.Millisecond)
	}

	write("B.java", "package p;\n\npublic class B {\n    public int run() {\n        return 0;\n    }\n}\n")
	second, _ := startServerWith(t, root, 5*time.Second)
	warm := deadCode(second, false)
	fresh := deadCode(second, true)
	if warm != fresh {
		t.Fatalf("first call after the restart: DeadCode [%s], fresh [%s] (before the edit: [%s])", warm, fresh, cold)
	}
	if !strings.Contains(fresh, "A.kt:3") {
		t.Fatalf("fresh DeadCode [%s] lacks A.kt:3; the edit no longer orphans helper()", fresh)
	}
}
