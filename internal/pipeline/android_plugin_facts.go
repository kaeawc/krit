package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/sysres"
)

// modulePluginsFingerprint hashes the plugins every build script of the project applies, as rules.ModulePluginIDs reads
// them: by id and through version-catalog aliases, without comments. Per-file rules such as AnnotateVersionCheck decide
// from the module's plugins whether it is a library, which the file's own content hash does not cover, so switching a
// module between application and library must change their cache keys. Other build script edits (dependencies,
// versions) leave it unchanged, and so does a catalog edit that leaves every plugin id alone. Scripts are spelled
// relative to the directory all of them share, so the same checkout at another absolute path keys its caches the same
// (#1662).
func modulePluginsFingerprint(project *android.Project) string {
	if project == nil || len(project.GradlePaths) == 0 {
		return ""
	}
	paths := append([]string(nil), project.GradlePaths...)
	sort.Strings(paths)
	root := fsutil.CommonDir(paths)
	discovery := librarymodel.NewCatalogDiscovery()
	// Modules of one build share their catalogs, so each source list is parsed once. The scripts resolve across the
	// CPUs, so the memo and the tables it returns, which are not safe for concurrent use, share one lock.
	var mu sync.Mutex
	catalogs := map[string]rules.CatalogPluginTables{}
	load := func(sources []librarymodel.CatalogSource) rules.CatalogPluginTables {
		var key strings.Builder
		for _, source := range sources {
			fmt.Fprintf(&key, "%s\x00%s\x00%s\x00%t\x01", source.Kind, source.Path, source.Accessor, source.Shadowed)
		}
		mu.Lock()
		defer mu.Unlock()
		tables, ok := catalogs[key.String()]
		if !ok {
			tables = rules.LoadCatalogPluginTables(sources)
			catalogs[key.String()] = tables
		}
		return func(accessor string) map[string]string {
			mu.Lock()
			defer mu.Unlock()
			return tables(accessor)
		}
	}
	lines := make([]string, len(paths))
	forEachParallel(len(paths), sysres.CPUs(), func(i int) {
		data, err := os.ReadFile(paths[i])
		if err != nil {
			return
		}
		plugins := rules.ModulePluginIDs(discovery, load, filepath.Dir(paths[i]), string(data))
		if len(plugins) == 0 {
			return
		}
		sort.Strings(plugins)
		lines[i] = fsutil.RelSlash(root, paths[i]) + "|" + strings.Join(plugins, ",") + "\n"
	})
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(line)
	}
	if b.Len() == 0 {
		return ""
	}
	return hashutil.HashHex([]byte(b.String()))[:16]
}

// withScanModuleScripts returns project with the build script of each scan path's own module added to its GradlePaths,
// for the per-file cache key. A scan below a module root, such as lib/src/main/java, holds no build script, yet
// per-file rules read the module's plugins and SDK levels from it, so the key must cover it. A scan at or above every
// module root lists those scripts already and gets project back unchanged, so its key stays as it was.
func withScanModuleScripts(project *android.Project, scanPaths []string) *android.Project {
	seen := map[string]bool{}
	if project != nil {
		for _, path := range project.GradlePaths {
			seen[path] = true
		}
	}
	var extra []string
	for _, path := range scanPaths {
		if script := scanPathModuleScript(path); script != "" && !seen[script] {
			seen[script] = true
			extra = append(extra, script)
		}
	}
	if len(extra) == 0 {
		return project
	}
	with := &android.Project{}
	if project != nil {
		*with = *project
	}
	with.GradlePaths = append(append([]string(nil), with.GradlePaths...), extra...)
	return with
}

// scanPathModuleScript returns the build script of the module a scan path lies in: the first build.gradle.kts or
// build.gradle at or above it, as the rules find a file's module (workModuleRoot). "" when there is none.
func scanPathModuleScript(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	dir := abs
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		dir = filepath.Dir(abs)
	}
	for depth := 0; depth < 24; depth++ {
		for _, name := range []string{"build.gradle.kts", "build.gradle"} {
			script := filepath.Join(dir, name)
			if info, err := os.Stat(script); err == nil && !info.IsDir() {
				return script
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
