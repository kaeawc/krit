package scan

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartCPUProfileNoOpWhenEmpty(t *testing.T) {
	var buf bytes.Buffer
	f, err := startCPUProfile("", &buf)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if f != nil {
		t.Fatalf("expected nil file, got %v", f)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no output, got %q", buf.String())
	}
}

func TestStartCPUProfileReportsCreateError(t *testing.T) {
	var buf bytes.Buffer
	f, err := startCPUProfile("/nonexistent_dir_for_startCPUProfile_test/cpu.pprof", &buf)
	if err == nil {
		t.Fatal("expected create error, got nil")
	}
	if f != nil {
		t.Fatalf("expected nil file on error, got %v", f)
	}
	if !strings.Contains(buf.String(), "could not create CPU profile") {
		t.Fatalf("expected create-error message, got %q", buf.String())
	}
}

func TestStartStopCPUProfileHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cpu.pprof")

	var buf bytes.Buffer
	f, err := startCPUProfile(path, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil file")
	}
	// Do a small amount of work so the profile has something to record.
	for i := 0; i < 10000; i++ {
		_ = i * i
	}
	stopCPUProfile(f)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected profile file to exist: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("CPU profile file is empty")
	}
}

func TestStopCPUProfileNilIsSafe(t *testing.T) {
	stopCPUProfile(nil) // must not panic
}

func TestWriteMemProfileNoOpWhenEmpty(t *testing.T) {
	var buf bytes.Buffer
	writeMemProfile("", &buf)
	if buf.Len() != 0 {
		t.Fatalf("empty path produced output: %q", buf.String())
	}
}

func TestWriteMemProfileReportsCreateError(t *testing.T) {
	// /nonexistent_/path/... cannot be created — os.Create fails before
	// any profiling work. The function must report and return without
	// panicking.
	var buf bytes.Buffer
	writeMemProfile("/nonexistent_dir_for_writeMemProfile_test/foo.pprof", &buf)
	out := buf.String()
	if !strings.Contains(out, "could not create memory profile") {
		t.Fatalf("expected create-error message, got %q", out)
	}
}

func TestWriteMemProfileWritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mem.pprof")

	var buf bytes.Buffer
	writeMemProfile(path, &buf)

	if buf.Len() != 0 {
		t.Fatalf("expected no errors, got: %q", buf.String())
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected profile file to exist: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("profile file is empty")
	}
}
