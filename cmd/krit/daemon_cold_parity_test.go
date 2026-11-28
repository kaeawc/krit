package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runKritInDir runs the built binary from dir and returns stdout, stderr and
// the exit code.
func runKritInDir(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run krit: %v", err)
		}
		code = exitErr.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

// parityFindings runs one scan from dir and returns its findings array,
// re-encoded so two runs compare byte for byte. A daemon run must say it used
// the daemon; a cold run must not.
func parityFindings(t *testing.T, dir string, daemon bool, args ...string) string {
	t.Helper()
	full := []string{"--no-fir", "--no-type-oracle", "-f", "json"}
	if !daemon {
		full = append(full, "--no-daemon", "--no-cache", "--no-parse-cache")
	}
	full = append(full, args...)
	stdout, stderr, code := runKritInDir(t, dir, full...)
	if code > 1 {
		t.Fatalf("krit %v: exit=%d stderr=%q", full, code, stderr)
	}
	if used := strings.Contains(stderr, "info: using daemon"); used != daemon {
		t.Fatalf("krit %v: daemon used = %v, want %v\nstderr=%q", full, used, daemon, stderr)
	}
	var report struct {
		Findings []any `json:"findings"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("krit %v: invalid JSON: %v\nstdout=%q", full, err, stdout)
	}
	out, err := json.Marshal(report.Findings)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func startParityDaemon(t *testing.T, repo string) {
	t.Helper()
	buildDaemonBinary(t)
	if _, _, code := runKrit(t, "daemon", "start", "--repo", repo, "--timeout", "10s"); code != 0 {
		log, _ := os.ReadFile(filepath.Join(repo, ".krit", "daemon.log"))
		t.Fatalf("daemon start failed: exit=%d log=%q", code, log)
	}
	t.Cleanup(func() { _, _, _ = runKrit(t, "daemon", "stop", "--repo", repo) })
}

func writeRepoFile(t *testing.T, repo, rel, content string) string {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A daemon scan after a settings.gradle.kts edit reports what a cold run
// reports. The daemon's watcher kept the script's resident parse, so the first
// warm scan after the edit served the old NamedArguments finding (#1968).
func TestDaemonScanAfterGradleScriptEditMatchesColdRun(t *testing.T) {
	repo := shortTempDir(t)
	writeRepoFile(t, repo, "A.kt", "package demo\n\nclass A\n")
	writeRepoFile(t, repo, "settings.gradle.kts",
		"rootProject.name = \"demo\"\ninclude(\":a\", \":b\", \":c\", \":d\", \":e\")\n")
	startParityDaemon(t, repo)

	rules := []string{"--enable-rules", "NamedArguments", repo}
	if got := parityFindings(t, repo, true, rules...); !strings.Contains(got, "has 5 arguments") {
		t.Fatalf("first daemon scan lacks the 5-argument include finding: %s", got)
	}
	writeRepoFile(t, repo, "settings.gradle.kts",
		"rootProject.name = \"demo\"\ninclude(\":a\", \":b\", \":c\", \":d\")\n")
	// The daemon learns of the edit from its file watcher; give the event
	// time to arrive before the next scan starts.
	time.Sleep(time.Second)

	warm := parityFindings(t, repo, true, rules...)
	cold := parityFindings(t, repo, false, rules...)
	if !strings.Contains(cold, "has 4 arguments") {
		t.Fatalf("cold run lacks the edited include finding: %s", cold)
	}
	if warm != cold {
		t.Fatalf("daemon scan after the edit differs from a cold run:\ndaemon: %s\ncold:   %s", warm, cold)
	}
}

// A daemon scan of "." names each file's module as a cold run does. Module
// directories are absolute and the scan's paths were relative, so every file
// fell into the root bucket and ModuleDeadCode said "module root" (#1968).
func TestDaemonScanOfDotMatchesColdRunModuleNames(t *testing.T) {
	repo := shortTempDir(t)
	writeRepoFile(t, repo, "settings.gradle.kts", "include(\":app\", \":lib\")\n")
	for _, m := range []string{"app", "lib"} {
		writeRepoFile(t, repo, m+"/build.gradle.kts", "plugins { kotlin(\"jvm\") }\n")
	}
	writeRepoFile(t, repo, "lib/src/main/kotlin/Unused.kt", "package lib\n\nclass Unused\n")
	writeRepoFile(t, repo, "app/src/main/kotlin/Main.kt", "package app\n\nfun main() {}\n")
	startParityDaemon(t, repo)

	rules := []string{"--enable-rules", "ModuleDeadCode", "."}
	warm := parityFindings(t, repo, true, rules...)
	cold := parityFindings(t, repo, false, rules...)
	if !strings.Contains(cold, "'Unused' in module :lib") {
		t.Fatalf("cold run lacks the :lib finding: %s", cold)
	}
	if warm != cold {
		t.Fatalf("daemon scan of . differs from a cold run:\ndaemon: %s\ncold:   %s", warm, cold)
	}
}
