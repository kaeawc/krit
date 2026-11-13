package pipeline

import (
	"encoding/hex"
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
)

type resDirFingerprints struct {
	values map[string]string
	cache  *resDirFingerprintCache
}

func (fps *resDirFingerprints) fingerprint(resDir string) string {
	if fps == nil {
		var tmp resDirFingerprints
		return tmp.fingerprint(resDir)
	}
	if fps.values == nil {
		fps.values = make(map[string]string)
	}
	if fp, ok := fps.values[resDir]; ok {
		return fp
	}
	fp := fps.dirFingerprint(resDir)
	// Resource rules such as the translation checks also read the module's other source-set res directories, so
	// a change there must invalidate findings cached under this resDir.
	if siblings := android.SiblingSourceSetResDirs(resDir); len(siblings) > 0 {
		h := hashutil.Hasher().New()
		h.Write([]byte(fp))
		for _, sib := range siblings {
			h.Write([]byte{0})
			h.Write([]byte(filepath.Base(filepath.Dir(sib))))
			h.Write([]byte{0})
			h.Write([]byte(fps.dirFingerprint(sib)))
		}
		fp = hex.EncodeToString(h.Sum(nil))
	}
	fps.values[resDir] = fp
	return fp
}

// dirFingerprint fingerprints the content of one directory, reusing the persistent cache when present.
func (fps *resDirFingerprints) dirFingerprint(dir string) string {
	if fps.cache != nil {
		return fps.cache.fingerprint(dir)
	}
	return resDirContentFingerprint(dir)
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

// iconInputFingerprint covers everything the icon index reads: the res directory and the manifest beside it.
func iconInputFingerprint(resDir string) string {
	return resDirContentFingerprint(resDir) + "\x00" + iconManifestFingerprint(resDir)
}

// iconManifestFingerprint hashes the manifest the icon index reads for a res directory, which assigns
// launcher and round icons and sets minSdk.
func iconManifestFingerprint(resDir string) string {
	if hx, err := hashutil.Default().HashFile(android.IconManifestPath(resDir), nil); err == nil {
		return hx
	}
	return "no-manifest"
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
		fp := fps.fingerprint(dir)
		h.Write([]byte(dir))
		h.Write([]byte{0})
		h.Write([]byte(fp))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
