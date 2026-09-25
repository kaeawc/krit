package scan

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/javafacts"
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
	javac, lookupErr := exec.LookPath("javac")
	if lookupErr != nil {
		// Surfaces the lookup failure as a warning string, not an err
		// — javac being absent is non-fatal: the caller falls back to
		// pure-AST analysis.
		return "", nil, javafacts.UnavailableWarning(fmt.Errorf("javac not found")), nil //nolint:nilerr // lookup failure is intentionally surfaced as warning
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
	if output, compileErr := exec.CommandContext(context.Background(), javac, "--release", "17", "-d", classesDir, srcFile).CombinedOutput(); compileErr != nil {
		return "", nil, javafacts.UnavailableWarning(fmt.Errorf("compile helper (Java facts need JDK >= 17): %w: %s", compileErr, string(output))), nil //nolint:nilerr // compile failure is intentionally surfaced as warning
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
	if output, compileErr := exec.CommandContext(context.Background(), javac, "--release", "17", "-d", classesDir, srcFile).CombinedOutput(); compileErr != nil {
		cleanup()
		return "", nil, javafacts.UnavailableWarning(fmt.Errorf("compile helper (Java facts need JDK >= 17): %w: %s", compileErr, string(output))), nil //nolint:nilerr // compile failure is intentionally surfaced as warning
	}
	return classesDir, cleanup, "", nil
}
