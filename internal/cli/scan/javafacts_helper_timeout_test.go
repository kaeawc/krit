package scan

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// installBlockingJavac makes the helper compile run a javac that never
// finishes, as macOS's /usr/bin/javac stub does when JAVA_HOME=/usr points
// it back at itself. The stub's child keeps the output pipe open after the
// stub is killed, so the compile must also bound its wait for output.
func installBlockingJavac(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		// jdk.Javac only finds javac.exe; point the lookup at a batch stub.
		javac := filepath.Join(bin, "javac.cmd")
		if err := os.WriteFile(javac, []byte("@ping -n 60 127.0.0.1 >nul\r\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		original := javaFactsJavac
		javaFactsJavac = func() (string, error) { return javac, nil }
		t.Cleanup(func() { javaFactsJavac = original })
	} else {
		if err := os.WriteFile(filepath.Join(bin, "javac"), []byte("#!/bin/sh\nsleep 60\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("JAVA_HOME", home)
	}
	originalTimeout, originalDelay := javaFactsCompileTimeout, javaFactsCompileWaitDelay
	javaFactsCompileTimeout = 200 * time.Millisecond
	javaFactsCompileWaitDelay = 200 * time.Millisecond
	t.Cleanup(func() {
		javaFactsCompileTimeout, javaFactsCompileWaitDelay = originalTimeout, originalDelay
	})
}

// compileWithDeadline fails the test instead of hanging the package when
// compileJavaFactsHelper does not return.
func compileWithDeadline(t *testing.T) (string, string, error) {
	t.Helper()
	type result struct {
		classpath, warning string
		err                error
	}
	done := make(chan result, 1)
	go func() {
		classpath, cleanup, warning, err := compileJavaFactsHelper(nil)
		if cleanup != nil {
			cleanup()
		}
		done <- result{classpath, warning, err}
	}()
	select {
	case r := <-done:
		return r.classpath, r.warning, r.err
	case <-time.After(20 * time.Second):
		t.Fatal("compileJavaFactsHelper did not return for a javac that never exits")
		return "", "", nil
	}
}

func assertCompileTimedOutWithWarning(t *testing.T) {
	t.Helper()
	classpath, warning, err := compileWithDeadline(t)
	if err != nil {
		t.Fatalf("err = %v, want a warning only", err)
	}
	if classpath != "" {
		t.Fatalf("classpath = %q, want none", classpath)
	}
	if !strings.Contains(warning, "did not finish within") {
		t.Fatalf("warning = %q, want a compile timeout", warning)
	}
}

func TestCompileJavaFactsHelperTimesOutOnBlockingJavac(t *testing.T) {
	installBlockingJavac(t)
	original := javaFactsCacheRoot
	cacheRoot := t.TempDir()
	javaFactsCacheRoot = func() (string, error) { return cacheRoot, nil }
	t.Cleanup(func() { javaFactsCacheRoot = original })

	assertCompileTimedOutWithWarning(t)
	matches, err := filepath.Glob(filepath.Join(cacheRoot, "java-facts-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("timed-out compile left cache entries: %v", matches)
	}
}

func TestCompileJavaFactsHelperOneShotTimesOutOnBlockingJavac(t *testing.T) {
	installBlockingJavac(t)
	original := javaFactsCacheRoot
	javaFactsCacheRoot = func() (string, error) { return "", errors.New("no cache root") }
	t.Cleanup(func() { javaFactsCacheRoot = original })

	assertCompileTimedOutWithWarning(t)
}
