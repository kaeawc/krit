package oracle

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// MergeFreshIntoCachedTypes merges fresh per-file oracle facts from a
// partial reanalyze into the cached types.json and persists the result.
// Fresh entries win on overlap (files and dependencies); cached entries
// without a fresh counterpart are preserved. Top-level Version /
// KotlinVersion are kept from cache when fresh leaves them zero.
func MergeFreshIntoCachedTypes(outputPath string, fresh *Data) (*Data, bool, error) {
	if outputPath == "" {
		return nil, false, fmt.Errorf("merge: empty outputPath")
	}
	if fresh == nil {
		fresh = &Data{Files: map[string]*File{}, Dependencies: map[string]*Class{}}
	}
	cached, err := readOracleJSON(outputPath)
	cacheMissing := err != nil
	if cacheMissing {
		cached = &Data{Files: map[string]*File{}, Dependencies: map[string]*Class{}}
	}
	merged, pruned := mergeOracleData(cached, fresh, func(path string) error {
		_, err := os.Stat(path)
		return err
	})
	// Skip the write when the merge is a no-op — fresh was empty AND
	// the cached file was readable. Saves a ~1MB re-marshal on the
	// hot warm path where the caller passed a hint that produced no
	// fresh facts (e.g. all stale paths resolved to cache hits after
	// per-file content-hash check).
	if !cacheMissing && !pruned && len(fresh.Files) == 0 && len(fresh.Dependencies) == 0 {
		return merged, pruned, nil
	}
	if err := writeOracleJSON(outputPath, merged); err != nil {
		return nil, pruned, fmt.Errorf("merge: write types.json: %w", err)
	}
	return merged, pruned, nil
}

// WriteTypesJSON persists a full oracle result as the cached types.json, so a
// caller that replaces a partial merge with a full analysis leaves the full
// result on disk for the next partial run to merge into.
func WriteTypesJSON(outputPath string, data *Data) error {
	return writeOracleJSON(outputPath, data)
}

// mergeOracleData performs the section-wise union described in
// MergeFreshIntoCachedTypes. Extracted so unit tests can exercise the
// merge rules without touching disk.
func mergeOracleData(cached, fresh *Data, stat func(string) error) (*Data, bool) {
	if cached == nil {
		cached = &Data{}
	}
	if fresh == nil {
		fresh = &Data{}
	}
	merged := &Data{
		Version:       cached.Version,
		KotlinVersion: cached.KotlinVersion,
		Files:         make(map[string]*File, len(cached.Files)+len(fresh.Files)),
		Dependencies:  make(map[string]*Class, len(cached.Dependencies)+len(fresh.Dependencies)),
	}
	if fresh.Version != 0 {
		merged.Version = fresh.Version
	}
	if fresh.KotlinVersion != "" {
		merged.KotlinVersion = fresh.KotlinVersion
	}
	pruned := false
	for path, f := range cached.Files {
		if keepOnStatErr(stat(path)) {
			merged.Files[path] = f
		} else {
			pruned = true
		}
	}
	for path, f := range fresh.Files {
		merged.Files[path] = f
	}
	for fqn, c := range cached.Dependencies {
		merged.Dependencies[fqn] = c
	}
	for fqn, c := range fresh.Dependencies {
		merged.Dependencies[fqn] = c
	}
	return merged, pruned
}

// keepOnStatErr reports whether a cached file should survive a stat error.
// Only a definite not-exist result proves the source file was deleted; other
// errors may be transient and must not discard cached facts.
func keepOnStatErr(err error) bool {
	return err == nil || !errors.Is(err, fs.ErrNotExist)
}
