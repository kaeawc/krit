package pipeline

import (
	"context"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/module"
)

// buildFilesFingerprint hashes every Gradle file manifest and manifest-source rules can read: the build scripts and
// settings files under the scan paths (Project.GradlePaths), plus, for each Gradle build that holds a manifest but
// whose settings file the scan does not hold, the files its module graph is read from (#1800).
// ProviderReadPermissionOnly walks that graph, so a scan of only app/ and lib/ still depends on the settings file and
// on the build script of an intermediate module neither path contains. It returns false when a scanned path can't
// be hashed.
func buildFilesFingerprint(project *android.Project) (string, bool) {
	h := hashutil.Hasher().New()
	if project == nil {
		return hex.EncodeToString(h.Sum(nil)), true
	}
	if !writePathHashes(h, "gradle", project.GradlePaths) {
		return "", false
	}
	for _, root := range unscannedBuildRoots(project) {
		g, err := module.DiscoverModules(context.Background(), root)
		if err != nil || g == nil || g.BuildSystem != "" {
			continue
		}
		_, _ = h.Write([]byte("graph\x00" + root + "\x00"))
		paths := make([]string, 0, len(g.Modules))
		for path := range g.Modules {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			_, _ = h.Write([]byte(path + "\x00" + g.Modules[path].Dir + "\x00"))
		}
		memo := hashutil.Default()
		for _, file := range g.InputFiles() {
			_, _ = h.Write([]byte(file + "\x00"))
			// A missing name is part of the key: creating the file changes the graph.
			if sum, err := memo.HashFile(file, nil); err == nil {
				_, _ = h.Write([]byte(sum))
			} else {
				_, _ = h.Write([]byte{'-'})
			}
			_, _ = h.Write([]byte{0})
		}
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

// unscannedBuildRoots returns, sorted, the build roots of the project's manifests that no scanned settings file
// covers. A manifest below a directory whose settings file is in GradlePaths needs no lookup: the scan walked that
// directory, so every build script and nested settings file under it is in GradlePaths too. The others pay one
// upward search each, deduplicated by directory.
func unscannedBuildRoots(project *android.Project) []string {
	var settingsDirs []string
	for _, path := range project.GradlePaths {
		switch filepath.Base(path) {
		case "settings.gradle", "settings.gradle.kts":
			settingsDirs = append(settingsDirs, filepath.Dir(path))
		}
	}
	seenDirs := map[string]bool{}
	roots := map[string]bool{}
	for _, manifest := range project.ManifestPaths {
		dir := filepath.Dir(manifest)
		if seenDirs[dir] || underAny(dir, settingsDirs) {
			continue
		}
		seenDirs[dir] = true
		if root := module.BuildRoot(dir); root != "" {
			roots[root] = true
		}
	}
	out := make([]string, 0, len(roots))
	for root := range roots {
		out = append(out, root)
	}
	sort.Strings(out)
	return out
}

func underAny(dir string, parents []string) bool {
	for _, parent := range parents {
		if dir == parent || strings.HasPrefix(dir, parent+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
