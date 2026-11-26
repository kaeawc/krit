package parity_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
)

// Baseline IDs must not depend on the run mode or the --base-path spelling:
// a baseline created in CI (in-process) has to silence the same findings on
// a local daemon run, cold or warm (#1600). Each variant writes
// --create-baseline in-process and through the daemon, from a cold and a
// warm .krit/, with --base-path . and with the absolute path, and every file
// must be byte-identical.
func TestCreateBaselineIsIdenticalAcrossRunModes(t *testing.T) {
	root := repoRoot(t)
	bin := buildVerdictKrit(t, root)
	t.Run("no-fir", func(t *testing.T) { checkBaselineModes(t, bin, root, "") })
	t.Run("fir", func(t *testing.T) {
		jar := firchecks.FindFirJar([]string{root})
		if jar == "" || !isExecutableJar(jar) {
			t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
		}
		abs, err := filepath.Abs(jar)
		if err != nil {
			t.Fatal(err)
		}
		checkBaselineModes(t, bin, root, abs)
	})
}

func checkBaselineModes(t *testing.T, bin, root, firJar string) {
	project := copyBaselineProject(t, filepath.Join(root, "playground", "android-app"), firJar != "")
	home := t.TempDir()
	t.Cleanup(func() { stopDelegatedDaemon(t, bin, home, project) })
	run := func(daemon bool, args ...string) string {
		t.Helper()
		full := []string{"-q", "-f", "json"}
		if firJar != "" {
			full = append(full, "--fir")
		} else {
			full = append(full, "--no-fir")
		}
		if !daemon {
			full = append(full, "--no-daemon", "--no-fir-daemon")
		}
		cmd := exec.Command(bin, append(append(full, args...), ".")...)
		cmd.Dir = project
		cmd.Env = append(os.Environ(), "HOME="+home, "KRIT_FIR_JAR="+firJar)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		var exit *exec.ExitError
		if err != nil && (!errors.As(err, &exit) || exit.ExitCode() > 1) {
			t.Fatalf("krit %v: %v\nstderr:\n%s", full, err, stderr.String())
		}
		return stdout.String()
	}

	var reference []byte
	var referenceName string
	for _, mode := range []string{"in-process", "daemon"} {
		for _, base := range []string{".", project} {
			stopDelegatedDaemon(t, bin, home, project)
			if err := os.RemoveAll(filepath.Join(project, ".krit")); err != nil {
				t.Fatal(err)
			}
			for _, temp := range []string{"cold", "warm"} {
				label := "abs"
				if base == "." {
					label = "dot"
				}
				name := mode + "/" + label + "/" + temp
				out := filepath.Join(t.TempDir(), "baseline.xml")
				run(mode == "daemon", "--base-path", base, "--create-baseline", out)
				got, err := os.ReadFile(out)
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				if reference == nil {
					reference, referenceName = got, name
					checkPathIDs(t, got)
					continue
				}
				if !bytes.Equal(got, reference) {
					t.Errorf("%s baseline differs from %s:\n%s", name, referenceName, diffIDs(reference, got))
				}
			}
		}
	}

	// Each mode's baseline silences the other mode's findings.
	in := filepath.Join(t.TempDir(), "in-process.xml")
	if err := os.WriteFile(in, reference, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, daemon := range []bool{true, false} {
		if n := countFindings(t, run(daemon, "--base-path", ".", "--baseline", in)); n != 0 {
			t.Errorf("daemon=%v run against the baseline reported %d new findings, want 0", daemon, n)
		}
	}
}

// checkPathIDs asserts the baseline names Kotlin sources and manifests by
// repository path, the forms the daemon used to reduce to a base name.
func checkPathIDs(t *testing.T, baseline []byte) {
	t.Helper()
	var kotlin, manifest bool
	for _, id := range baselineIDsOf(baseline) {
		file := strings.SplitN(id, ":", 3)[1]
		kotlin = kotlin || (strings.HasSuffix(file, ".kt") && strings.Contains(file, string(filepath.Separator)))
		manifest = manifest || file == filepath.Join("src", "main", "AndroidManifest.xml")
	}
	if !kotlin || !manifest {
		t.Fatalf("baseline lacks repository-path IDs (kotlin=%v manifest=%v):\n%s", kotlin, manifest, baseline)
	}
}

func baselineIDsOf(baseline []byte) []string {
	var ids []string
	for _, line := range strings.Split(string(baseline), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "<ID>") {
			ids = append(ids, strings.TrimSuffix(strings.TrimPrefix(line, "<ID>"), "</ID>"))
		}
	}
	return ids
}

func diffIDs(want, got []byte) string {
	in := map[string]bool{}
	for _, id := range baselineIDsOf(want) {
		in[id] = true
	}
	var b strings.Builder
	for _, id := range baselineIDsOf(got) {
		if !in[id] {
			b.WriteString("+ " + id + "\n")
		}
		delete(in, id)
	}
	for id := range in {
		b.WriteString("- " + id + "\n")
	}
	return b.String()
}

func countFindings(t *testing.T, out string) int {
	t.Helper()
	var report struct {
		Findings []json.RawMessage `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("parse krit output: %v\n%s", err, out)
	}
	return len(report.Findings)
}

// copyBaselineProject copies src into a fresh git checkout. With fir it
// declares a stdlib-only classpath so the FIR checkers run.
func copyBaselineProject(t *testing.T, src string, fir bool) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			if d.Name() == ".krit" || d.Name() == "build" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	if fir {
		cfg := filepath.Join(dst, "krit.yml")
		existing, _ := os.ReadFile(cfg)
		if err := os.WriteFile(cfg, append(existing, []byte("\noracle:\n  classpath: []\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=test@example.com", "-c", "user.name=Test", "add", "-A"},
		{"-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-q", "-m", "seed"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dst
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git %v: %v\n%s", args, err, out)
		}
	}
	return dst
}
