package javafacts

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// linuxMaxArgStrlen is Linux's MAX_ARG_STRLEN: the most bytes one execve
// argument may hold, whatever ARG_MAX is.
const linuxMaxArgStrlen = 128 * 1024

func TestLaunchArgsManyFilesUseArgfile(t *testing.T) {
	files := make([]string, 100_001)
	for i := range files {
		files[i] = fmt.Sprintf("/repo/module-%d/src/main/java/p/File%d.java", i%2000, i)
	}
	opts := DefaultOptions()
	want := append([]string{"-cp", "/helper/classes", opts.HelperMain, "--out", "/tmp/facts.json"}, files...)
	assertSingleArgfile(t, "/helper/classes", files, opts, want)
}

func TestLaunchArgsLongClasspathUseArgfile(t *testing.T) {
	opts := DefaultOptions()
	opts.Classpath = longClasspath(func(i int) string {
		return fmt.Sprintf("/gradle/caches/module-%05d/build/classes/java/main", i)
	})
	files := []string{"/repo/src/A.java"}
	want := []string{"-cp", "/helper/classes", opts.HelperMain, "--out", "/tmp/facts.json", "--classpath", opts.Classpath, "/repo/src/A.java"}
	assertSingleArgfile(t, "/helper/classes", files, opts, want)
}

func TestLaunchArgsShortArgvUnchanged(t *testing.T) {
	opts := DefaultOptions()
	opts.Classpath = "/lib/a.jar"
	got, cleanup, err := launchArgs("/helper/classes", "/tmp/facts.json", []string{"/repo/A.java", "/repo/B.java"}, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	want := []string{"-cp", "/helper/classes", opts.HelperMain, "--out", "/tmp/facts.json", "--classpath", "/lib/a.jar", "/repo/A.java", "/repo/B.java"}
	if !slices.Equal(got, want) {
		t.Fatalf("short argv changed:\n got %q\nwant %q", got, want)
	}
}

// longClasspath joins entry(0), entry(1), ... until the classpath is longer
// than Linux allows in a single argument.
func longClasspath(entry func(int) string) string {
	var b strings.Builder
	for i := 0; b.Len() <= linuxMaxArgStrlen; i++ {
		if i > 0 {
			b.WriteRune(os.PathListSeparator)
		}
		b.WriteString(entry(i))
	}
	return b.String()
}

func assertSingleArgfile(t *testing.T, helperClasspath string, files []string, opts Options, want []string) {
	t.Helper()
	got, cleanup, err := launchArgs(helperClasspath, "/tmp/facts.json", files, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if len(got) != 1 || !strings.HasPrefix(got[0], "@") {
		t.Fatalf("argv is not a single @argfile: %d args, first %.80q", len(got), got[0])
	}
	body, err := os.ReadFile(strings.TrimPrefix(got[0], "@"))
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(strings.TrimSuffix(string(body), "\n"), "\n"); !slices.Equal(lines, want) {
		t.Fatalf("argfile holds %d args, want %d (or contents differ)", len(lines), len(want))
	}
}

// TestInvokeHelperReadsArgfile runs the real helper with a classpath longer
// than Linux allows in one argument, so the launch must go through an argfile
// that the java launcher expands into the helper's own arguments.
func TestInvokeHelperReadsArgfile(t *testing.T) {
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac not available")
	}
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not available")
	}
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	helper := filepath.Join(repoRoot, "tools", "krit-java-facts", "src", "main", "java", "dev", "jasonpearson", "krit", "javafacts", "Main.java")
	classes := filepath.Join(t.TempDir(), "classes")
	if out, err := exec.Command(javac, "-d", classes, helper).CombinedOutput(); err != nil {
		t.Fatalf("compile helper: %v\n%s", err, out)
	}
	srcRoot := filepath.Join(t.TempDir(), "src")
	writeFile(t, filepath.Join(srcRoot, "android", "os", "Handler.java"), `package android.os; public class Handler {}`)
	target := filepath.Join(srcRoot, "test", "MyHandler.java")
	writeFile(t, target, `package test;
class MyHandler extends android.os.Handler {}`)
	missing := t.TempDir()
	opts := DefaultOptions()
	opts.Java = java
	opts.Classpath = srcRoot + string(os.PathListSeparator) + longClasspath(func(i int) string {
		return filepath.Join(missing, fmt.Sprintf("missing-%05d.jar", i))
	})
	opts.Timeout = 0
	facts, warning, err := Invoke(context.Background(), classes, []string{target}, opts, nil)
	if err != nil || warning != "" {
		t.Fatalf("Invoke: err=%v warning=%q", err, warning)
	}
	for _, class := range facts.Classes {
		if class.QualifiedName == "test.MyHandler" && slices.Contains(class.Supertypes, "android.os.Handler") {
			return
		}
	}
	t.Fatalf("helper did not resolve test.MyHandler against the argfile classpath: %+v", facts.Classes)
}
