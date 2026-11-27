package scan

import (
	"encoding/json"
	"flag"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestFilterColumnsNewSince(t *testing.T) {
	columns := scanner.CollectFindings([]scanner.Finding{
		{File: "/repo/src/A.kt", Line: 1, Col: 1, Rule: "RuleA", RuleSet: "style", Severity: "warning", Message: "old"},
		{File: "/repo/src/B.kt", Line: 2, Col: 1, Rule: "RuleB", RuleSet: "style", Severity: "warning", Message: "new"},
	})
	baseIDs := map[string]bool{
		deltaFindingID("src/A.kt", "RuleA", "old"): true,
	}
	filtered := filterColumnsNewSince(&columns, baseIDs, "/repo")
	if filtered.Len() != 1 {
		t.Fatalf("filtered.Len() = %d, want 1", filtered.Len())
	}
	if got := filtered.RuleAt(0); got != "RuleB" {
		t.Fatalf("remaining rule = %q", got)
	}
}

func TestMapPathToWorktree(t *testing.T) {
	got := mapPathToWorktree("/repo", "/tmp/base", "/repo/app/src")
	if got != "/tmp/base/app/src" {
		t.Fatalf("mapPathToWorktree() = %q", got)
	}
}

// PR mode under --delta (#1660): the base scan gets the current side's
// files, mapped into its worktree, and the forwarded model, before the scan
// paths.
func TestDeltaBaseScanArgsCarryThePRModeFilesAndModel(t *testing.T) {
	f := freshScanFlags(t)
	*f.Verbose = true
	args := deltaSnapshotArgsForFlags(f, []string{"/repo/app"}, "/repo", "/tmp/base",
		baseScanInputs{PRFileList: "/tmp/list/pr-files.txt", GradleModel: "/tmp/list/model/gradle-model"})
	for flag, want := range map[string]string{"--pr-files": "/tmp/list/pr-files.txt", "--gradle-model": "/tmp/list/model/gradle-model"} {
		at := slices.Index(args, flag)
		if at < 0 || at+1 >= len(args) || args[at+1] != want {
			t.Fatalf("args lack %s %s: %v", flag, want, args)
		}
	}
	if !slices.Contains(args, "-v") {
		t.Fatalf("a verbose --delta does not ask the base scan for its log: %v", args)
	}
	if last := args[len(args)-1]; last != "/tmp/base/app" {
		t.Fatalf("scan path must come last, after the flags: %v", args)
	}
	if args := deltaSnapshotArgsForFlags(freshScanFlags(t), nil, "/repo", "/tmp/base", baseScanInputs{}); slices.Contains(args, "--pr-files") || slices.Contains(args, "-v") || slices.Contains(args, "--gradle-model") {
		t.Fatalf("whole-repository base scan args = %v", args)
	}
	noModel := freshScanFlags(t)
	*noModel.NoGradleModel = true
	if args := deltaSnapshotArgsForFlags(noModel, nil, "/repo", "/tmp/base", baseScanInputs{}); !slices.Contains(args, "--no-gradle-model") {
		t.Fatalf("--no-gradle-model is not passed on: %v", args)
	}

	dir := t.TempDir()
	list := filepath.Join(dir, "pr-files.txt")
	if err := writePRFileList(list, []string{"/repo/app/src/A.kt", "/repo/lib/B.kt"}, "/repo", "/tmp/base"); err != nil {
		t.Fatal(err)
	}
	got, err := readPRFileList(list)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"/tmp/base/app/src/A.kt": true, "/tmp/base/lib/B.kt": true}
	if !maps.Equal(got, want) {
		t.Fatalf("round-tripped list = %v, want %v", got, want)
	}
}

// deltaModelFixture is a checkout with a model under .krit/<name> whose
// project lists a source dir, a generated source dir and an R.jar, and a
// worktree that has the source dir but none of the build outputs.
type deltaModelFixture struct {
	repo, tmp, scratch string
}

func newDeltaModelFixture(t *testing.T, name string) deltaModelFixture {
	t.Helper()
	root := t.TempDir()
	fx := deltaModelFixture{repo: filepath.Join(root, "repo"), tmp: filepath.Join(root, "base"), scratch: filepath.Join(root, "scratch")}
	for _, d := range []string{
		"repo/app/src/main/kotlin", "repo/app/build/generated/ksp/main/kotlin", "repo/app/build/intermediates",
		"base/app/src/main/kotlin", "scratch",
	} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(fx.repo, "settings.gradle.kts"), "")
	writeFile(t, filepath.Join(fx.repo, "app/build/intermediates/R.jar"), "jar")
	model := map[string]any{"schema": 2, "rootDir": fx.repo, "futureField": map[string]any{"kept": true}, "projects": []any{map[string]any{
		"path": ":app", "dir": filepath.Join(fx.repo, "app"),
		"sourceSets": []any{map[string]any{
			"name": "main", "sourceDirs": []any{filepath.Join(fx.repo, "app/src/main/kotlin")},
			"generatedSourceDirs": []any{filepath.Join(fx.repo, "app/build/generated/ksp/main/kotlin")},
			"generatedClasspath":  []any{filepath.Join(fx.repo, "app/build/intermediates/R.jar")},
			"classpath":           []any{"/gradle/caches/lib.jar"},
		}},
	}}}
	data, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(fx.repo, ".krit", name, "model.json"), string(data))
	writeFile(t, filepath.Join(fx.repo, ".krit", name, "inputs.stamp"), "stamp")
	return fx
}

type forwardedSet struct {
	SourceDirs          []string `json:"sourceDirs"`
	GeneratedSourceDirs []string `json:"generatedSourceDirs"`
	GeneratedClasspath  []string `json:"generatedClasspath"`
	Classpath           []string `json:"classpath"`
}

type forwardedModel struct {
	RootDir     string         `json:"rootDir"`
	FutureField map[string]any `json:"futureField"`
	Projects    []struct {
		Dir        string         `json:"dir"`
		SourceSets []forwardedSet `json:"sourceSets"`
	} `json:"projects"`
}

func readForwardedModel(t *testing.T, path string) forwardedModel {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("forwarded model: %v", err)
	}
	var m forwardedModel
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Projects) != 1 || len(m.Projects[0].SourceSets) != 1 {
		t.Fatalf("forwarded model shape: %s", data)
	}
	return m
}

// The base scan gets the current model with the base ref's inputs rebased
// into the worktree and the build outputs the worktree lacks kept at this
// checkout's (review of #1941: R.jar and KSP output were rebased to paths
// that did not exist).
func TestDeltaForwardsTheModelWithInputsRebasedAndOutputsKept(t *testing.T) {
	for _, name := range []string{"gradle-model", "bazel-model"} {
		t.Run(name, func(t *testing.T) {
			fx := newDeltaModelFixture(t, name)
			got, err := forwardModelToBase(freshScanFlags(t), []string{fx.repo}, fx.repo, fx.tmp, fx.scratch)
			if err != nil {
				t.Fatal(err)
			}
			if got != (baseModel{Forwarded: true}) {
				t.Fatalf("a discovered model is found by discovery on both sides: %+v", got)
			}
			m := readForwardedModel(t, filepath.Join(fx.tmp, ".krit", name, "model.json"))
			set := m.Projects[0].SourceSets[0]
			switch {
			case m.RootDir != filepath.ToSlash(fx.tmp), m.Projects[0].Dir != filepath.ToSlash(filepath.Join(fx.tmp, "app")):
				t.Errorf("root %q, project dir %q: not rebased into the worktree", m.RootDir, m.Projects[0].Dir)
			case set.SourceDirs[0] != filepath.ToSlash(filepath.Join(fx.tmp, "app/src/main/kotlin")):
				t.Errorf("source dir %q is not the base ref's", set.SourceDirs[0])
			case set.GeneratedSourceDirs[0] != filepath.ToSlash(filepath.Join(fx.repo, "app/build/generated/ksp/main/kotlin")),
				set.GeneratedClasspath[0] != filepath.ToSlash(filepath.Join(fx.repo, "app/build/intermediates/R.jar")):
				t.Errorf("build outputs %v %v do not point at this checkout's", set.GeneratedSourceDirs, set.GeneratedClasspath)
			case set.Classpath[0] != "/gradle/caches/lib.jar":
				t.Errorf("a path outside the repository changed: %q", set.Classpath[0])
			case m.FutureField["kept"] != true:
				t.Errorf("an unknown field was dropped: %v", m.FutureField)
			}
			if data, err := os.ReadFile(filepath.Join(fx.tmp, ".krit", name, "inputs.stamp")); err != nil || string(data) != "stamp" {
				t.Errorf("other model files are not copied: %q %v", data, err)
			}
		})
	}
}

// An output the worktree has (a committed generated dir) is the base ref's.
func TestDeltaForwardedModelPrefersTheWorktreesOutput(t *testing.T) {
	fx := newDeltaModelFixture(t, "gradle-model")
	if err := os.MkdirAll(filepath.Join(fx.tmp, "app/build/generated/ksp/main/kotlin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := forwardModelToBase(freshScanFlags(t), []string{fx.repo}, fx.repo, fx.tmp, fx.scratch); err != nil {
		t.Fatal(err)
	}
	set := readForwardedModel(t, filepath.Join(fx.tmp, ".krit", "gradle-model", "model.json")).Projects[0].SourceSets[0]
	if want := filepath.ToSlash(filepath.Join(fx.tmp, "app/build/generated/ksp/main/kotlin")); set.GeneratedSourceDirs[0] != want {
		t.Fatalf("generated dir = %q, want the worktree's %q", set.GeneratedSourceDirs[0], want)
	}
}

func TestDeltaModelForwardingCases(t *testing.T) {
	// A model the base ref commits is its own.
	fx := newDeltaModelFixture(t, "gradle-model")
	own := filepath.Join(fx.tmp, ".krit", "gradle-model", "model.json")
	writeFile(t, own, `{"schema":2,"base":true}`)
	if got, err := forwardModelToBase(freshScanFlags(t), []string{fx.repo}, fx.repo, fx.tmp, fx.scratch); err != nil || !got.Forwarded {
		t.Fatalf("base-committed model: %+v %v", got, err)
	}
	if data, _ := os.ReadFile(own); string(data) != `{"schema":2,"base":true}` {
		t.Fatalf("the base ref's model was overwritten: %s", data)
	}

	// --no-gradle-model forwards nothing.
	fx = newDeltaModelFixture(t, "gradle-model")
	noModel := freshScanFlags(t)
	*noModel.NoGradleModel = true
	if got, err := forwardModelToBase(noModel, []string{fx.repo}, fx.repo, fx.tmp, fx.scratch); err != nil || got != (baseModel{}) {
		t.Fatalf("--no-gradle-model: %+v %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(fx.tmp, ".krit")); err == nil {
		t.Fatal("--no-gradle-model copied a model")
	}

	// --gradle-model, here outside the repository, is copied outside the
	// worktree and passed explicitly, so neither side discovers it.
	fx = newDeltaModelFixture(t, "gradle-model")
	explicit := freshScanFlags(t)
	*explicit.GradleModel = filepath.Join(t.TempDir(), "exported")
	if err := os.CopyFS(*explicit.GradleModel, os.DirFS(filepath.Join(fx.repo, ".krit", "gradle-model"))); err != nil {
		t.Fatal(err)
	}
	got, err := forwardModelToBase(explicit, []string{fx.repo}, fx.repo, fx.tmp, fx.scratch)
	if err != nil || got.GradleModelArg != filepath.Join(fx.scratch, "model", "exported") || !got.Forwarded {
		t.Fatalf("--gradle-model: %+v %v", got, err)
	}
	set := readForwardedModel(t, filepath.Join(got.GradleModelArg, "model.json")).Projects[0].SourceSets[0]
	if set.SourceDirs[0] != filepath.ToSlash(filepath.Join(fx.tmp, "app/src/main/kotlin")) {
		t.Fatalf("explicit model source dir = %q", set.SourceDirs[0])
	}
	if _, err := os.Stat(filepath.Join(fx.tmp, ".krit")); err == nil {
		t.Fatal("an explicit model went where the base scan would discover it")
	}

	// No model: nothing to forward.
	empty := t.TempDir()
	writeFile(t, filepath.Join(empty, "settings.gradle.kts"), "")
	if got, err := forwardModelToBase(freshScanFlags(t), []string{empty}, empty, t.TempDir(), t.TempDir()); err != nil || got != (baseModel{}) {
		t.Fatalf("no model: %+v %v", got, err)
	}
}

// A model that cannot be copied whole is an error (so the warning fires),
// and no partial copy is left where the base scan would discover it.
func TestDeltaModelThatCannotBeCopiedWholeIsAnError(t *testing.T) {
	fx := newDeltaModelFixture(t, "gradle-model")
	if err := os.Symlink(t.TempDir(), filepath.Join(fx.repo, ".krit", "gradle-model", "linked-dir")); err != nil {
		t.Fatal(err)
	}
	got, err := forwardModelToBase(freshScanFlags(t), []string{fx.repo}, fx.repo, fx.tmp, fx.scratch)
	if err == nil || got.Forwarded {
		t.Fatalf("forwarded a model it could not copy: %+v %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(fx.tmp, ".krit", "gradle-model")); err == nil {
		t.Fatal("a partial model copy was left in the worktree")
	}

	empty := newDeltaModelFixture(t, "gradle-model")
	for _, name := range []string{"model.json", "inputs.stamp"} {
		if err := os.Remove(filepath.Join(empty.repo, ".krit", "gradle-model", name)); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := forwardModelToBase(freshScanFlags(t), []string{empty.repo}, empty.repo, empty.tmp, empty.scratch); err == nil || got.Forwarded {
		t.Fatalf("an empty model directory counts as forwarded: %+v %v", got, err)
	}
}

func TestIsBuildInput(t *testing.T) {
	for path, want := range map[string]bool{
		"build.gradle.kts": true, "app/build.gradle": true, "settings.gradle.kts": true,
		"gradle/libs.versions.toml": true, "gradle.properties": true, "buildSrc/src/main/kotlin/Deps.kt": true,
		"tools/build-logic/convention/src/main/kotlin/A.kt": true, "MODULE.bazel": true, "app/BUILD.bazel": true,
		"defs.bzl": true, "app/src/main/kotlin/Build.kt": false, "README.md": false, "app/src/main/kotlin/A.kt": false,
	} {
		if got := isBuildInput(path); got != want {
			t.Errorf("isBuildInput(%q) = %v, want %v", path, got, want)
		}
	}
}

// The base scan's wiring without a JDK (review of #1941): a fake krit stands
// in for the base scan and checks what it gets, through the runner, so a
// dropped PR-file list or model copy fails here and not only in the
// JDK-gated parity test.
func TestDeltaBaseScanGetsTheChangedFilesTheModelAndItsOwnStateDir(t *testing.T) {
	t.Run("model", func(t *testing.T) { testDeltaBaseScanWiring(t, false) })
	// A model kept elsewhere and linked into .krit (review of #1941: WalkDir
	// did not follow the link, so nothing was copied and nothing warned).
	t.Run("symlinked model", func(t *testing.T) { testDeltaBaseScanWiring(t, true) })
}

func testDeltaBaseScanWiring(t *testing.T, symlinked bool) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(root, "repo")
	writeFile(t, filepath.Join(repo, ".gitignore"), ".krit/\n")
	writeFile(t, filepath.Join(repo, "settings.gradle.kts"), "include(\":app\")\n")
	src := filepath.Join(repo, "app/src/main/kotlin/A.kt")
	writeFile(t, src, "package a\n\nfun a() = 1\n")
	writeFile(t, filepath.Join(repo, "app/src/main/kotlin/B.kt"), "package a\n\nfun b() = 1\n")
	model := `{"schema":2,"rootDir":"` + repo + `","projects":[{"path":":app","dir":"` + filepath.Join(repo, "app") +
		`","sourceSets":[{"name":"main","sourceDirs":["` + filepath.Join(repo, "app/src/main/kotlin") + `"]}]}]}`
	modelDir := filepath.Join(repo, ".krit/gradle-model")
	if symlinked {
		modelDir = filepath.Join(root, "model-cache", "gradle-model")
	}
	writeFile(t, filepath.Join(modelDir, "model.json"), model)
	if symlinked {
		if err := os.MkdirAll(filepath.Join(repo, ".krit"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(modelDir, filepath.Join(repo, ".krit", "gradle-model")); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-qm", "base"}} {
		cmd := exec.Command("git", append([]string{"-c", "user.email=k@example.com", "-c", "user.name=k"}, args...)...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	writeFile(t, src, "package a\n\nfun a() = 2\n")
	writeFile(t, filepath.Join(repo, "build.gradle.kts"), "// a build change\n")
	// git diff REF lists tracked changes only, as a PR's committed files are.
	if out, err := exec.Command("git", "-C", repo, "add", "build.gradle.kts").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	record := filepath.Join(root, "record.txt")
	fake := filepath.Join(root, "krit")
	writeFile(t, fake, "#!/bin/sh\n{\n"+
		"  echo \"cwd=$PWD\"\n  echo \"state=$KRIT_STATE_DIR\"\n"+
		"  while [ $# -gt 0 ]; do case \"$1\" in --pr-files) echo \"list=$(cat \"$2\" | tr '\\n' ' ')\"; shift 2;; *) shift;; esac; done\n"+
		"  echo \"model=$(tr -d '\\n' < .krit/gradle-model/model.json)\"\n"+
		"} > "+record+"\necho '{\"findings\":[]}'\n")
	if err := os.Chmod(fake, 0o755); err != nil {
		t.Fatal(err)
	}
	old := deltaBaseExecutable
	deltaBaseExecutable = func() (string, error) { return fake, nil }
	t.Cleanup(func() { deltaBaseExecutable = old })
	t.Setenv(fsutil.StateDirEnv, filepath.Join(root, "shared-state"))
	t.Chdir(repo)

	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse([]string{"--delta", "HEAD"}); err != nil {
		t.Fatal(err)
	}
	r := &runner{f: f, paths: []string{repo}, files: []string{src, filepath.Join(repo, "app/src/main/kotlin/B.kt")}}
	r.prFiles = r.prModeFiles()
	cols := scanner.CollectFindings(nil)
	r.allColumns = &cols
	stderr := captureStderr(t, func() {
		if _, err := r.filterColumnsByDelta("HEAD"); err != nil {
			t.Fatal(err)
		}
	})
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the base scan did not run: %v", err)
	}
	got := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		k, v, _ := strings.Cut(line, "=")
		got[k] = v
	}
	worktree := got["cwd"]
	if worktree == "" || worktree == repo {
		t.Fatalf("the base scan ran in %q, not a worktree", worktree)
	}
	if want := filepath.Join(worktree, "app/src/main/kotlin/A.kt") + " "; got["list"] != want {
		t.Errorf("--pr-files list = %q, want only the changed file in the worktree %q", got["list"], want)
	}
	if !strings.Contains(got["model"], filepath.Join(worktree, "app/src/main/kotlin")) {
		t.Errorf("the worktree's model does not name its own sources: %s", got["model"])
	}
	if state := got["state"]; state == "" || strings.HasPrefix(state, filepath.Join(root, "shared-state")) || strings.HasPrefix(state, worktree) {
		t.Errorf("KRIT_STATE_DIR = %q: the base scan must get a state dir of its own", state)
	}
	if _, err := os.Stat(got["state"]); err == nil {
		t.Errorf("the base scan's state dir %s outlived it", got["state"])
	}
	if strings.Contains(stderr, "runs without this checkout's build model") {
		t.Errorf("the model was not forwarded:\n%s", stderr)
	}
	if !strings.Contains(stderr, "warning: --delta: build files changed since HEAD (build.gradle.kts)") {
		t.Errorf("no warning that the base compiles with this checkout's model after a build change:\n%s", stderr)
	}
}

func TestForwardBaseScanLogMarksEachLine(t *testing.T) {
	var b strings.Builder
	forwardBaseScanLog(&b, "verbose: one\n\nverbose: two\n")
	if got, want := b.String(), "--delta base: verbose: one\n--delta base: verbose: two\n"; got != want {
		t.Fatalf("forwarded log = %q, want %q", got, want)
	}
}

// captureStderr returns what fn writes to os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(reader)
		done <- string(data)
	}()
	os.Stderr = writer
	defer func() { os.Stderr = original }()
	fn()
	_ = writer.Close()
	out := <-done
	_ = reader.Close()
	return out
}
