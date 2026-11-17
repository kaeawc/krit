package jdk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeJDK creates <dir>/bin/<tools> as empty executables and returns dir.
func fakeJDK(t *testing.T, tools ...string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		if err := os.WriteFile(filepath.Join(bin, executableName(tool)), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestToolPrefersJavaHomeOverPath(t *testing.T) {
	home := fakeJDK(t, "java", "javac")
	onPath := fakeJDK(t, "java", "javac")
	t.Setenv("PATH", filepath.Join(onPath, "bin"))
	t.Setenv(HomeEnv, home)

	for _, tool := range []string{"java", "javac"} {
		got, err := Tool(tool)
		if err != nil {
			t.Fatalf("Tool(%q): %v", tool, err)
		}
		if want := filepath.Join(home, "bin", executableName(tool)); got != want {
			t.Errorf("Tool(%q) = %q, want %q", tool, got, want)
		}
	}
}

func TestToolFallsBackToPathWithoutJavaHome(t *testing.T) {
	onPath := fakeJDK(t, "javac")
	t.Setenv("PATH", filepath.Join(onPath, "bin"))
	t.Setenv(HomeEnv, "")

	got, err := Javac()
	if err != nil {
		t.Fatalf("Javac: %v", err)
	}
	if !strings.HasPrefix(got, onPath) {
		t.Errorf("Javac() = %q, want a path under %q", got, onPath)
	}
}

func TestToolDoesNotFallBackWhenJavaHomeLacksTool(t *testing.T) {
	home := fakeJDK(t, "java")
	onPath := fakeJDK(t, "javac")
	t.Setenv("PATH", filepath.Join(onPath, "bin"))
	t.Setenv(HomeEnv, home)

	if got, err := Javac(); err == nil {
		t.Fatalf("Javac() = %q, want an error naming JAVA_HOME", got)
	} else if !strings.Contains(err.Error(), HomeEnv) {
		t.Errorf("error %q does not name %s", err, HomeEnv)
	}
}
