package serve

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/devjar"
)

// An analyze that races the watcher event for an edit may serve the old
// findings, but the next call after the event must match a clean scan, and the
// racing call must not leave anything behind that makes the event's later
// invalidation ineffective. Found by the warm-versus-clean mutation matrix
// (#786, row annotation-deprecated-raced, #832).
func TestAnalyzeProject_EditRacingWatcherEventIsSeenAfterEvent(t *testing.T) {
	const before = "package demo\n\nclass R {\n    fun a() {\n        println(\"x\")\n    }\n}\n"
	const after = "package demo\n\n@Deprecated(\"old\")\nclass R {\n    fun a() {\n        println(\"x\")\n    }\n}\n"

	const mid = "package demo\n\nclass R {\n    fun a() {\n        println(\"x\")\n        println(\"y\")\n    }\n}\n"

	type callFn = func(daemon.AnalyzeProjectArgs) daemon.AnalyzeProjectResult
	stages := []struct {
		name string
		// first is what is written before race runs; empty means after.
		first string
		// race runs between the write and the delivery of the watcher event.
		race func(fw *fileWatcher, p string, write func(string), call callFn) daemon.AnalyzeProjectResult
		// delivered: race delivers the events itself, so none is added after.
		delivered bool
		// exact: the racing call itself must match a clean scan. Only when the
		// watcher has already seen the write; before that nothing can know.
		exact bool
	}{
		{"analyze before any event", "", func(_ *fileWatcher, _ string, _ func(string), call callFn) daemon.AnalyzeProjectResult {
			return call(daemon.AnalyzeProjectArgs{NoFir: true})
		}, false, false},
		{"analyze inside the debounce window", "", func(fw *fileWatcher, p string, _ func(string), call callFn) daemon.AnalyzeProjectResult {
			fw.handle(backendEvent{Path: p, Op: opWrite})
			return call(daemon.AnalyzeProjectArgs{NoFir: true})
		}, false, true},
		// A second write in the same burst, between two analyzes: the second
		// write starts no immediate invalidation, so only the debounced one and
		// the bundle shortcut's refusal to trust a dirty file's stat stand
		// between the call that follows and a bundle recorded against the
		// first write's content.
		{"two writes in one burst", mid, func(fw *fileWatcher, p string, write func(string), call callFn) daemon.AnalyzeProjectResult {
			fw.handle(backendEvent{Path: p, Op: opWrite})
			call(daemon.AnalyzeProjectArgs{NoFir: true})
			write(after)
			fw.handle(backendEvent{Path: p, Op: opWrite})
			b := call(daemon.AnalyzeProjectArgs{NoFir: true})
			time.Sleep(3 * fw.debounceWindow)
			return b
		}, true, false},
	}
	for _, st := range stages {
		t.Run(st.name, func(t *testing.T) {
			// No JVM helper: this is about source findings, and a shared oracle
			// daemon would make the test depend on what else is running.
			t.Setenv(devjar.DirEnv, t.TempDir())
			t.Setenv("HOME", t.TempDir())
			t.Setenv("USERPROFILE", t.TempDir())
			socket, state := startServerForTest(t)
			p := writeKotlinFile(t, state.root, "R.kt", before)
			call := func(a daemon.AnalyzeProjectArgs) (r daemon.AnalyzeProjectResult) {
				if err := daemon.Call(socket, daemon.VerbAnalyzeProject, a, &r); err != nil {
					t.Fatal(err)
				}
				return
			}
			findings := func(r daemon.AnalyzeProjectResult) string {
				var shape struct {
					Findings json.RawMessage `json:"findings"`
				}
				if err := json.Unmarshal(r.Findings, &shape); err != nil {
					t.Fatal(err)
				}
				return string(shape.Findings)
			}
			fw := &fileWatcher{
				state:          state.workspace,
				debounce:       map[string]*time.Timer{},
				debounceGen:    map[string]uint64{},
				debounceWindow: 150 * time.Millisecond,
			}
			call(daemon.AnalyzeProjectArgs{NoFir: true})
			if !call(daemon.AnalyzeProjectArgs{NoFir: true}).Stats.FindingsBundleHit {
				t.Fatal("control must hit the bundle")
			}

			write := func(c string) { writeKotlinFile(t, state.root, "R.kt", c) }
			if st.first != "" {
				write(st.first)
			} else {
				write(after)
			}
			raced := findings(st.race(fw, p, write, call))

			// The event (or its debounced remainder) is delivered.
			if !st.delivered {
				fw.handle(backendEvent{Path: p, Op: opWrite})
			}
			deadline := time.Now().Add(5 * time.Second)
			for state.workspace.DirtyCount() == 0 && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			if state.workspace.DirtyCount() == 0 {
				t.Fatal("the watcher never reported the edit")
			}
			got := []string{}
			if st.exact {
				got = append(got, raced)
			}
			for i := 0; i < 2; i++ {
				got = append(got, findings(call(daemon.AnalyzeProjectArgs{NoFir: true})))
			}
			// A clean scan of the edited project, bypassing every cache.
			clean := findings(call(daemon.AnalyzeProjectArgs{NoFir: true, NoCache: true}))
			for i, g := range got {
				if g != clean {
					t.Fatalf("result %d differs from a clean scan:\n got: %s\nwant: %s", i+1, g, clean)
				}
			}
		})
	}
}
