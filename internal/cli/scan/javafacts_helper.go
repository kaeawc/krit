package scan

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/javafacts"
	"github.com/kaeawc/krit/internal/jdk"
	javafactshelper "github.com/kaeawc/krit/tools/krit-java-facts"
)

// javaFactsCacheRoot returns the directory under which compiled helper
// classes are cached, keyed by the embedded source content.
var javaFactsCacheRoot = func() (string, error) {
	dir, err := fsutil.UserKritDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "jars"), nil
}

// javaFactsJavac locates javac; tests replace it to run a stand-in compiler.
var javaFactsJavac = jdk.Javac

// javaFactsCompileTimeout bounds one javac run of the helper. A healthy JDK
// compiles the single helper file in seconds; the limit leaves room for a cold
// CI runner while keeping a javac that never exits (such as macOS's
// /usr/bin/javac stub under a misconfigured JAVA_HOME) from hanging the scan.
var javaFactsCompileTimeout = 2 * time.Minute

// javaFactsCompileWaitDelay bounds how long Wait keeps reading javac's output
// after the timeout kills it, in case a child process still holds the pipes.
var javaFactsCompileWaitDelay = 5 * time.Second

// javacIdentity is a cheap, JVM-free fingerprint of a javac binary: its
// symlink-resolved path plus that file's size and mtime. Folded into the helper
// cache key so switching JDKs invalidates compiled classes from another JDK.
type javacIdentity struct {
	path    string
	size    int64
	modTime int64
}

// resolveJavacIdentity returns javac's identity for cache-keying purposes.
// It falls back to a deterministic degraded identity if resolution or stat
// fails, without running a JVM.
var resolveJavacIdentity = func(javac string) javacIdentity {
	resolved, err := filepath.EvalSymlinks(javac)
	if err != nil {
		return javacIdentity{path: javac}
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return javacIdentity{path: resolved}
	}
	return javacIdentity{path: resolved, size: info.Size(), modTime: info.ModTime().UnixNano()}
}

// compileJavaFactsHelper compiles the embedded helper with --release 17 into
// javaFactsCacheRoot()/java-facts-<key>, keyed by embedded source, javac identity,
// and JAVA_HOME. The .complete marker denotes a usable cache entry.
// Concurrent callers populate via rename-into-place; a loser reuses the winner's completed entry.
func compileJavaFactsHelper(_ []string) (classpath string, cleanup func(), warning string, err error) {
	javac, lookupErr := javaFactsJavac()
	if lookupErr != nil {
		// Surfaces the lookup failure as a warning string, not an err
		// — javac being absent is non-fatal: the caller falls back to
		// pure-AST analysis.
		return "", nil, javafacts.UnavailableWarning(fmt.Errorf("javac not found: %w", lookupErr)), nil //nolint:nilerr // lookup failure is intentionally surfaced as warning
	}
	identity := resolveJavacIdentity(javac)

	jarsRoot, rootErr := javaFactsCacheRoot()
	if rootErr != nil {
		return compileJavaFactsHelperOneShot(javac)
	}
	if err := os.MkdirAll(jarsRoot, 0o700); err != nil {
		return compileJavaFactsHelperOneShot(javac)
	}
	h := sha256.New()
	_, _ = h.Write(javafactshelper.Source)
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(identity.path))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(os.Getenv("JAVA_HOME")))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte("17"))
	_, _ = h.Write([]byte{0})
	var numeric [8]byte
	binary.LittleEndian.PutUint64(numeric[:], uint64(identity.size))
	_, _ = h.Write(numeric[:])
	binary.LittleEndian.PutUint64(numeric[:], uint64(identity.modTime))
	_, _ = h.Write(numeric[:])
	digest := h.Sum(nil)
	key := hex.EncodeToString(digest[:])[:16]
	cacheDir := filepath.Join(jarsRoot, "java-facts-"+key)
	if _, err := os.Stat(filepath.Join(cacheDir, ".complete")); err == nil {
		return cacheDir, func() {}, "", nil
	}

	tmp, err := os.MkdirTemp(jarsRoot, "java-facts-"+key+"-tmp-*")
	if err != nil {
		return "", nil, "", err
	}
	defer os.RemoveAll(tmp)
	srcFile := filepath.Join(tmp, "src", "dev", "jasonpearson", "krit", "javafacts", "Main.java")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0o700); err != nil {
		return "", nil, "", err
	}
	if err := os.WriteFile(srcFile, javafactshelper.Source, 0o600); err != nil {
		return "", nil, "", err
	}
	classesDir := filepath.Join(tmp, "classes")
	if warning := runJavaFactsJavac(javac, classesDir, srcFile); warning != "" {
		return "", nil, warning, nil
	}
	if err := os.WriteFile(filepath.Join(classesDir, ".complete"), nil, 0o600); err != nil {
		return "", nil, "", err
	}
	if err := os.Rename(classesDir, cacheDir); err != nil {
		if _, statErr := os.Stat(filepath.Join(cacheDir, ".complete")); statErr != nil {
			return "", nil, "", err
		}
	}
	return cacheDir, func() {}, "", nil
}

func compileJavaFactsHelperOneShot(javac string) (classpath string, cleanup func(), warning string, err error) {
	tmp, err := os.MkdirTemp("", "krit-java-facts-helper-*")
	if err != nil {
		return "", nil, "", err
	}
	cleanup = func() { _ = os.RemoveAll(tmp) }
	srcFile := filepath.Join(tmp, "src", "dev", "jasonpearson", "krit", "javafacts", "Main.java")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0o700); err != nil {
		cleanup()
		return "", nil, "", err
	}
	if err := os.WriteFile(srcFile, javafactshelper.Source, 0o600); err != nil {
		cleanup()
		return "", nil, "", err
	}
	classesDir := filepath.Join(tmp, "classes")
	if warning := runJavaFactsJavac(javac, classesDir, srcFile); warning != "" {
		cleanup()
		return "", nil, warning, nil
	}
	return classesDir, cleanup, "", nil
}

// runJavaFactsJavac compiles the helper source into classesDir. A failed or
// timed-out compile returns the "Java facts unavailable" warning, so the scan
// falls back to AST-only Java analysis.
func runJavaFactsJavac(javac, classesDir, srcFile string) string {
	ctx, cancel := context.WithTimeout(context.Background(), javaFactsCompileTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, javac, "--release", "17", "-d", classesDir, srcFile)
	cmd.WaitDelay = javaFactsCompileWaitDelay
	output, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return javafacts.UnavailableWarning(fmt.Errorf("compile helper: %s did not finish within %s (is JAVA_HOME a real JDK?)", javac, javaFactsCompileTimeout))
	}
	if err != nil {
		return javafacts.UnavailableWarning(fmt.Errorf("compile helper (Java facts need JDK >= 17): %w: %s", err, string(output)))
	}
	return ""
}
