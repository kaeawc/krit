package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// matrixBaselineCacheEntry is the on-disk wrapper around a cached
// baseline matrixCaseReport. The cacheKey is stored redundantly so
// consumers can sanity-check the filename vs. contents before trusting
// the payload.
//
// SampleFindingKeys sidecars the per-sample FindingKeys slices that
// matrixSample intentionally excludes from its public JSON encoding
// (json:"-"). Without it, the loaded baseline would have empty finding
// sets and applyMatrixDiffs would report every singles-case finding as
// "introduced".
type matrixBaselineCacheEntry struct {
	KritVersion       string           `json:"kritVersion"`
	GeneratedAt       string           `json:"generatedAt"`
	CacheKey          string           `json:"cacheKey"`
	Report            matrixCaseReport `json:"report"`
	SampleFindingKeys [][]string       `json:"sampleFindingKeys"`
}

// matrixCacheExcludeDirs lists directory basenames that are skipped
// when hashing a non-git target. Keeping this list conservative and
// short avoids walking huge irrelevant trees (build outputs, caches,
// dependency stores).
var matrixCacheExcludeDirs = map[string]bool{
	".git":         true,
	".gradle":      true,
	".idea":        true,
	"node_modules": true,
	".krit-cache":  true,
	"build":        true,
	"out":          true,
}

// computeMatrixBaselineCacheKey hashes all inputs that could affect
// the baseline experiment case output for a given matrix invocation.
// Any change to the krit binary, target source tree(s), baseline
// experiment flags, or relevant CLI flags produces a new key.
func computeMatrixBaselineCacheKey(exe string, baselineEnabled []string, flagArgs []string, targets []string) (string, error) {
	h := sha256.New()

	// 1. Absolute, sorted target paths.
	absTargets := make([]string, 0, len(targets))
	for _, t := range targets {
		abs, err := filepath.Abs(t)
		if err != nil {
			return "", fmt.Errorf("abs target %s: %w", t, err)
		}
		absTargets = append(absTargets, abs)
	}
	sort.Strings(absTargets)
	fmt.Fprintf(h, "targets:%d\n", len(absTargets))
	for _, t := range absTargets {
		fmt.Fprintf(h, "  %s\n", t)
	}

	// 2. Krit binary identity.
	exeHash, err := hashFileContents(exe)
	if err != nil {
		return "", fmt.Errorf("hash executable: %w", err)
	}
	fmt.Fprintf(h, "exe:%s\n", exeHash)

	// 3. Target source tree identity per target.
	for _, t := range absTargets {
		treeHash, err := hashTargetTree(t)
		if err != nil {
			return "", fmt.Errorf("hash tree %s: %w", t, err)
		}
		fmt.Fprintf(h, "tree:%s:%s\n", t, treeHash)
	}

	// 4. Baseline experiment flags (usually empty).
	baselineSorted := append([]string(nil), baselineEnabled...)
	sort.Strings(baselineSorted)
	fmt.Fprintf(h, "enabled:%s\n", strings.Join(baselineSorted, ","))

	// 5. Stripped flag args (reuse matrix runner's strip logic so we
	// never bake driver flags like -experiment-matrix into the key).
	stripped := stripExperimentMatrixArgs(flagArgs)
	sort.Strings(stripped)
	fmt.Fprintf(h, "args:%d\n", len(stripped))
	for _, a := range stripped {
		fmt.Fprintf(h, "  %s\n", a)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashFileContents(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashTargetTree returns an identity hash for the target source tree.
// Git working trees are hashed from HEAD + porcelain status so we avoid
// walking the whole tree; non-git directories fall back to a mtime/size
// walk with a short exclude list.
func hashTargetTree(target string) (string, error) {
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		// Non-dir (file or stat error): fall back to file hash or empty.
		if err == nil {
			return hashFileContents(target)
		}
		return "", err
	}

	// Git working tree probe.
	if _, err := os.Stat(filepath.Join(target, ".git")); err == nil {
		if gitHash, ok := hashGitTree(target); ok {
			return gitHash, nil
		}
	}
	return hashFilesystemTree(target)
}

func hashGitTree(target string) (string, bool) {
	revCmd := exec.Command("git", "-C", target, "rev-parse", "HEAD")
	revOut, err := revCmd.Output()
	if err != nil {
		return "", false
	}
	statCmd := exec.Command("git", "-C", target, "status", "--porcelain")
	statOut, err := statCmd.Output()
	if err != nil {
		return "", false
	}
	h := sha256.New()
	h.Write(revOut)
	h.Write([]byte{0})
	h.Write(statOut)
	return "git:" + hex.EncodeToString(h.Sum(nil)), true
}

func hashFilesystemTree(target string) (string, error) {
	h := sha256.New()
	err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Tolerate individual stat errors — skip and keep walking.
			return nil
		}
		if info.IsDir() {
			if matrixCacheExcludeDirs[info.Name()] && path != target {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(target, path)
		if relErr != nil {
			rel = path
		}
		fmt.Fprintf(h, "%s\x00%d\x00%d\n", rel, info.ModTime().UnixNano(), info.Size())
		return nil
	})
	if err != nil {
		return "", err
	}
	return "fs:" + hex.EncodeToString(h.Sum(nil)), nil
}

// matrixCacheDir resolves the directory where baseline cache entries
// are stored. Prefers ~/.cache/krit/matrix-baseline, falls back to the
// OS temp dir if $HOME is unset or the primary path is unwritable.
func matrixCacheDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dir := filepath.Join(home, ".cache", "krit", "matrix-baseline")
		if err := os.MkdirAll(dir, 0o755); err == nil {
			return dir
		}
	}
	fallback := filepath.Join(os.TempDir(), "krit-matrix-baseline")
	if err := os.MkdirAll(fallback, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "krit: warning: cannot create cache dir %s: %v\n", fallback, err)
	}
	return fallback
}

func matrixCachePath(key string) string {
	return filepath.Join(matrixCacheDir(), key+".json")
}

// tryLoadBaseline attempts to load a cached baseline report for the
// given cache key. Returns (report, true) on a verified hit and
// (nil, false) on any miss, error, or key mismatch — callers must be
// prepared to fall through to a fresh run on miss.
func tryLoadBaseline(key string) (*matrixCaseReport, bool) {
	if key == "" {
		return nil, false
	}
	path := matrixCachePath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var entry matrixBaselineCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}
	if entry.CacheKey != key {
		return nil, false
	}
	report := entry.Report
	// Restore per-sample FindingKeys (json:"-" on matrixSample drops
	// them from the normal report encoding, so they live in a sidecar).
	if len(entry.SampleFindingKeys) == len(report.Samples) {
		for i := range report.Samples {
			report.Samples[i].FindingKeys = append([]string(nil), entry.SampleFindingKeys[i]...)
		}
	}
	return &report, true
}

// saveBaseline writes a freshly-computed baseline report to the cache.
// All errors are swallowed — the cache is best-effort and must never
// fail the outer matrix run.
func saveBaseline(key string, report matrixCaseReport) {
	if key == "" {
		return
	}
	sampleKeys := make([][]string, len(report.Samples))
	for i, s := range report.Samples {
		sampleKeys[i] = append([]string(nil), s.FindingKeys...)
	}
	entry := matrixBaselineCacheEntry{
		KritVersion:       version,
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
		CacheKey:          key,
		Report:            report,
		SampleFindingKeys: sampleKeys,
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return
	}
	path := matrixCachePath(key)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		fmt.Fprintf(os.Stderr, "krit: warning: cache write failed for %s: %v\n", path, err)
		if rmErr := os.Remove(tmp); rmErr != nil && !os.IsNotExist(rmErr) {
			fmt.Fprintf(os.Stderr, "krit: warning: temp file cleanup failed for %s: %v\n", tmp, rmErr)
		}
	}
}

// clearMatrixCache removes every cached baseline entry. It is safe to
// call when the cache directory does not exist.
func clearMatrixCache() error {
	dir := matrixCacheDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
