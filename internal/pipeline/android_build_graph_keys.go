package pipeline

import (
	"context"
	"encoding/hex"
	"hash"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/rules"
)

// buildFilesFingerprint hashes every Gradle file manifest and manifest-source rules can read: the build scripts and
// settings files under the scan paths (Project.GradlePaths), and, when readsGraph, the files the module graph of
// each manifest's Gradle build is read from that the scan didn't hash (#1800). ProviderReadPermissionOnly walks that
// graph, which reaches build files no scan path holds: a scan of only app/ and lib/, an explicit file list, a module
// that is gitignored or mapped into a dot-directory or build/. It also reads each manifest's own build script (is the
// module an app, its namespace), which a build with no settings file has no graph for and a scan of only source
// directories doesn't hold, so those candidates are hashed too. It returns false when a scanned path can't be hashed.
func buildFilesFingerprint(project *android.Project, readsGraph bool) (string, bool) {
	h := hashutil.Hasher().New()
	if project == nil {
		return hex.EncodeToString(h.Sum(nil)), true
	}
	if !writePathHashes(h, "gradle", project.GradlePaths) {
		return "", false
	}
	if !readsGraph {
		return hex.EncodeToString(h.Sum(nil)), true
	}
	scanned := make(map[string]bool, len(project.GradlePaths))
	for _, path := range project.GradlePaths {
		scanned[path] = true
	}
	memo := hashutil.Default()
	inputs := projectBuildGraphs(project)
	for _, build := range inputs.layouts {
		_, _ = h.Write([]byte("graph\x00" + build.root + "\x00"))
		for _, m := range build.modules {
			_, _ = h.Write([]byte(m + "\x00"))
		}
		writeUnscannedFileHashes(h, memo, build.files, scanned)
	}
	_, _ = h.Write([]byte("scripts\x00"))
	writeUnscannedFileHashes(h, memo, inputs.scripts, scanned)
	// Every directory a catalog search globbed, with what it found: a catalog created in one changes what is read.
	_, _ = h.Write([]byte("catalogs\x00"))
	for _, c := range inputs.catalogDirs {
		_, _ = h.Write([]byte(c.dir + "\x00" + strings.Join(c.matches, "\x00") + "\x01"))
	}
	writeUnscannedFileHashes(h, memo, inputs.catalogs, scanned)
	return hex.EncodeToString(h.Sum(nil)), true
}

// writeUnscannedFileHashes writes the path and content hash of every file the scan didn't hash. A missing file is
// part of the key: creating it changes what is read.
func writeUnscannedFileHashes(h hash.Hash, memo *hashutil.Memo, files []string, scanned map[string]bool) {
	for _, file := range files {
		if scanned[file] {
			continue
		}
		_, _ = h.Write([]byte(file + "\x00"))
		if sum, err := memo.HashFile(file, nil); err == nil {
			_, _ = h.Write([]byte(sum))
		} else {
			_, _ = h.Write([]byte{'-'})
		}
		_, _ = h.Write([]byte{0})
	}
}

// buildGraphLayout is the part of one Gradle build's module graph a cache key needs: its root, its modules as
// "path\x00dir", and the files the graph is read from (module.Graph.InputFiles).
type buildGraphLayout struct {
	root    string
	modules []string
	files   []string
}

// buildGraphInputs is what the module-graph rules read from disk beyond the scan: the layout of every build that
// holds a manifest they read the graph for, sorted by root; the build-script candidates of every manifest
// (rules.ManifestBuildScriptCandidates) that no layout lists, sorted; and the version-catalog search of every
// manifest's module directory (rules.VersionCatalogSearch), which can make a module an app through a plugin alias:
// each globbed directory with its matches, sorted by directory, and the matched catalogs, sorted.
type buildGraphInputs struct {
	layouts     []buildGraphLayout
	scripts     []string
	catalogDirs []catalogDir
	catalogs    []string
}

// catalogDir is one directory a version-catalog search globbed and the catalogs it found there.
type catalogDir struct {
	dir     string
	matches []string
}

// buildGraphMemo shares projectBuildGraphs between the findings-bundle and manifest-source keys of one run. An entry
// is valid for one generation of the shared hashutil Memo, which the CLI and the daemon clear at the start of every
// scan, and for one set of manifest and Gradle paths. It only feeds cache keys, so runs without caches never read a
// stale one.
var buildGraphMemo struct {
	mu         sync.Mutex
	generation uint64
	identity   string
	inputs     buildGraphInputs
	valid      bool
}

// projectBuildGraphs returns the build files the module-graph rules read for the project's manifests. When no
// manifest declares what a rule reads the graph for (rules.ManifestNeedsModuleGraph), the rules read nothing and it
// is empty. Otherwise it holds the layout of every Gradle build with such a manifest, and the build-script
// candidates of every manifest, since the rule reads each manifest's script to find the apps. The upward build-root
// search is shared across manifests, and each build is discovered once per run, without source roots.
func projectBuildGraphs(project *android.Project) buildGraphInputs {
	identity := strings.Join(project.ManifestPaths, "\x00") + "\x01" + strings.Join(project.GradlePaths, "\x00")
	generation := hashutil.Default().Generation()
	buildGraphMemo.mu.Lock()
	defer buildGraphMemo.mu.Unlock()
	if buildGraphMemo.valid && buildGraphMemo.generation == generation && buildGraphMemo.identity == identity {
		return buildGraphMemo.inputs
	}
	var finder module.BuildRootFinder
	roots := map[string]bool{}
	needed := false
	memo := hashutil.Default()
	for _, manifest := range project.ManifestPaths {
		// The manifest content is in every key this feeds, so skipping a manifest no rule discovers a graph for is
		// safe: editing it in moves the key anyway. An unreadable manifest is searched. Reading it through the memo
		// leaves its hash there for the manifest fingerprints, so it is read once.
		var content []byte
		_, err := memo.HashFile(manifest, func() ([]byte, error) {
			b, err := os.ReadFile(manifest)
			content = b
			return b, err
		})
		if err == nil && content == nil {
			content, err = os.ReadFile(manifest)
		}
		if err == nil && !rules.ManifestNeedsModuleGraph(content) {
			continue
		}
		needed = true
		if root := finder.Find(filepath.Dir(manifest)); root != "" {
			roots[root] = true
		}
	}
	sorted := make([]string, 0, len(roots))
	for root := range roots {
		sorted = append(sorted, root)
	}
	sort.Strings(sorted)
	layouts := make([]buildGraphLayout, 0, len(sorted))
	for _, root := range sorted {
		g, err := module.DiscoverModuleLayout(context.Background(), root)
		if err != nil || g == nil || g.BuildSystem != "" {
			continue
		}
		layout := buildGraphLayout{root: root, files: g.InputFiles()}
		for path, m := range g.Modules {
			layout.modules = append(layout.modules, path+"\x00"+m.Dir)
		}
		sort.Strings(layout.modules)
		layouts = append(layouts, layout)
	}
	inputs := buildGraphInputs{layouts: layouts}
	if needed {
		inputs.scripts = manifestScriptsOutsideGraphs(project.ManifestPaths, layouts)
		inputs.catalogDirs, inputs.catalogs = manifestCatalogSearches(project.ManifestPaths)
	}
	buildGraphMemo.generation, buildGraphMemo.identity, buildGraphMemo.inputs, buildGraphMemo.valid =
		generation, identity, inputs, true
	return inputs
}

// manifestScriptsOutsideGraphs returns, sorted, the build-script candidates of the manifests that none of the
// layouts already lists.
func manifestScriptsOutsideGraphs(manifests []string, layouts []buildGraphLayout) []string {
	seen := map[string]bool{}
	for _, layout := range layouts {
		for _, file := range layout.files {
			seen[file] = true
		}
	}
	var scripts []string
	for _, manifest := range manifests {
		for _, script := range rules.ManifestBuildScriptCandidates(manifest) {
			if !seen[script] {
				seen[script] = true
				scripts = append(scripts, script)
			}
		}
	}
	sort.Strings(scripts)
	return scripts
}

// manifestCatalogSearches runs the version-catalog search of every manifest's module directory, the directory of its
// first build-script candidate that exists (lookupManifestGradleInfo's choice; which one that is, is keyed by the
// candidates' hashes). Each directory is globbed and each module directory searched once.
func manifestCatalogSearches(manifests []string) ([]catalogDir, []string) {
	globbed := map[string][]string{}
	glob := func(dir string) []string {
		matches, ok := globbed[dir]
		if !ok {
			matches = rules.VersionCatalogGlob(dir)
			globbed[dir] = matches
		}
		return matches
	}
	searched := map[string]bool{}
	catalogSet := map[string]bool{}
	for _, manifest := range manifests {
		moduleDir := ""
		for _, script := range rules.ManifestBuildScriptCandidates(manifest) {
			if _, err := os.Stat(script); err == nil {
				moduleDir = filepath.Dir(script)
				break
			}
		}
		if moduleDir == "" || searched[moduleDir] {
			continue
		}
		searched[moduleDir] = true
		_, matches := rules.VersionCatalogSearch(moduleDir, glob)
		for _, catalog := range matches {
			catalogSet[catalog] = true
		}
	}
	dirs := make([]catalogDir, 0, len(globbed))
	for dir, matches := range globbed {
		dirs = append(dirs, catalogDir{dir: dir, matches: matches})
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].dir < dirs[j].dir })
	catalogs := make([]string, 0, len(catalogSet))
	for catalog := range catalogSet {
		catalogs = append(catalogs, catalog)
	}
	sort.Strings(catalogs)
	return dirs, catalogs
}
