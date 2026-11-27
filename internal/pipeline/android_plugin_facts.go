package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

// modulePluginPatterns match the plugin declarations of a build script: Android plugin ids, `id("...")` and
// `alias(libs.plugins....)`. Each starts with a literal, which regexp finds without trying every position; an `id`
// counts only at a word boundary (modulePlugins).
var modulePluginPatterns = []*regexp.Regexp{
	regexp.MustCompile(`com\.android\.[a-z-]+`),
	regexp.MustCompile(`id\s*\(?\s*["'][^"']+["']`),
	regexp.MustCompile(`alias\s*\(\s*[\w.]+\s*\)`),
}

// modulePlugins returns the plugin declarations modulePluginPatterns find in a build script.
func modulePlugins(content string) []string {
	var plugins []string
	for i, pattern := range modulePluginPatterns {
		for _, loc := range pattern.FindAllStringIndex(content, -1) {
			if i == 1 && loc[0] > 0 && isGradleWordByte(content[loc[0]-1]) {
				continue
			}
			plugins = append(plugins, content[loc[0]:loc[1]])
		}
	}
	return plugins
}

func isGradleWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// modulePluginsFingerprint hashes the plugins every build script of the project applies. Per-file rules such as
// AnnotateVersionCheck decide from the module's plugin whether it is a library, which the file's own content hash
// does not cover, so switching a module between application and library must change their cache keys. Other build
// script edits (dependencies, versions) leave it unchanged. A plugin applied through a version-catalog alias counts as
// the id the catalogs librarymodel's discovery finds resolve it to (rules.BuildScriptPluginIDs), so a catalog edit
// that changes the id moves the key and a version bump does not. Scripts are spelled relative to the directory all of
// them share, so the same checkout at another absolute path keys its caches the same (#1662).
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
		content := string(data)
		plugins := modulePlugins(content)
		if len(plugins) == 0 {
			return
		}
		for _, id := range rules.BuildScriptPluginIDs(discovery, load, filepath.Dir(paths[i]), content) {
			plugins = append(plugins, "="+id)
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
