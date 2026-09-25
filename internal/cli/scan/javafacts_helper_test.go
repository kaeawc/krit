package scan

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCompileJavaFactsHelperCachesCompiledClasses(t *testing.T) {
	if _, err := exec.LookPath("javac"); err != nil {
		t.Skip("javac is not available")
	}
	original := javaFactsCacheRoot
	cacheRoot := t.TempDir()
	javaFactsCacheRoot = func() (string, error) { return cacheRoot, nil }
	t.Cleanup(func() { javaFactsCacheRoot = original })

	classpath, cleanup, warning, err := compileJavaFactsHelper(nil)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil || warning != "" {
		t.Fatalf("first compile: warning=%q err=%v", warning, err)
	}
	classFile := filepath.Join(classpath, "dev", "jasonpearson", "krit", "javafacts", "Main.class")
	if _, err := os.Stat(classFile); err != nil {
		t.Fatalf("Main.class missing: %v", err)
	}
	marker := filepath.Join(classpath, ".complete")
	before, err := os.Stat(marker)
	if err != nil {
		t.Fatalf("completion marker missing: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	secondPath, secondCleanup, warning, err := compileJavaFactsHelper(nil)
	if secondCleanup != nil {
		defer secondCleanup()
	}
	if err != nil || warning != "" {
		t.Fatalf("second compile: warning=%q err=%v", warning, err)
	}
	if secondPath != classpath {
		t.Fatalf("classpath changed: first %q, second %q", classpath, secondPath)
	}
	after, err := os.Stat(marker)
	if err != nil {
		t.Fatalf("completion marker missing after second call: %v", err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("completion marker mtime changed: before %v, after %v", before.ModTime(), after.ModTime())
	}
}

func TestCompileJavaFactsHelperCacheKeyIncludesJavacIdentity(t *testing.T) {
	if _, err := exec.LookPath("javac"); err != nil {
		t.Skip("javac is not available")
	}
	originalRoot := javaFactsCacheRoot
	cacheRoot := t.TempDir()
	javaFactsCacheRoot = func() (string, error) { return cacheRoot, nil }
	t.Cleanup(func() { javaFactsCacheRoot = originalRoot })

	originalIdentity := resolveJavacIdentity
	t.Cleanup(func() { resolveJavacIdentity = originalIdentity })
	resolveJavacIdentity = func(string) javacIdentity {
		return javacIdentity{path: "javac-a", size: 1, modTime: 1}
	}
	firstPath, firstCleanup, warning, err := compileJavaFactsHelper(nil)
	if firstCleanup != nil {
		defer firstCleanup()
	}
	if err != nil || warning != "" {
		t.Fatalf("first compile: warning=%q err=%v", warning, err)
	}

	resolveJavacIdentity = func(string) javacIdentity {
		return javacIdentity{path: "javac-b", size: 2, modTime: 2}
	}
	secondPath, secondCleanup, warning, err := compileJavaFactsHelper(nil)
	if secondCleanup != nil {
		defer secondCleanup()
	}
	if err != nil || warning != "" {
		t.Fatalf("second compile: warning=%q err=%v", warning, err)
	}
	if secondPath == firstPath {
		t.Fatalf("classpath did not change for a different javac identity: %q", firstPath)
	}
}

func TestCompileJavaFactsHelperCacheKeyIncludesJavaHome(t *testing.T) {
	if _, err := exec.LookPath("javac"); err != nil {
		t.Skip("javac is not available")
	}
	originalRoot := javaFactsCacheRoot
	cacheRoot := t.TempDir()
	javaFactsCacheRoot = func() (string, error) { return cacheRoot, nil }
	t.Cleanup(func() { javaFactsCacheRoot = originalRoot })
	originalIdentity := resolveJavacIdentity
	resolveJavacIdentity = func(string) javacIdentity {
		return javacIdentity{path: "javac-fixed", size: 1, modTime: 1}
	}
	t.Cleanup(func() { resolveJavacIdentity = originalIdentity })

	t.Setenv("JAVA_HOME", "")
	firstPath, firstCleanup, warning, err := compileJavaFactsHelper(nil)
	if firstCleanup != nil {
		defer firstCleanup()
	}
	if err != nil || warning != "" {
		t.Fatalf("first compile: warning=%q err=%v", warning, err)
	}
	t.Setenv("JAVA_HOME", "/fake/jdk-b")
	secondPath, secondCleanup, warning, err := compileJavaFactsHelper(nil)
	if secondCleanup != nil {
		defer secondCleanup()
	}
	if err != nil || warning != "" {
		t.Fatalf("second compile: warning=%q err=%v", warning, err)
	}
	if secondPath == firstPath {
		t.Fatalf("classpath did not change for a different JAVA_HOME: %q", firstPath)
	}
}

func TestCompileJavaFactsHelperUsesJava17Release(t *testing.T) {
	if _, err := exec.LookPath("javac"); err != nil {
		t.Skip("javac is not available")
	}
	original := javaFactsCacheRoot
	javaFactsCacheRoot = func() (string, error) { return t.TempDir(), nil }
	t.Cleanup(func() { javaFactsCacheRoot = original })

	classpath, cleanup, warning, err := compileJavaFactsHelper(nil)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil || warning != "" {
		t.Fatalf("compile: warning=%q err=%v", warning, err)
	}
	classFile := filepath.Join(classpath, "dev", "jasonpearson", "krit", "javafacts", "Main.class")
	contents, err := os.ReadFile(classFile)
	if err != nil {
		t.Fatalf("read Main.class: %v", err)
	}
	if len(contents) < 8 {
		t.Fatalf("Main.class is too short: %d bytes", len(contents))
	}
	if major := binary.BigEndian.Uint16(contents[6:8]); major != 61 {
		t.Fatalf("class file major version = %d, want 61 (Java 17)", major)
	}
}

func TestCompileJavaFactsHelperFallsBackWhenJarsRootIsUnwritable(t *testing.T) {
	if _, err := exec.LookPath("javac"); err != nil {
		t.Skip("javac is not available")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can write through directory permission bits")
	}
	readOnlyDir := t.TempDir()
	if err := os.Chmod(readOnlyDir, 0o500); err != nil {
		t.Fatalf("chmod read-only directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0o700) })
	original := javaFactsCacheRoot
	javaFactsCacheRoot = func() (string, error) { return filepath.Join(readOnlyDir, "jars"), nil }
	t.Cleanup(func() { javaFactsCacheRoot = original })

	classpath, cleanup, warning, err := compileJavaFactsHelper(nil)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil || warning != "" {
		t.Fatalf("compile: warning=%q err=%v", warning, err)
	}
	classFile := filepath.Join(classpath, "dev", "jasonpearson", "krit", "javafacts", "Main.class")
	if _, err := os.Stat(classFile); err != nil {
		t.Fatalf("one-shot Main.class missing: %v", err)
	}
}
