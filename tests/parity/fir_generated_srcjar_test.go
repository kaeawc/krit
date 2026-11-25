package parity_test

import (
	"archive/zip"
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

// writeSrcjarProject writes a project whose only source uses gen.Foo and
// gen.Bar, which are defined nowhere but in the returned srcjar: the shape of
// Bazel codegen (KSP, Wire, SQLDelight), whose output reaches the compiler as
// a source jar.
func writeSrcjarProject(t *testing.T) (project, uses, jar string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project = filepath.Join(dir, "project")
	uses = filepath.Join(project, "src", "main", "kotlin", "app", "Uses.kt")
	if err := os.MkdirAll(filepath.Dir(uses), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "settings.gradle.kts"), []byte("rootProject.name = \"fixture\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "package app\n\nimport gen.Bar\nimport gen.Foo\n\nfun describe(): String = Foo(Bar.ID).label()\n"
	if err := os.WriteFile(uses, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	jar = filepath.Join(dir, "bazel-out", "gen-ksp.srcjar")
	if err := os.MkdirAll(filepath.Dir(jar), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		"gen/Foo.kt":           "package gen\n\nclass Foo(val id: Int) {\n    fun label(): String = \"foo-\" + id\n}\n",
		"gen/Bar.kt":           "package gen\n\nobject Bar {\n    const val ID: Int = 7\n}\n",
		"META-INF/MANIFEST.MF": "Manifest-Version: 1.0\n",
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jar, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return project, uses, jar
}

func writeSrcjarModel(t *testing.T, project string, jars []string) {
	t.Helper()
	set := map[string]any{
		"name": "main", "kind": "main", "platform": "jvm",
		"sourceDirs": []string{filepath.Join(project, "src", "main", "kotlin")},
	}
	if len(jars) > 0 {
		set["generatedSourceJars"] = jars
	}
	model, err := json.Marshal(map[string]any{"schema": 2, "projects": []map[string]any{{
		"path": ":", "dir": project, "sourceSets": []map[string]any{set},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	modelDir := filepath.Join(project, ".krit", "gradle-model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "root.json"), model, 0o644); err != nil {
		t.Fatal(err)
	}
}

func runSrcjarKrit(t *testing.T, bin, jar, project, kritHome string, extra ...string) string {
	t.Helper()
	args := append([]string{"--fir", "--no-fir-daemon", "--no-daemon", "--no-cache", "--no-type-oracle", "-f", "json", "-v"}, extra...)
	args = append(args, project)
	cmd := exec.Command(bin, args...)
	cmd.Dir = project
	cmd.Env = append(os.Environ(), "KRIT_FIR_JAR="+jar, "KRIT_WORKER_AOT=0", "KRIT_STATE_DIR="+t.TempDir(), "KRIT_HOME="+kritHome)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() > 1) {
		t.Fatalf("krit %v: %v\nstderr:\n%s", args, err, stderr.String())
	}
	return stderr.String()
}

// A type defined only in a generated srcjar resolves for FIR once the srcjar
// is passed with --generated-srcjar or listed in the model's
// generatedSourceJars (#1588), so the file using it is authoritative instead
// of gated, and nothing inside the srcjar is reported.
func TestFirResolvesGeneratedSrcjarEndToEnd(t *testing.T) {
	root := repoRoot(t)
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	bin := buildVerdictKrit(t, root)
	project, uses, srcjar := writeSrcjarProject(t)
	kritHome := t.TempDir()
	authoritative := "verbose: FIR verdict: 1 authoritative files, 0 gated (compiler error or crash)"

	writeSrcjarModel(t, project, nil)
	without := runSrcjarKrit(t, bin, firJar, project, kritHome)
	if line := "verbose: FIR gated: " + uses + ": Unresolved reference"; !strings.Contains(without, line) {
		t.Fatalf("without the srcjar: missing %q; stderr:\n%s", line, without)
	}

	flag := runSrcjarKrit(t, bin, firJar, project, kritHome, "--generated-srcjar", srcjar)
	if !strings.Contains(flag, authoritative) {
		t.Fatalf("--generated-srcjar: missing %q; stderr:\n%s", authoritative, flag)
	}
	if !strings.Contains(flag, "generated srcjars: 1 of 1 unpacked as generated source roots") {
		t.Fatalf("--generated-srcjar: no unpack summary; stderr:\n%s", flag)
	}
	unpacked, err := filepath.Glob(filepath.Join(kritHome, "srcjars", "*", "gen", "Foo.kt"))
	if err != nil || len(unpacked) != 1 {
		t.Fatalf("srcjar not unpacked under KRIT_HOME/srcjars: %v, %v", unpacked, err)
	}

	writeSrcjarModel(t, project, []string{srcjar})
	model := runSrcjarKrit(t, bin, firJar, project, kritHome)
	if !strings.Contains(model, authoritative) {
		t.Fatalf("model generatedSourceJars: missing %q; stderr:\n%s", authoritative, model)
	}
}
