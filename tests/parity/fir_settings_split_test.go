package parity_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
)

// writeSettingsSplitProject writes a Gradle-shaped project whose source sets
// compile with different settings: :feature enables
// -Xcontext-sensitive-resolution (still experimental in Kotlin 2.4) and opts
// in to core.Exp, :core uses the compiler defaults. With withOther, :other also
// uses the defaults and calls the opt-in API without opting in, an error its
// real build reports.
func writeSettingsSplitProject(t *testing.T, withOther bool) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"settings.gradle.kts":                    "rootProject.name = \"fixture\"\ninclude(\":core\", \":feature\", \":other\")\n",
		"krit.yml":                               "oracle:\n  classpath: []\n",
		"core/src/main/kotlin/core/Api.kt":       "package core\n\n@RequiresOptIn(level = RequiresOptIn.Level.ERROR)\nannotation class Exp\n\n@Exp\nfun risky(): Int = 1\n\nenum class Color { RED, GREEN }\n",
		"feature/src/main/kotlin/feature/Use.kt": "package feature\n\nimport core.Color\nimport core.risky\n\nfun score(c: Color): Int = when (c) {\n    RED -> 1\n    GREEN -> 2\n}\n\nfun useRisky(): Int = risky()\n",
	}
	if withOther {
		files["other/src/main/kotlin/other/Bad.kt"] = "package other\n\nimport core.risky\n\nfun unchecked(): Int = risky()\n"
	}
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	set := func(project, optIns, args, deps string) string {
		return fmt.Sprintf(`{"path":":%[1]s","dir":%[2]q,"sourceSets":[{"name":"main","kind":"main","platform":"jvm","sourceDirs":[%[3]q],"projectDeps":[%[4]s],"kotlin":{"optIn":[%[5]s],"freeCompilerArgs":[%[6]s]}}]}`,
			project, filepath.Join(dir, project), filepath.Join(dir, project, "src", "main", "kotlin"), deps, optIns, args)
	}
	projects := []string{
		set("core", "", "", ""),
		set("feature", `"core.Exp"`, `"-Xcontext-sensitive-resolution"`, `":core"`),
	}
	if withOther {
		projects = append(projects, set("other", "", "", `":core"`))
	}
	modelDir := filepath.Join(dir, ".krit", "gradle-model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	model := `{"schema":2,"projects":[` + strings.Join(projects, ",") + `]}`
	if err := os.WriteFile(filepath.Join(modelDir, "root.json"), []byte(model), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runSettingsSplitKrit(t *testing.T, bin, jar, project, shards string) string {
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

// The merged whole-repository session intersects flags and unions opt-ins
// across source sets. By default the pass rechecks each group of source sets
// that share their settings in a session of its own (#1577): :feature's file,
// gated by the dropped -Xcontext-sensitive-resolution, becomes authoritative,
// and :other's missing opt-in, hidden by the union, is reported as the real
// build would. KRIT_MODULE_SHARDS=merged keeps the single merged session.
func TestFirSplitsSessionsBySourceSetSettingsEndToEnd(t *testing.T) {
	root := repoRoot(t)
	jar := firchecks.FindFirJar([]string{root})
	if jar == "" || !isExecutableJar(jar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	bin := buildVerdictKrit(t, root)

	t.Run("dropped flag", func(t *testing.T) {
		project := writeSettingsSplitProject(t, false)
		use := filepath.Join(project, "feature", "src", "main", "kotlin", "feature", "Use.kt")
		merged := runSettingsSplitKrit(t, bin, jar, project, "merged")
		for _, line := range []string{
			"verbose: FIR verdict: 1 authoritative files, 1 gated (compiler error or crash)",
			"verbose: FIR gated: " + use + ": 'when' expression must be exhaustive",
		} {
			if !strings.Contains(merged, line) {
				t.Fatalf("merged session: missing %q; stderr:\n%s", line, merged)
			}
		}
		if strings.Contains(merged, "FIR settings:") {
			t.Fatalf("KRIT_MODULE_SHARDS=merged split the session; stderr:\n%s", merged)
		}
		split := runSettingsSplitKrit(t, bin, jar, project, "")
		for _, line := range []string{
			"verbose: FIR settings: 3 sessions for 2 settings groups (grouping key: ",
			"verbose: FIR settings: session 3: :feature:main [-opt-in=core.Exp -Xcontext-sensitive-resolution]",
			"1 gated files now authoritative, 0 now gated on a missing opt-in",
			"verbose: FIR verdict: 2 authoritative files, 0 gated (compiler error or crash)",
		} {
			if !strings.Contains(split, line) {
				t.Fatalf("split sessions: missing %q; stderr:\n%s", line, split)
			}
		}
	})

	t.Run("hidden opt-in", func(t *testing.T) {
		project := writeSettingsSplitProject(t, true)
		bad := filepath.Join(project, "other", "src", "main", "kotlin", "other", "Bad.kt")
		merged := runSettingsSplitKrit(t, bin, jar, project, "merged")
		if strings.Contains(merged, "FIR gated: "+bad) {
			t.Fatalf("merged session gated %s; the opt-in union should hide its error; stderr:\n%s", bad, merged)
		}
		split := runSettingsSplitKrit(t, bin, jar, project, "")
		for _, line := range []string{
			"verbose: FIR settings: session 2: :core:main, :other:main [(compiler defaults)]",
			"1 gated files now authoritative, 1 now gated on a missing opt-in",
			"verbose: FIR gated: " + bad + ": This declaration needs opt-in.",
		} {
			if !strings.Contains(split, line) {
				t.Fatalf("split sessions: missing %q; stderr:\n%s", line, split)
			}
		}
	})
}
