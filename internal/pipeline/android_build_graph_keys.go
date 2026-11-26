package pipeline

import (
	"context"
	"encoding/hex"
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
// that is gitignored or mapped into a dot-directory or build/. It returns false when a scanned path can't be hashed.
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
	for _, build := range projectBuildGraphs(project) {
		_, _ = h.Write([]byte("graph\x00" + build.root + "\x00"))
		for _, m := range build.modules {
			_, _ = h.Write([]byte(m + "\x00"))
		}
		for _, file := range build.files {
			if scanned[file] {
				continue
			}
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

// buildGraphLayout is the part of one Gradle build's module graph a cache key needs: its root, its modules as
// "path\x00dir", and the files the graph is read from (module.Graph.InputFiles).
type buildGraphLayout struct {
	root    string
	modules []string
	files   []string
}

// buildGraphMemo shares projectBuildGraphs between the findings-bundle and manifest-source keys of one run. An entry
// is valid for one generation of the shared hashutil Memo, which the CLI and the daemon clear at the start of every
// scan, and for one set of manifest and Gradle paths. It only feeds cache keys, so runs without caches never read a
// stale one.
var buildGraphMemo struct {
	mu         sync.Mutex
	generation uint64
	identity   string
	layouts    []buildGraphLayout
	valid      bool
}

// projectBuildGraphs returns, sorted by root, the module-graph layout of every Gradle build that holds one of the
// project's manifests a rule can read the graph for (rules.ManifestNeedsModuleGraph). The upward build-root search is
// shared across manifests, and each build is discovered once per run, without source roots.
func projectBuildGraphs(project *android.Project) []buildGraphLayout {
	identity := strings.Join(project.ManifestPaths, "\x00") + "\x01" + strings.Join(project.GradlePaths, "\x00")
	generation := hashutil.Default().Generation()
	buildGraphMemo.mu.Lock()
	defer buildGraphMemo.mu.Unlock()
	if buildGraphMemo.valid && buildGraphMemo.generation == generation && buildGraphMemo.identity == identity {
		return buildGraphMemo.layouts
	}
	var finder module.BuildRootFinder
	roots := map[string]bool{}
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
	buildGraphMemo.generation, buildGraphMemo.identity, buildGraphMemo.layouts, buildGraphMemo.valid =
		generation, identity, layouts, true
	return layouts
}
