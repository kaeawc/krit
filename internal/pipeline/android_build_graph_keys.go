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
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/sysres"
)

// buildFileReaders says which build files beyond the scan the active rules read, and so which sections the cache keys
// over their findings need.
type buildFileReaders struct {
	// moduleGraph: a rule reads the Gradle module graph of a manifest's build (rules.ReadsModuleGraph).
	moduleGraph bool
	// gradleInfo: a manifest or manifest-source rule reads each manifest's build script and the version catalogs its
	// aliases resolve through (rules.ReadsManifestGradleInfo). A module-graph rule reads them too.
	gradleInfo bool
	// resources: the active resource rules that read build files for a res directory (rules.ResourceBuildInfoReader),
	// nil when none does (#1883).
	resources *resourceBuildInfoRules
}

func buildFileReadersOf(active []*api.Rule) buildFileReaders {
	graph := rules.ReadsModuleGraph(active)
	return buildFileReaders{
		moduleGraph: graph,
		gradleInfo:  graph || rules.ReadsManifestGradleInfo(active),
		resources:   resourceBuildInfoRulesOf(active),
	}
}

// buildFilesFingerprint hashes every Gradle file manifest and manifest-source rules can read: the build scripts and
// settings files under the scan paths (Project.GradlePaths), and the files outside them that reads says the rules
// read. ProviderReadPermissionOnly walks the module graph of each manifest's Gradle build, which reaches build files no
// scan path holds: a scan of only app/ and lib/, an explicit file list, a module that is gitignored or mapped into a
// dot-directory or build/ (#1800). Every manifest rule reads its manifest's build script (is the module an app or a
// library, its SDK levels) and the version catalogs that script's aliases resolve through, which a scan of only
// source directories doesn't hold and no scan holds the catalogs of (#1853): see manifestGradleInfoFingerprint. A
// resource rule that reads build files reads them for each res directory, which a scan of only res directories doesn't
// hold either (#1883): see gradleInfoFingerprint. It returns false when a scanned path can't be hashed.
func buildFilesFingerprint(project *android.Project, reads buildFileReaders) (string, bool) {
	h := hashutil.Hasher().New()
	if project == nil {
		return hex.EncodeToString(h.Sum(nil)), true
	}
	if !writePathHashes(h, "gradle", project.GradlePaths) {
		return "", false
	}
	if reads.moduleGraph {
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
			writeUnscannedFileHashes(h, memo, build.files, scanned)
		}
	}
	if reads.gradleInfo || reads.resources != nil {
		_, _ = h.Write([]byte("gradle-info\x00" + gradleInfoFingerprint(project, reads)))
	}
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

// runScoped memoizes one value for one run. An entry is valid for one generation of the shared hashutil Memo, which
// the CLI and the daemon clear at the start of every scan, and for one identity (the project's paths). The values
// only feed cache keys, so runs without caches never read a stale one.
type runScoped[T any] struct {
	mu         sync.Mutex
	generation uint64
	identity   string
	value      T
	valid      bool
}

func (m *runScoped[T]) get(identity string, compute func() T) T {
	generation := hashutil.Default().Generation()
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.valid || m.generation != generation || m.identity != identity {
		m.value = compute()
		m.generation, m.identity, m.valid = generation, identity, true
	}
	return m.value
}

// buildGraphMemo shares projectBuildGraphs between the findings-bundle and manifest-source keys of one run.
var buildGraphMemo runScoped[[]buildGraphLayout]

// projectBuildGraphs returns the layout of every Gradle build the module-graph rules read for the project's
// manifests, sorted by root. When no manifest declares what a rule reads the graph for (rules.ManifestNeedsModuleGraph),
// the rules read nothing and it is empty. The upward build-root search is shared across manifests, and each build is
// discovered once per run, without source roots.
func projectBuildGraphs(project *android.Project) []buildGraphLayout {
	return buildGraphMemo.get(strings.Join(project.ManifestPaths, "\x00"), func() []buildGraphLayout {
		return discoverBuildGraphs(project.ManifestPaths)
	})
}

func discoverBuildGraphs(manifests []string) []buildGraphLayout {
	var finder module.BuildRootFinder
	roots := map[string]bool{}
	memo := hashutil.Default()
	for _, manifest := range manifests {
		// The manifest content is in every key this feeds, so skipping a manifest no rule discovers a graph for is
		// safe: editing it in moves the key anyway. An unreadable manifest is searched. Reading it through the memo
		// leaves its hash there for the manifest fingerprints, so it is read once.
		content, err := readThroughMemo(memo, manifest)
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
	return layouts
}

// readThroughMemo reads a file, leaving its content hash in the memo so the fingerprints that hash it next don't read
// it again.
func readThroughMemo(memo *hashutil.Memo, path string) ([]byte, error) {
	var content []byte
	_, err := memo.HashFile(path, func() ([]byte, error) {
		b, err := os.ReadFile(path)
		content = b
		return b, err
	})
	if err == nil && content == nil {
		content, err = os.ReadFile(path)
	}
	return content, err
}

// manifestGradleInfoMemo shares manifestGradleInfoFingerprint between the keys of one run.
var manifestGradleInfoMemo runScoped[string]

// manifestGradleInfoFingerprint hashes what readManifestGradleInfo reads from disk for the project's manifests
// beyond the manifest itself (#1853):
//   - the build script each manifest's module is read from (rules.ManifestBuildScript, the first of its candidates
//     that exists, searched again every run, so creating a nearer one or deleting the one read moves the key) and that
//     script's content;
//   - for a script with plugin or libs. dependency aliases, the catalog sources librarymodel's discovery finds for it,
//     each with its kind and accessor name (#1872). Discovery runs again every run, so a catalog created in a
//     directory it globs, or a settings file that declares one, moves the key, and so do a settings edit that
//     changes which files its versionCatalogs blocks import or the names they import them under, such as a
//     create("libs") renamed, a renamed defaultLibrariesExtensionName, and a block that declares a catalog under the
//     name of an unimported gradle/<name>.versions.toml, which then no longer serves that name (Shadowed);
//   - the content, or absence, of every catalog source those discoveries found: TOML files, and the settings files
//     and settings plugins that declare catalogs programmatically.
//
// Scripts with no alias read no catalog, so their discoveries are skipped (rules.BuildScriptCatalogReads); the
// script's own content is keyed, so adding an alias moves the key. A settings edit that leaves every source list
// alone, such as an include(...), doesn't move it. Each directory is discovered, and each script searched, once per
// run, across the CPUs, and the fingerprint is computed once per run for every key that folds it in.
func manifestGradleInfoFingerprint(project *android.Project) string {
	if project == nil || len(project.ManifestPaths) == 0 {
		return ""
	}
	return manifestGradleInfoMemo.get(strings.Join(project.ManifestPaths, "\x00"), func() string {
		return computeManifestGradleInfoFingerprint(project.ManifestPaths)
	})
}

func computeManifestGradleInfoFingerprint(manifests []string) string {
	memo := hashutil.Default()
	workers := sysres.CPUs()
	scripts := make([]string, len(manifests))
	forEachParallel(len(manifests), workers, func(i int) { scripts[i] = rules.ManifestBuildScript(manifests[i]) })
	owners := make([]string, len(manifests))
	var unique []string
	seen := map[string]bool{}
	for i, script := range scripts {
		owners[i] = manifests[i] + "\x00" + script
		if script != "" && !seen[script] {
			seen[script] = true
			unique = append(unique, script)
		}
	}
	discovery := librarymodel.NewCatalogDiscovery()
	type scriptReads struct {
		library      string // the script with librarymodel's catalog sources for it
		libraryPaths []string
	}
	reads := make([]scriptReads, len(unique))
	forEachParallel(len(unique), workers, func(i int) {
		script := unique[i]
		content, err := readThroughMemo(memo, script)
		if err != nil {
			return
		}
		if pluginAliases, dependencyAliases := rules.BuildScriptCatalogReads(string(content)); !pluginAliases && !dependencyAliases {
			return
		}
		entry := script
		sources, _ := discovery.FindVersionCatalogSourcesForGradlePath(script)
		for _, source := range sources {
			entry += "\x00" + string(source.Kind) + "\x00" + source.Path + "\x00" + source.Accessor
			if source.Shadowed {
				entry += "\x00shadowed"
			}
			reads[i].libraryPaths = append(reads[i].libraryPaths, source.Path)
		}
		reads[i].library = entry
	})
	catalogSet := map[string]bool{}
	var librarySources []string
	for _, r := range reads {
		for _, catalog := range r.libraryPaths {
			catalogSet[catalog] = true
		}
		if r.library != "" {
			librarySources = append(librarySources, r.library)
		}
	}
	h := hashutil.Hasher().New()
	_, _ = h.Write([]byte("scripts\x00"))
	sort.Strings(owners)
	for _, owner := range owners {
		_, _ = h.Write([]byte(owner + "\x01"))
	}
	sort.Strings(unique)
	writeUnscannedFileHashes(h, memo, unique, nil)
	_, _ = h.Write([]byte("catalog-sources\x00"))
	sort.Strings(librarySources)
	for _, entry := range librarySources {
		_, _ = h.Write([]byte(entry + "\x01"))
	}
	_, _ = h.Write([]byte("catalogs\x00"))
	writeUnscannedFileHashes(h, memo, sortedKeys(catalogSet), nil)
	return hex.EncodeToString(h.Sum(nil))
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
