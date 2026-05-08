package pipeline

import (
	"encoding/hex"
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/kaeawc/krit/internal/hashutil"
)

type resDirFingerprints struct {
	values map[string]string
	cache  *resDirFingerprintCache
}

func (fps *resDirFingerprints) fingerprint(resDir string) string {
	if fps == nil {
		return resDirContentFingerprint(resDir)
	}
	if fps.values == nil {
		fps.values = make(map[string]string)
	}
	if fp, ok := fps.values[resDir]; ok {
		return fp
	}
	var fp string
	if fps.cache != nil {
		fp = fps.cache.fingerprint(resDir)
	} else {
		fp = resDirContentFingerprint(resDir)
	}
	fps.values[resDir] = fp
	return fp
}

func newResDirFingerprints(cacheDir string, capacity int) resDirFingerprints {
	return resDirFingerprints{
		values: make(map[string]string, capacity),
		cache:  newResDirFingerprintCache(cacheDir),
	}
}

// resDirContentFingerprint computes a stable fingerprint of a resource
// directory's content. It walks every regular file under resDir,
// sorts paths, and hashes (relPath, contentHash) for each. Reads land
// in hashutil.Default()'s memo so subsequent calls with the same
// (size, mtime) tuples are free.
//
// A walk error is folded into the fingerprint as a sentinel so a
// transient I/O failure produces a different (cache-missing) FP rather
// than silently matching a successful read.
func resDirContentFingerprint(resDir string) string {
	h := hashutil.Hasher().New()
	memo := hashutil.Default()
	var paths []string
	walkErr := filepath.WalkDir(resDir, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			// Continue walking; record the error contribution
			// after we've sorted the rest.
			return nil //nolint:nilerr // intentional: walk errors are folded into the fingerprint sentinel
		}
		if d.IsDir() {
			return nil
		}
		paths = append(paths, p)
		return nil
	})
	sort.Strings(paths)
	for _, p := range paths {
		hx, err := memo.HashFile(p, nil)
		rel, _ := filepath.Rel(resDir, p)
		h.Write([]byte(rel))
		h.Write([]byte{0})
		if err != nil {
			h.Write([]byte("\x00err\x00"))
			continue
		}
		h.Write([]byte(hx))
		h.Write([]byte{0})
	}
	if walkErr != nil {
		h.Write([]byte("\x00walk-err\x00"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// mergedResourceIndexFingerprint composes per-resDir content fingerprints
// into one stable hex digest. Sorting the dirs first makes the result
// order-independent; including each dir's path prevents two identical-content
// dirs at different paths from colliding.
func mergedResourceIndexFingerprint(resDirs []string) string {
	return mergedResourceIndexFingerprintWith(resDirs, nil)
}

func mergedResourceIndexFingerprintWith(resDirs []string, fps *resDirFingerprints) string {
	h := hashutil.Hasher().New()
	sorted := append([]string(nil), resDirs...)
	sort.Strings(sorted)
	for _, dir := range sorted {
		var fp string
		if fps != nil {
			fp = fps.fingerprint(dir)
		} else {
			fp = resDirContentFingerprint(dir)
		}
		h.Write([]byte(dir))
		h.Write([]byte{0})
		h.Write([]byte(fp))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
