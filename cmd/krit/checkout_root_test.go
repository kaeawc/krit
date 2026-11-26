package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// writeCheckoutRootProject writes a project whose files sit right where the
// path heuristics would read the folders above the root (#1814):
// InvalidPackageDeclaration's directory suffix (sub/Foo.kt declares app.sub,
// and the root is named app), packageOmitsCommonRoot's source root (Root.kt
// declares x.app, and the root's parent is named kotlin), and
// isTestFrameworkModulePath's module tokens (a JUnit import in main code under
// a root whose ancestor is named junit-kit). The Android playground rides
// along so every other rule sees a real project too.
func writeCheckoutRootProject(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"settings.gradle.kts":                     "rootProject.name = \"app\"\n",
		"sub/Foo.kt":                              "package app.sub\n\nclass Foo\n",
		"Root.kt":                                 "package x.app\n\nclass Root\n",
		"lib/src/main/kotlin/p/UsesJUnit.kt":      "package p\n\nimport org.junit.Test\n\nclass UsesJUnit {\n    @Test fun t() {}\n}\n",
		"lib/src/main/kotlin/p/LibraryClass.kt":   "package p\n\nclass LibraryClass\n",
		"lib/build.gradle.kts":                    "plugins { kotlin(\"jvm\") }\n",
		"android/settings.gradle.kts":             "rootProject.name = \"android\"\n",
		"android/app/src/main/kotlin/a/Screen.kt": "package a\n\nclass Screen\n",
	}
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	playground := copyTree(t, "../../playground/android-app")
	if err := os.RemoveAll(filepath.Join(playground, ".krit")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(playground, filepath.Join(root, "android", "playground")); err != nil {
		t.Fatal(err)
	}
}

// relativeFindings scans root and returns its findings as sorted
// "rule path:line:col" keys with paths relative to root, plus the stderr.
func relativeFindings(t *testing.T, root string, extra ...string) ([]string, string) {
	t.Helper()
	args := []string{"--no-fir", "--no-daemon", "--no-type-oracle", "--all-rules", "-f", "json"}
	if !slices.Contains(extra, "-v") {
		args = append(args, "-q")
	}
	args = append(args, extra...)
	cmd := exec.Command(binPath, append(args, root)...)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() > 1 {
			t.Fatalf("krit %v %s: %v\n%s", extra, root, err, stderr.String())
		}
	}
	var report struct {
		Findings []scopeFinding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &report); err != nil {
		t.Fatalf("invalid JSON for %s: %v", root, err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(report.Findings))
	for _, f := range report.Findings {
		rel := f.File
		for _, r := range []string{resolved, root} {
			if r2, err := filepath.Rel(r, f.File); err == nil && !strings.HasPrefix(r2, "..") {
				rel = filepath.ToSlash(r2)
				break
			}
		}
		keys = append(keys, fmt.Sprintf("%s %s:%d:%d", f.Rule, rel, f.Line, f.Column))
	}
	sort.Strings(keys)
	return keys, stderr.String()
}

func diffKeys(a, b []string) (onlyA, onlyB []string) {
	inB := make(map[string]int, len(b))
	for _, k := range b {
		inB[k]++
	}
	for _, k := range a {
		if inB[k] > 0 {
			inB[k]--
			continue
		}
		onlyA = append(onlyA, k)
	}
	inA := make(map[string]int, len(a))
	for _, k := range a {
		inA[k]++
	}
	for _, k := range b {
		if inA[k] > 0 {
			inA[k]--
			continue
		}
		onlyB = append(onlyB, k)
	}
	return onlyA, onlyB
}

// No rule's verdict may depend on the folders above the project root: the
// per-file cache moves entries to a checkout at another absolute path
// (#1662) and serves their verdicts there (#1814). The two roots' ancestors
// stay free of internal/cache's rulePathMarkers, since a root holding one is
// never relocated.
func TestFindingsIgnoreFoldersAboveTheProjectRoot(t *testing.T) {
	base := neutralTempDir(t)
	rootA := filepath.Join(base, "ci1", "junit-kit", "kotlin", "app")
	rootB := filepath.Join(base, "ci2", "plain", "x", "app@2")
	writeCheckoutRootProject(t, rootA)
	writeCheckoutRootProject(t, rootB)

	a, _ := relativeFindings(t, rootA, "--no-cache")
	b, _ := relativeFindings(t, rootB, "--no-cache")
	if len(a) == 0 {
		t.Fatal("no findings at the first root; the project exercises nothing")
	}
	if onlyA, onlyB := diffKeys(a, b); len(onlyA)+len(onlyB) > 0 {
		t.Fatalf("findings depend on the folders above the project root:\nonly under %s:\n  %s\nonly under %s:\n  %s",
			rootA, strings.Join(onlyA, "\n  "), rootB, strings.Join(onlyB, "\n  "))
	}
}

// neutralTempDir returns a temporary directory whose path holds no segment a
// rule's path heuristic or a relocation marker reacts to. t.TempDir's path
// names the test, and "test" is both.
func neutralTempDir(t *testing.T) string {
	t.Helper()
	base, err := os.MkdirTemp("", "krit-root-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	return base
}

var cachedFilesRE = regexp.MustCompile(`Cache: (\d+)/(\d+) files cached`)

// A .krit restored into a checkout at another absolute path (a CI cache from
// another agent's workspace) serves its per-file entries there, and the warm
// scan matches a fresh one at the new root (#1662, #1806).
func TestRelocatedAnalysisCacheMatchesFreshScan(t *testing.T) {
	base := neutralTempDir(t)
	rootA := filepath.Join(base, "ci1", "app")
	writeCheckoutRootProject(t, rootA)
	if _, stderr := relativeFindings(t, rootA); stderr != "" && strings.Contains(stderr, "panic") {
		t.Fatalf("cold run: %s", stderr)
	}

	rootB := filepath.Join(base, "ci2", "app@2")
	if err := os.MkdirAll(filepath.Dir(rootB), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cp", "-a", rootA, rootB).CombinedOutput(); err != nil {
		t.Fatalf("cp -a: %v\n%s", err, out)
	}

	warm, stderr := relativeFindings(t, rootB, "-v")
	m := cachedFilesRE.FindStringSubmatch(stderr)
	if m == nil {
		t.Fatalf("warm run at the new root printed no cache summary:\n%s", stderr)
	}
	if cached, _ := strconv.Atoi(m[1]); cached == 0 {
		t.Fatalf("the relocated cache served no file at the new root: %s", m[0])
	}
	fresh, _ := relativeFindings(t, rootB, "--no-cache")
	if onlyWarm, onlyFresh := diffKeys(warm, fresh); len(onlyWarm)+len(onlyFresh) > 0 {
		t.Fatalf("relocated cache disagrees with a fresh scan at %s:\nonly warm:\n  %s\nonly fresh:\n  %s",
			rootB, strings.Join(onlyWarm, "\n  "), strings.Join(onlyFresh, "\n  "))
	}
}
