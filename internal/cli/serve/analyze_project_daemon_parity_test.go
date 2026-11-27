package serve

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// A warm daemon call after a settings.gradle.kts edit must report the edited
// script, as a cold run does. The watcher handled the script as a Gradle file
// only, so its resident parse survived the edit and the next call served the
// pre-edit NamedArguments finding (#1968).
func TestAnalyzeProject_GradleKotlinScriptEditIsSeenByWarmCall(t *testing.T) {
	isolateFromHelperJars(t)
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "A.kt", "package demo\n\nclass A\n")
	settings := writeKotlinFile(t, state.root, "settings.gradle.kts",
		"rootProject.name = \"demo\"\ninclude(\":a\", \":b\", \":c\", \":d\", \":e\")\n")
	w, err := startFileWatcher(context.Background(), state.root, state.workspace, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	t.Cleanup(w.Stop)
	<-w.Ready()

	call := func(args daemon.AnalyzeProjectArgs) string {
		t.Helper()
		args.NoFir = true
		args.EnableRules = "NamedArguments"
		var r daemon.AnalyzeProjectResult
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject, args, &r); err != nil {
			t.Fatal(err)
		}
		return string(r.Findings)
	}
	if got := call(daemon.AnalyzeProjectArgs{}); !strings.Contains(got, "has 5 arguments") {
		t.Fatalf("cold call lacks the 5-argument include finding: %s", got)
	}
	call(daemon.AnalyzeProjectArgs{})
	state.workspace.DrainDirty()

	if err := os.WriteFile(settings, []byte("rootProject.name = \"demo\"\ninclude(\":a\", \":b\", \":c\", \":d\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !waitForCondition(func() bool { return state.workspace.DirtyCount() > 0 }) {
		t.Fatal("the watcher never reported the settings edit")
	}
	warm := call(daemon.AnalyzeProjectArgs{})
	fresh := call(daemon.AnalyzeProjectArgs{NoCache: true, NoParseCache: true})
	if !strings.Contains(fresh, "has 4 arguments") {
		t.Fatalf("the no-cache call lacks the edited include finding: %s", fresh)
	}
	if strings.Contains(warm, "has 5 arguments") || !strings.Contains(warm, "has 4 arguments") {
		t.Fatalf("warm call after the edit served the old script (%s)", filepath.Base(settings))
	}
}

// A scan of "." through the daemon must name each file's module as the CLI
// does. Discovery spells module directories absolute while the request's
// source paths are relative to the caller's directory, so every file fell into
// the root bucket and ModuleDeadCode said "module root" for :lib (#1968).
func TestAnalyzeProject_RelativeScanPathKeepsModuleNames(t *testing.T) {
	isolateFromHelperJars(t)
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "settings.gradle.kts", "include(\":app\", \":lib\")\n")
	for _, m := range []string{"app", "lib"} {
		if err := os.MkdirAll(filepath.Join(state.root, m, "src", "main", "kotlin"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeKotlinFile(t, state.root, filepath.Join(m, "build.gradle.kts"), "plugins { kotlin(\"jvm\") }\n")
	}
	writeKotlinFile(t, state.root, filepath.Join("lib", "src", "main", "kotlin", "Unused.kt"), "package lib\n\nclass Unused\n")
	writeKotlinFile(t, state.root, filepath.Join("app", "src", "main", "kotlin", "Main.kt"), "package app\n\nfun main() {}\n")

	call := func(args daemon.AnalyzeProjectArgs) string {
		t.Helper()
		args.NoFir = true
		args.EnableRules = "ModuleDeadCode"
		args.Paths = []string{"."}
		args.Cwd = state.root
		var r daemon.AnalyzeProjectResult
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject, args, &r); err != nil {
			t.Fatal(err)
		}
		var out struct {
			Findings json.RawMessage `json:"findings"`
		}
		if err := json.Unmarshal(r.Findings, &out); err != nil {
			t.Fatalf("decode findings: %v", err)
		}
		return string(out.Findings)
	}
	got := call(daemon.AnalyzeProjectArgs{})
	if !strings.Contains(got, "'Unused' in module :lib") || strings.Contains(got, "module root") {
		t.Fatalf("daemon scan of . lost the module names: %s", got)
	}
	if fresh := call(daemon.AnalyzeProjectArgs{NoCache: true, NoParseCache: true}); fresh != got {
		t.Fatalf("warm findings differ from a no-cache call:\nwarm:  %s\nfresh: %s", got, fresh)
	}
}
