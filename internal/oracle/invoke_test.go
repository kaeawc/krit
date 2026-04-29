package oracle

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// FindJar tests
// ---------------------------------------------------------------------------

func TestFindJar_NoJarReturnsEmpty(t *testing.T) {
	// Use a temp dir with no jar files; FindJar should return empty string.
	tmp := t.TempDir()
	result := FindJar([]string{tmp})
	if result != "" {
		t.Errorf("expected empty string when no jar exists, got %q", result)
	}
}

func TestFindJar_FindsJarInProjectDir(t *testing.T) {
	tmp := t.TempDir()
	kritDir := filepath.Join(tmp, ".krit")
	if err := os.MkdirAll(kritDir, 0755); err != nil {
		t.Fatal(err)
	}
	jarPath := filepath.Join(kritDir, "krit-types.jar")
	if err := os.WriteFile(jarPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	result := FindJar([]string{tmp})
	if result == "" {
		t.Fatal("expected FindJar to find the jar")
	}
	if filepath.Base(result) != "krit-types.jar" {
		t.Errorf("expected filename krit-types.jar, got %q", filepath.Base(result))
	}
}

// ---------------------------------------------------------------------------
// FindSourceDirs tests
// ---------------------------------------------------------------------------

func TestFindSourceDirs_FindsKotlinDir(t *testing.T) {
	tmp := t.TempDir()
	kotlinDir := filepath.Join(tmp, "src", "main", "kotlin")
	if err := os.MkdirAll(kotlinDir, 0755); err != nil {
		t.Fatal(err)
	}

	dirs := FindSourceDirs([]string{tmp})
	if len(dirs) == 0 {
		t.Fatal("expected to find at least one source directory")
	}
	found := false
	for _, d := range dirs {
		if d == kotlinDir {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %s in results, got %v", kotlinDir, dirs)
	}
}

func TestFindSourceDirs_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	dirs := FindSourceDirs([]string{tmp})
	if len(dirs) != 0 {
		t.Errorf("expected 0 source dirs for empty dir, got %d: %v", len(dirs), dirs)
	}
}

func TestFindSourceDirs_SkipsBuildDir(t *testing.T) {
	tmp := t.TempDir()
	// Put kotlin dir under build/ -- should be skipped
	kotlinDir := filepath.Join(tmp, "build", "src", "main", "kotlin")
	if err := os.MkdirAll(kotlinDir, 0755); err != nil {
		t.Fatal(err)
	}

	dirs := FindSourceDirs([]string{tmp})
	if len(dirs) != 0 {
		t.Errorf("expected 0 source dirs (build should be skipped), got %d: %v", len(dirs), dirs)
	}
}

func TestFindSourceDirs_SkipsKritCacheDir(t *testing.T) {
	tmp := t.TempDir()
	kotlinDir := filepath.Join(tmp, "src", "main", "res", ".krit", "parse-cache", "kotlin")
	if err := os.MkdirAll(kotlinDir, 0755); err != nil {
		t.Fatal(err)
	}

	dirs := FindSourceDirs([]string{tmp})
	if len(dirs) != 0 {
		t.Errorf("expected 0 source dirs (.krit cache should be skipped), got %d: %v", len(dirs), dirs)
	}
}

func TestFindSourceDirs_RespectsGitignore(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte(".claude/worktrees/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ignoredKotlinDir := filepath.Join(tmp, ".claude", "worktrees", "copy", "src", "main", "kotlin")
	if err := os.MkdirAll(ignoredKotlinDir, 0755); err != nil {
		t.Fatal(err)
	}
	keepKotlinDir := filepath.Join(tmp, "src", "main", "kotlin")
	if err := os.MkdirAll(keepKotlinDir, 0755); err != nil {
		t.Fatal(err)
	}

	dirs := FindSourceDirs([]string{tmp})
	if !containsPath(dirs, keepKotlinDir) {
		t.Fatalf("expected kept source dir %q in results: %v", keepKotlinDir, dirs)
	}
	if containsPath(dirs, ignoredKotlinDir) {
		t.Fatalf("expected gitignored source dir %q to be skipped: %v", ignoredKotlinDir, dirs)
	}
}

func TestCollectKtFiles_RespectsGitignore(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("generated/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.Join(tmp, "src", "main", "kotlin")
	if err := os.MkdirAll(filepath.Join(sourceDir, "generated"), 0755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(sourceDir, "Keep.kt")
	ignored := filepath.Join(sourceDir, "generated", "Ignored.kt")
	for _, path := range []string{keep, ignored} {
		if err := os.WriteFile(path, []byte("package test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := CollectKtFiles([]string{sourceDir})
	if err != nil {
		t.Fatalf("CollectKtFiles returned error: %v", err)
	}
	if !containsPath(files, keep) {
		t.Fatalf("expected kept file %q in results: %v", keep, files)
	}
	if containsPath(files, ignored) {
		t.Fatalf("expected gitignored file %q to be skipped: %v", ignored, files)
	}
}

func containsPath(paths []string, want string) bool {
	wantAbs, _ := filepath.Abs(want)
	for _, path := range paths {
		gotAbs, _ := filepath.Abs(path)
		if gotAbs == wantAbs {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// CachePath tests
// ---------------------------------------------------------------------------

func TestCachePath_NonEmpty(t *testing.T) {
	tmp := t.TempDir()
	result := CachePath([]string{tmp})
	if result == "" {
		t.Fatal("expected non-empty cache path")
	}
	expected := filepath.Join(tmp, ".krit", "types.json")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestCachePath_Deterministic(t *testing.T) {
	tmp := t.TempDir()
	a := CachePath([]string{tmp})
	b := CachePath([]string{tmp})
	if a != b {
		t.Errorf("expected deterministic result, got %q and %q", a, b)
	}
}

func TestCachePath_EmptyScanPaths(t *testing.T) {
	result := CachePath(nil)
	if result != "" {
		t.Errorf("expected empty string for nil scan paths, got %q", result)
	}
}

func TestCachePath_FileAsProjectDir(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "some.kt")
	os.WriteFile(filePath, []byte("fun main(){}"), 0644)

	result := CachePath([]string{filePath})
	// Should use the file's parent directory
	expected := filepath.Join(tmp, ".krit", "types.json")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

// ---------------------------------------------------------------------------
// LoadFromData tests
// ---------------------------------------------------------------------------

func TestLoadFromData_ValidData(t *testing.T) {
	data := &OracleData{
		Version:      1,
		Dependencies: map[string]*OracleClass{},
		Files:        map[string]*OracleFile{},
	}
	o, err := LoadFromData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o == nil {
		t.Fatal("expected non-nil oracle")
	}
}

func TestLoadFromData_NilData(t *testing.T) {
	_, err := LoadFromData(nil)
	if err == nil {
		t.Error("expected error for nil data")
	}
}

func TestLoadFromData_BadVersion(t *testing.T) {
	data := &OracleData{
		Version: 0,
	}
	_, err := LoadFromData(data)
	if err == nil {
		t.Error("expected error for version 0")
	}
}

func TestLoadFromData_WithDependencies(t *testing.T) {
	data := &OracleData{
		Version: 1,
		Dependencies: map[string]*OracleClass{
			"com.example.Foo": {
				FQN:  "com.example.Foo",
				Kind: "class",
			},
		},
		Files: map[string]*OracleFile{},
	}
	o, err := LoadFromData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info := o.LookupClass("com.example.Foo")
	if info == nil {
		t.Fatal("expected to find Foo")
	}
	if info.Name != "Foo" {
		t.Errorf("expected simple name Foo, got %q", info.Name)
	}
}

// ---------------------------------------------------------------------------
// runOracleProcess tests — drive the full failure-mode matrix of the
// subprocess harness using `sh -c` as a fake krit-types so we don't need
// a real JVM binary in the test environment.
// ---------------------------------------------------------------------------

func shBinary(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("sh")
	if err != nil {
		t.Skipf("sh not in PATH, skipping subprocess test: %v", err)
	}
	return path
}

func TestRunOracleProcess_CleanExitWritesOutput(t *testing.T) {
	sh := shBinary(t)
	tmp := t.TempDir()
	outputPath := filepath.Join(tmp, "types.json")

	// Fake oracle: write a tiny JSON file, then exit 0.
	script := `echo '{"version":1}' > "$0"`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := runOracleProcess(ctx, sh,
		[]string{"-c", script, outputPath},
		outputPath, 5*time.Second, 500*time.Millisecond, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != outputPath {
		t.Errorf("expected result=%q, got %q", outputPath, result)
	}
	body, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
	if !strings.Contains(string(body), `"version":1`) {
		t.Errorf("expected JSON written by subprocess, got %q", body)
	}
}

func TestRunOracleProcess_NonZeroExitCapturesStderr(t *testing.T) {
	sh := shBinary(t)
	tmp := t.TempDir()
	outputPath := filepath.Join(tmp, "types.json")

	// Fake oracle: scream on stderr, exit 1. Must not race the grace period.
	script := `echo "FirPropertyImpl without source element" >&2; echo "at kotlin.analysis.Foo" >&2; exit 1`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := runOracleProcess(ctx, sh,
		[]string{"-c", script},
		outputPath, 5*time.Second, 500*time.Millisecond, false)
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}
	if !strings.Contains(err.Error(), "krit-types failed") {
		t.Errorf("expected error to mention krit-types, got %q", err)
	}
	if !strings.Contains(err.Error(), "FirPropertyImpl") {
		t.Errorf("expected error to include captured stderr (FirPropertyImpl), got %q", err)
	}
	if !strings.Contains(err.Error(), "stderr tail:") {
		t.Errorf("expected error to prefix stderr with 'stderr tail:', got %q", err)
	}
}

func TestRunOracleProcess_HardTimeoutProducesDiagnostic(t *testing.T) {
	sh := shBinary(t)
	tmp := t.TempDir()
	outputPath := filepath.Join(tmp, "types.json")

	// Fake oracle: print a diagnostic, then hang past the deadline without
	// writing the output file. `exec sleep` replaces the shell so the
	// subprocess is a single process and Kill() unambiguously ends it;
	// otherwise killing sh would leave an orphan sleep holding the stderr
	// pipe open and cmd.Wait() would block until it exited naturally.
	script := `echo "starting analysis" >&2; exec sleep 10`
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := runOracleProcess(ctx, sh,
		[]string{"-c", script},
		outputPath, 300*time.Millisecond, 10*time.Second, false)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected error to mention timeout, got %q", err)
	}
	if !strings.Contains(err.Error(), "starting analysis") {
		t.Errorf("expected stderr tail to include 'starting analysis', got %q", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("expected timeout to fire well under the 10s sleep, took %s", elapsed)
	}
}

func TestRunOracleProcess_GracePeriodKillsAfterOutputWritten(t *testing.T) {
	sh := shBinary(t)
	tmp := t.TempDir()
	outputPath := filepath.Join(tmp, "types.json")

	// Fake oracle: write output immediately, then hang. `exec sleep`
	// replaces the shell so Kill() actually ends the whole subprocess
	// (otherwise the orphan sleep child would keep the stderr pipe open
	// and block cmd.Wait). The grace period logic should then force-kill
	// the subprocess at ~500ms + 200ms and return the output path as
	// success because the content is already on disk.
	script := `echo '{"version":1}' > "$0"; exec sleep 10`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	result, err := runOracleProcess(ctx, sh,
		[]string{"-c", script, outputPath},
		outputPath, 5*time.Second, 200*time.Millisecond, false)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected nil error when output written before grace period fires, got %v", err)
	}
	if result != outputPath {
		t.Errorf("expected result=%q, got %q", outputPath, result)
	}
	if elapsed > 3*time.Second {
		t.Errorf("expected grace kill within ~1s, took %s", elapsed)
	}
}

// ---------------------------------------------------------------------------
// stderrTail ring buffer tests
// ---------------------------------------------------------------------------

func TestStderrTail_KeepsLastBytes(t *testing.T) {
	tail := &stderrTail{}
	big := strings.Repeat("abc\n", 5000) // 20 KB, exceeds the 8 KB cap
	tail.Write([]byte(big))
	got := tail.String()
	if len(got) != stderrTailSize {
		t.Errorf("expected tail to be capped at %d bytes, got %d", stderrTailSize, len(got))
	}
	// The last line should still be a complete "abc\n" sequence.
	if !strings.HasSuffix(got, "abc\n") {
		t.Errorf("expected tail to end with 'abc\\n', got suffix %q", got[len(got)-8:])
	}
}

func TestFirstLines_TruncatesToN(t *testing.T) {
	input := "line 1\nline 2\nline 3\nline 4\nline 5\n"
	got := firstLines(input, 3)
	want := "line 1\nline 2\nline 3"
	if got != want {
		t.Errorf("firstLines(3): expected %q, got %q", want, got)
	}
}

func TestFirstLines_ShorterThanN(t *testing.T) {
	got := firstLines("only\ntwo", 10)
	if got != "only\ntwo" {
		t.Errorf("expected full input when fewer lines than n, got %q", got)
	}
}
