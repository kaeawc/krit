package parity_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
)

// sourceSetFixtures declare class Fake in both test and androidTest. The real
// build compiles them separately; one merged compile reports a redeclaration.
var sourceSetFixtures = map[string]string{
	"settings.gradle.kts":                "rootProject.name = \"fixture\"\n",
	"src/main/kotlin/app/Api.kt":         "package app\n\ninternal fun shared(): Int = 1\n",
	"src/test/kotlin/app/Fake.kt":        "package app\n\nclass Fake {\n    fun load(): Int = shared()\n}\n",
	"src/androidTest/kotlin/app/Fake.kt": "package app\n\nclass Fake {\n    fun load(): Int = shared() + 1\n}\n",
}

func writeSourceSetProject(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range sourceSetFixtures {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	set := func(name string) map[string]any {
		return map[string]any{"name": name, "kind": name, "platform": "jvm",
			"sourceDirs": []string{filepath.Join(dir, "src", name, "kotlin")}}
	}
	model, err := json.Marshal(map[string]any{"schema": 2, "projects": []map[string]any{{
		"path": ":", "dir": dir, "sourceSets": []map[string]any{set("main"), set("test"), set("androidTest")},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	modelDir := filepath.Join(dir, ".krit", "gradle-model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "root.json"), model, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// runSourceSetKrit scans project with --fir and returns krit's stderr.
func runSourceSetKrit(t *testing.T, bin, jar, project, shards string) string {
	t.Helper()
	args := []string{"--fir", "--no-fir-daemon", "--no-daemon", "--no-cache", "--no-type-oracle", "-f", "json", "-v", project}
	cmd := exec.Command(bin, args...)
	cmd.Dir = project
	cmd.Env = append(os.Environ(), "KRIT_FIR_JAR="+jar, "KRIT_MODULE_SHARDS="+shards, "KRIT_WORKER_AOT=0", "KRIT_STATE_DIR="+t.TempDir())
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() > 1) {
		t.Fatalf("krit %v: %v\nstderr:\n%s", args, err, stderr.String())
	}
	return stderr.String()
}

// A class declared in both test and androidTest gates both files in the
// merged compile. By default the pass rechecks that project with each source
// set compiled on its own, so every file gets FIR's verdict;
// KRIT_MODULE_SHARDS=merged keeps the merged verdicts.
func TestFirRechecksCrossSourceSetRedeclarationsEndToEnd(t *testing.T) {
	root := repoRoot(t)
	jar := firchecks.FindFirJar([]string{root})
	if jar == "" || !isExecutableJar(jar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	bin := buildVerdictKrit(t, root)
	project := writeSourceSetProject(t)
	fake := func(set string) string { return filepath.Join(project, "src", set, "kotlin", "app", "Fake.kt") }

	merged := runSourceSetKrit(t, bin, jar, project, "merged")
	for _, line := range []string{
		"verbose: FIR verdict: 1 authoritative files, 2 gated (compiler error or crash)",
		"verbose: FIR gated: " + fake("test") + ": Redeclaration:",
		"verbose: FIR gated: " + fake("androidTest") + ": Redeclaration:",
	} {
		if !strings.Contains(merged, line) {
			t.Fatalf("merged compile: missing %q; stderr:\n%s", line, merged)
		}
	}
	if strings.Contains(merged, "FIR source sets:") {
		t.Fatalf("KRIT_MODULE_SHARDS=merged should not recheck; stderr:\n%s", merged)
	}

	isolated := runSourceSetKrit(t, bin, jar, project, "")
	for _, line := range []string{
		"verbose: FIR source sets: 2 files gated by duplicate declarations; rechecked 3 files with each source set compiled on its own",
		", 0 still gated by one",
		"verbose: FIR verdict: 3 authoritative files, 0 gated (compiler error or crash)",
	} {
		if !strings.Contains(isolated, line) {
			t.Fatalf("default scan: missing %q; stderr:\n%s", line, isolated)
		}
	}
}
