package parity_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
)

// PR mode (#1660): `krit --diff REF` compiles only the modules that hold the
// changed files, through the krit-fir jar, for the type oracle and the FIR
// check, and reports what the whole-repository compile reports for them.
func TestFirPRModeCompilesOnlyTheChangedModulesEndToEnd(t *testing.T) {
	root := repoRoot(t)
	jar := firchecks.FindFirJar([]string{root})
	if jar == "" || !isExecutableJar(jar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	bin := buildVerdictKrit(t, root)
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(project, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("settings.gradle.kts", "include(\":core\")\ninclude(\":feature\")\n")
	write("core/src/main/kotlin/core/Core.kt", "package core\n\nclass Core(val v: Int) {\n    fun twice(): Int = v * 2\n}\n")
	write("feature/src/main/kotlin/feature/Feature.kt", "package feature\n\nimport core.Core\n\nfun feature(): Int = Core(1).twice()\n")
	write("feature/src/main/kotlin/feature/Other.kt", "package feature\n\nsuspend fun other(): Int = 1\n")
	set := func(project, name string, deps ...string) map[string]any {
		return map[string]any{"name": "main", "kind": "main", "platform": "jvm", "projectDeps": deps,
			"sourceDirs": []string{filepath.Join(project, name, "src", "main", "kotlin")}}
	}
	model, _ := json.Marshal(map[string]any{"schema": 2, "projects": []map[string]any{
		{"path": ":core", "dir": filepath.Join(project, "core"), "sourceSets": []map[string]any{set(project, "core")}},
		{"path": ":feature", "dir": filepath.Join(project, "feature"), "sourceSets": []map[string]any{set(project, "feature", ":core")}},
	}})
	write(".krit/gradle-model/model.json", string(model))
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.email=k@example.com", "-c", "user.name=k"}, args...)...)
		cmd.Dir = project
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-qm", "base")
	// The change: a redundant suspend modifier, which FIR confirms.
	write("feature/src/main/kotlin/feature/Feature.kt", "package feature\n\nimport core.Core\n\nsuspend fun feature(): Int = Core(1).twice()\n")

	scan := func(extra ...string) (stderr string, findings []string) {
		t.Helper()
		args := append([]string{"--diff", "HEAD", "--fir", "--no-daemon", "--no-fir-daemon", "--no-cache", "--no-cache-oracle",
			"--enable-rules", "RedundantSuspendModifier", "-v", "-f", "json"}, extra...)
		cmd := exec.Command(bin, append(args, project)...)
		cmd.Dir = project
		cmd.Env = append(os.Environ(), "KRIT_FIR_JAR="+jar, "KRIT_WORKER_AOT=0", "KRIT_STATE_DIR="+t.TempDir(), "KRIT_MODULE_SHARDS=")
		var stdout, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &errOut
		var exit *exec.ExitError
		if err := cmd.Run(); err != nil && (!errors.As(err, &exit) || exit.ExitCode() > 1) {
			t.Fatalf("krit %v: %v\nstderr:\n%s", args, err, errOut.String())
		}
		var report struct {
			Findings []struct {
				File string `json:"file"`
				Line int    `json:"line"`
				Rule string `json:"rule"`
			} `json:"findings"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("parse output: %v\n%s", err, stdout.String())
		}
		for _, f := range report.Findings {
			findings = append(findings, filepath.Base(f.File)+":"+f.Rule)
		}
		slices.Sort(findings)
		return errOut.String(), findings
	}

	prErr, prFindings := scan()
	for _, want := range []string{
		"verbose: PR mode: the type oracle and FIR analyzed 1 changed file in 1 module",
		"FIR analyzed 1 of 1 changed files (PR mode: 2 unchanged files keep Go's verdict).",
	} {
		if !strings.Contains(prErr, want) {
			t.Fatalf("PR mode scan lacks %q:\n%s", want, prErr)
		}
	}
	if !slices.Equal(prFindings, []string{"Feature.kt:RedundantSuspendModifier"}) {
		t.Fatalf("PR mode findings = %v\n%s", prFindings, prErr)
	}

	wholeErr, wholeFindings := scan("--no-pr-mode")
	if strings.Contains(wholeErr, "PR mode:") {
		t.Fatalf("--no-pr-mode still ran PR mode:\n%s", wholeErr)
	}
	if !slices.Equal(wholeFindings, prFindings) {
		t.Fatalf("whole-repository findings %v differ from PR mode's %v", wholeFindings, prFindings)
	}
}

// PR mode under --delta (#1660): the current scan and the base-ref scan both
// compile only the changed files' modules, the base scan with the untracked
// Gradle model copied into its worktree, so an unchanged file keeps Go's
// verdict on both sides and its findings cancel out. The new finding is the
// one the whole-repository --delta reports.
func TestFirPRModeDeltaComparesLikeForLikeEndToEnd(t *testing.T) {
	root := repoRoot(t)
	jar := firchecks.FindFirJar([]string{root})
	if jar == "" || !isExecutableJar(jar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	bin := buildVerdictKrit(t, root)
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(project, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", ".krit/\n")
	write("settings.gradle.kts", "include(\":core\")\ninclude(\":feature\")\n")
	write("core/src/main/kotlin/core/Core.kt", "package core\n\nclass Core(val v: Int) {\n    fun twice(): Int = v * 2\n}\n")
	write("feature/src/main/kotlin/feature/Feature.kt", "package feature\n\nimport core.Core\n\nfun feature(): Int = Core(1).twice()\n")
	// An existing finding in a file the change leaves alone.
	write("feature/src/main/kotlin/feature/Other.kt", "package feature\n\nsuspend fun other(): Int = 1\n")
	set := func(project, name string, deps ...string) map[string]any {
		return map[string]any{"name": "main", "kind": "main", "platform": "jvm", "projectDeps": deps,
			"sourceDirs": []string{filepath.Join(project, name, "src", "main", "kotlin")}}
	}
	model, _ := json.Marshal(map[string]any{"schema": 2, "rootDir": project, "projects": []map[string]any{
		{"path": ":core", "dir": filepath.Join(project, "core"), "sourceSets": []map[string]any{set(project, "core")}},
		{"path": ":feature", "dir": filepath.Join(project, "feature"), "sourceSets": []map[string]any{set(project, "feature", ":core")}},
	}})
	write(".krit/gradle-model/model.json", string(model))
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.email=k@example.com", "-c", "user.name=k"}, args...)...)
		cmd.Dir = project
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-qm", "base")
	write("feature/src/main/kotlin/feature/Feature.kt", "package feature\n\nimport core.Core\n\nsuspend fun feature(): Int = Core(1).twice()\n")

	scan := func(extra ...string) (stderr string, findings []string) {
		t.Helper()
		args := append([]string{"--delta", "HEAD", "--fir", "--no-daemon", "--no-fir-daemon", "--no-cache", "--no-cache-oracle",
			"--enable-rules", "RedundantSuspendModifier", "-v", "-f", "json"}, extra...)
		cmd := exec.Command(bin, append(args, project)...)
		cmd.Dir = project
		cmd.Env = append(os.Environ(), "KRIT_FIR_JAR="+jar, "KRIT_WORKER_AOT=0", "KRIT_STATE_DIR="+t.TempDir(), "KRIT_MODULE_SHARDS=")
		var stdout, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &errOut
		var exit *exec.ExitError
		if err := cmd.Run(); err != nil && (!errors.As(err, &exit) || exit.ExitCode() > 1) {
			t.Fatalf("krit %v: %v\nstderr:\n%s", args, err, errOut.String())
		}
		var report struct {
			Findings []struct {
				File string `json:"file"`
				Rule string `json:"rule"`
			} `json:"findings"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("parse output: %v\n%s", err, stdout.String())
		}
		for _, f := range report.Findings {
			findings = append(findings, filepath.Base(f.File)+":"+f.Rule)
		}
		slices.Sort(findings)
		return errOut.String(), findings
	}

	prErr, prFindings := scan()
	for _, want := range []string{
		"verbose: PR mode: the type oracle and FIR analyzed 1 changed file in 1 module",
		"--delta base: verbose: PR mode: the type oracle and FIR analyzed 1 changed file in 1 module",
	} {
		if !strings.Contains(prErr, want) {
			t.Fatalf("PR mode --delta lacks %q:\n%s", want, prErr)
		}
	}
	if !slices.Equal(prFindings, []string{"Feature.kt:RedundantSuspendModifier"}) {
		t.Fatalf("PR mode --delta findings = %v\n%s", prFindings, prErr)
	}

	wholeErr, wholeFindings := scan("--no-pr-mode")
	if strings.Contains(wholeErr, "PR mode:") {
		t.Fatalf("--no-pr-mode still ran PR mode:\n%s", wholeErr)
	}
	if !slices.Equal(wholeFindings, prFindings) {
		t.Fatalf("whole-repository --delta findings %v differ from PR mode's %v", wholeFindings, prFindings)
	}
}
