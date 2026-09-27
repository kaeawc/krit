package oracle

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPreflightJavaMajorVersionInvalidatesOnBinaryChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "java")
	write := func(version string, when time.Time) {
		t.Helper()
		if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'openjdk version \""+version+"\"' >&2\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
	}
	when := time.Now().Add(-time.Hour)
	write("21", when)
	if got := PreflightJavaMajorVersion(path); got != 21 {
		t.Fatalf("first version = %d", got)
	}
	write("17.0.1", when.Add(time.Minute))
	if got := PreflightJavaMajorVersion(path); got != 17 {
		t.Fatalf("changed version = %d", got)
	}
}
