// Package devjar identifies local JVM helper builds across git worktrees.
package devjar

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SourceHash hashes build inputs by relative path and contents. The shared
// rule API and bundled notice/license files are inputs to both helper jars.
func SourceHash(root, tool string) (string, error) {
	if tool != "krit-fir" && tool != "krit-types" {
		return "", fmt.Errorf("unsupported dev jar %q", tool)
	}
	paths := []string{
		"LICENSE", "tools/THIRD_PARTY_NOTICES.txt",
		filepath.Join("tools", tool, "build.gradle.kts"),
		filepath.Join("tools", tool, "settings.gradle.kts"),
		filepath.Join("tools", tool, "gradle.properties"),
		filepath.Join("tools", tool, "gradle", "wrapper", "gradle-wrapper.properties"),
		"tools/krit-rule-api/build.gradle.kts",
		"tools/krit-rule-api/settings.gradle.kts",
		"tools/krit-rule-api/gradle.properties",
		"tools/krit-rule-api/gradle/wrapper/gradle-wrapper.properties",
	}
	for _, dir := range []string{filepath.Join("tools", tool, "src", "main"), "tools/krit-rule-api/src/main"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			paths = append(paths, rel)
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(paths)
	h := sha256.New()
	// KRIT_VERSION is baked into the jars' RuleApiVersion, so jars built for
	// different versions must not share a cache key.
	writeRecord(h, []byte("KRIT_VERSION"))
	writeRecord(h, []byte(strings.TrimSpace(os.Getenv("KRIT_VERSION"))))
	for _, rel := range paths {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return "", err
		}
		// Length-prefixed path plus a fixed-size content digest keeps each
		// record unambiguous, so no file's bytes can mimic a record boundary.
		sum := sha256.Sum256(data)
		writeRecord(h, []byte(filepath.ToSlash(rel)))
		_, _ = h.Write(sum[:])
	}
	return hex.EncodeToString(h.Sum(nil))[:20], nil
}

func writeRecord(h hash.Hash, data []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(data)))
	_, _ = h.Write(size[:])
	_, _ = h.Write(data)
}

// CheckoutRoot finds a local source checkout, starting at the current working
// directory. Scan paths are fallback hints for callers outside the checkout.
func CheckoutRoot(tool string, scanPaths []string) string {
	cwd, _ := os.Getwd()
	for _, start := range append([]string{cwd}, scanPaths...) {
		if start == "" {
			continue
		}
		abs, err := filepath.Abs(start)
		if err != nil {
			continue
		}
		if info, err := os.Stat(abs); err == nil && !info.IsDir() {
			abs = filepath.Dir(abs)
		}
		for dir := abs; ; dir = filepath.Dir(dir) {
			if info, err := os.Stat(filepath.Join(dir, "tools", tool, "build.gradle.kts")); err == nil && !info.IsDir() {
				return dir
			}
			if parent := filepath.Dir(dir); parent == dir || strings.TrimSpace(dir) == "" {
				break
			}
		}
	}
	return ""
}

// DirEnv overrides the shared development jar cache root, which otherwise
// defaults to ~/.krit/jars/dev. Tests point it at a temp dir so a jar built
// with `make fir-jar` or `make types-jar` can't leak into jar lookups.
const DirEnv = "KRIT_DEV_JAR_DIR"

// CacheDir returns the shared development jar cache root: $KRIT_DEV_JAR_DIR
// when set, else ~/.krit/jars/dev. Returns "" when neither is available.
func CacheDir() string {
	if dir := os.Getenv(DirEnv); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".krit", "jars", "dev")
}

// CachePath returns a checkout-specific shared cache path. A missing checkout
// or cache root simply disables the additive development lookup.
func CachePath(tool string, scanPaths []string) string {
	root := CheckoutRoot(tool, scanPaths)
	if root == "" {
		return ""
	}
	hash, err := SourceHash(root, tool)
	if err != nil {
		return ""
	}
	dir := CacheDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, hash, tool+".jar")
}
