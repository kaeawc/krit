package pipeline

import (
	"encoding/hex"
	"io/fs"
	"os"
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
	// UnsafeNativeCodeLocation also reads the files directly inside the source set's assets folder.
	if assets := resDirAssetsFingerprint(resDir); assets != "" {
		h := hashutil.Hasher().New()
		h.Write([]byte(fp))
		h.Write([]byte("\x00assets\x00"))
		h.Write([]byte(assets))
		fp = hex.EncodeToString(h.Sum(nil))
	}
	// Wear resource checks (WearBackNavigation, WearPasswordInput) decide from the module manifest whether the res
	// directory belongs to a Wear app, so adding or removing the watch feature must invalidate them too.
	if manifests := resDirManifestFingerprint(resDir); manifests != "" {
		h := hashutil.Hasher().New()
		h.Write([]byte(fp))
		h.Write([]byte("\x00manifest\x00"))
		h.Write([]byte(manifests))
		fp = hex.EncodeToString(h.Sum(nil))
	}
	// RelativeClassResolution reads the module's namespace, applicationId and plugins from its build script (and
	// plugin aliases from the version catalog), so editing them must invalidate its findings.
	if scripts := resDirBuildScriptFingerprint(resDir); scripts != "" {
		h := hashutil.Hasher().New()
		h.Write([]byte(fp))
		h.Write([]byte("\x00build\x00"))
		h.Write([]byte(scripts))
		fp = hex.EncodeToString(h.Sum(nil))
	}
	fps.values[resDir] = fp
	return fp
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// resDirBuildScriptFingerprint fingerprints the build script of the module that owns resDir (`<module>/src/<set>/res`
// or `<module>/res`), the nearest gradle/libs.versions.toml and settings script above it, or returns "" when none
// exists.
func resDirBuildScriptFingerprint(resDir string) string {
	module := filepath.Dir(resDir)
	if filepath.Base(filepath.Dir(module)) == "src" {
		module = filepath.Dir(filepath.Dir(module))
	}
	paths := []string{filepath.Join(module, "build.gradle.kts"), filepath.Join(module, "build.gradle")}
	// The nearest version catalog and settings script above the module: VectorDrawableCompat names the module's
	// Gradle path, which the settings script decides.
	catalog, settings := false, false
	for dir, depth := module, 0; depth < 8 && (!catalog || !settings); dir, depth = filepath.Dir(dir), depth+1 {
		if p := filepath.Join(dir, "gradle", "libs.versions.toml"); !catalog && fileExists(p) {
			paths, catalog = append(paths, p), true
		}
		for _, name := range []string{"settings.gradle.kts", "settings.gradle"} {
			if p := filepath.Join(dir, name); !settings && dir != module && fileExists(p) {
				paths, settings = append(paths, p), true
			}
		}
		if parent := filepath.Dir(dir); parent == dir {
			break
		}
	}
	h := hashutil.Hasher().New()
	memo := hashutil.Default()
	found := false
	for _, path := range paths {
		hx, err := memo.HashFile(path, nil)
		if err != nil {
			continue
		}
		found = true
		h.Write([]byte(filepath.Base(path)))
		h.Write([]byte{0})
		h.Write([]byte(hx))
	}
	if !found {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// resDirManifestFingerprint fingerprints the manifests a resource check reads for resDir: the AndroidManifest.xml of
// its source set and the module's main one, or returns "" when neither exists.
func resDirManifestFingerprint(resDir string) string {
	sourceSet := filepath.Dir(resDir)
	paths := []string{filepath.Join(sourceSet, "AndroidManifest.xml")}
	if filepath.Base(filepath.Dir(sourceSet)) == "src" && filepath.Base(sourceSet) != "main" {
		paths = append(paths, filepath.Join(filepath.Dir(sourceSet), "main", "AndroidManifest.xml"))
	}
	h := hashutil.Hasher().New()
	memo := hashutil.Default()
	found := false
	for _, path := range paths {
		hx, err := memo.HashFile(path, nil)
		if err != nil {
			continue
		}
		h.Write([]byte(path))
		h.Write([]byte{0})
		h.Write([]byte(hx))
		h.Write([]byte{0})
		found = true
	}
	if !found {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// resDirAssetsFingerprint fingerprints what UnsafeNativeCodeLocation reads from the assets folders of resDir (every
// source set's, see android.SourceSetAssetDirs): the names of the entries directly inside them that are ELF binaries,
// decided by android.IsELFFile exactly as the rule decides (following symlinks, reading the 4-byte header). Other
// content does not affect the findings, so it is not hashed; a large model or media asset costs one 4-byte read per
// run instead of a full hash. Returns "" when no asset folder holds an ELF binary.
func resDirAssetsFingerprint(resDir string) string {
	h := hashutil.Hasher().New()
	found := false
	for _, dir := range android.SourceSetAssetDirs(resDir) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		set := filepath.Base(filepath.Dir(dir))
		for _, e := range entries {
			if !android.IsELFFile(filepath.Join(dir, e.Name())) {
				continue
			}
			h.Write([]byte(set))
			h.Write([]byte{0})
			h.Write([]byte(e.Name()))
			h.Write([]byte{0})
			found = true
		}
	}
	if !found {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
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
