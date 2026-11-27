package librarymodel

import (
	"cmp"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/lazyre"
)

var (
	// defaultLibrariesExtensionName = "x", defaultLibrariesExtensionName.set("x"), in either DSL.
	defaultLibrariesExtensionNameRe = lazyre.New(`\bdefaultLibrariesExtensionName\s*(?:=|\.\s*set\s*\()\s*["']([A-Za-z_][A-Za-z0-9_]*)["']`)
	// versionCatalogs { ... }, or a chained versionCatalogs.create("name") { ... }.
	versionCatalogsBlockRe = lazyre.New(`\bversionCatalogs\s*(?:\{|\.\s*(?:create|register|maybeCreate)\s*\(\s*["']([A-Za-z_][A-Za-z0-9_]*)["']\s*\)\s*(?:\.\s*apply\s*)?\{)`)
	// The head of one catalog inside versionCatalogs { }: create("name") {, register("name") {, or Groovy's name {.
	catalogBlockHeadRe = lazyre.New(`^(?:(?:create|register|maybeCreate)\s*\(\s*["']([A-Za-z_][A-Za-z0-9_]*)["']\s*\)\s*(?:\.\s*apply\s*)?|([A-Za-z_][A-Za-z0-9_]*)\s*)\{`)
)

// notCatalogNames are Groovy block heads inside versionCatalogs { } that configure something other than a catalog
// of that name.
var notCatalogNames = map[string]bool{
	"all": true, "configureEach": true, "else": true, "try": true, "finally": true, "do": true,
	"apply": true, "run": true, "with": true, "also": true, "let": true,
}

// settingsCatalogBlock is the body of one catalog a settings file declares, as a span of its comment-stripped
// content.
type settingsCatalogBlock struct {
	name       string
	start, end int
}

// settingsCatalogBlocks finds the catalogs a comment-stripped settings file (or settings plugin) declares:
// each create("name") { }, register("name") { } or Groovy name { } directly inside a versionCatalogs { } block, and
// each versionCatalogs.create("name") { } chain.
func settingsCatalogBlocks(content string) []settingsCatalogBlock {
	var blocks []settingsCatalogBlock
	for _, m := range versionCatalogsBlockRe.FindAllStringSubmatchIndex(content, -1) {
		open := m[1] - 1
		end := matchingBrace(content, open)
		if end < 0 {
			continue
		}
		if m[2] >= 0 {
			blocks = append(blocks, settingsCatalogBlock{name: content[m[2]:m[3]], start: open + 1, end: end})
			continue
		}
		blocks = append(blocks, catalogBlocksIn(content, open+1, end)...)
	}
	return blocks
}

// catalogBlocksIn returns the catalog blocks at the top level of content[start:end], the body of versionCatalogs { }.
func catalogBlocksIn(content string, start, end int) []settingsCatalogBlock {
	var blocks []settingsCatalogBlock
	depth := 0
	for i := start; i < end; i++ {
		switch ch := content[i]; {
		case ch == '"' || ch == '\'':
			i = skipQuoted(content, i)
		case ch == '{':
			depth++
		case ch == '}':
			depth--
		case depth == 0 && isIdentStart(ch) && (i == start || !isIdentPart(content[i-1])):
			m := catalogBlockHeadRe.FindStringSubmatchIndex(content[i:end])
			if m == nil {
				for i+1 < end && isIdentPart(content[i+1]) {
					i++
				}
				continue
			}
			name := ""
			if m[2] >= 0 {
				name = content[i+m[2] : i+m[3]]
			} else {
				name = content[i+m[4] : i+m[5]]
			}
			open := i + m[1] - 1
			blockEnd := matchingBrace(content, open)
			if blockEnd < 0 || blockEnd > end {
				return blocks
			}
			if !notCatalogNames[name] {
				blocks = append(blocks, settingsCatalogBlock{name: name, start: open + 1, end: blockEnd})
			}
			i = blockEnd
		}
	}
	return blocks
}

// catalogBlockAt returns the name of the catalog block holding offset, or "" when none does.
func catalogBlockAt(blocks []settingsCatalogBlock, offset int) string {
	for _, block := range blocks {
		if offset >= block.start && offset < block.end {
			return block.name
		}
	}
	return ""
}

// matchingBrace returns the index of the brace closing the one at open, skipping string literals, or -1.
func matchingBrace(content string, open int) int {
	depth := 0
	for i := open; i < len(content); i++ {
		switch content[i] {
		case '"', '\'':
			i = skipQuoted(content, i)
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// skipQuoted returns the index of the quote closing the string literal that opens at start, or the last index.
func skipQuoted(content string, start int) int {
	quote := content[start]
	for i := start + 1; i < len(content); i++ {
		switch content[i] {
		case '\\':
			i++
		case quote:
			return i
		}
	}
	return len(content) - 1
}

func isIdentStart(ch byte) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9')
}

// CatalogPluginsByAccessor is the [plugins] table of every catalog a build script can reach by accessor name, as
// `alias(<accessor>.plugins.<key>)` (#1872).
type CatalogPluginsByAccessor struct {
	// named: the catalogs Gradle imports, by the name it imports them under.
	named map[string]map[string]CatalogPlugin
	// unimported: the gradle/<stem>.versions.toml files no settings file imports, by stem.
	unimported map[string]map[string]CatalogPlugin
}

// Plugins returns the plugins of the catalog build scripts reach as accessor, keyed by catalog alias, such as
// "android-application". When no catalog Gradle imports has that name, it falls back to an unimported
// gradle/<accessor>.versions.toml: the settings file that imports it may be out of krit's sight (a settings plugin,
// a computed path), and the file name is the name every documented import gives it. Nil when neither exists.
func (p CatalogPluginsByAccessor) Plugins(accessor string) map[string]CatalogPlugin {
	if plugins, ok := p.named[accessor]; ok {
		return plugins
	}
	return p.unimported[accessor]
}

// LoadCatalogPluginsByAccessor reads the plugin tables of the sources, as FindVersionCatalogSourcesForGradlePath
// returns them, outermost first: a nearer catalog's alias overrides an outer one of the same catalog name. A
// programmatic source contributes each block's plugin(...) declarations to the catalog the block names. Catalogs
// under one name merge, as Gradle merges gradle/libs.versions.toml into a `libs` catalog the settings file
// declares programmatically.
func LoadCatalogPluginsByAccessor(sources []CatalogSource) CatalogPluginsByAccessor {
	p := CatalogPluginsByAccessor{named: map[string]map[string]CatalogPlugin{}, unimported: map[string]map[string]CatalogPlugin{}}
	for _, source := range sources {
		parsed := catalogSourcePlugins(source)
		switch {
		case source.Kind == CatalogSourceSettingsProgrammatic:
			for name, plugins := range parsed {
				mergeCatalogPlugins(p.named, name, plugins)
			}
		case source.Accessor != "":
			mergeCatalogPlugins(p.named, source.Accessor, parsed[""])
		case source.Kind == CatalogSourceMergedTOML || source.Kind == CatalogSourceStandardTOML:
			mergeCatalogPlugins(p.unimported, strings.TrimSuffix(filepath.Base(source.Path), ".versions.toml"), parsed[""])
		}
	}
	return p
}

func mergeCatalogPlugins(into map[string]map[string]CatalogPlugin, name string, plugins map[string]CatalogPlugin) {
	table := into[name]
	if table == nil {
		table = make(map[string]CatalogPlugin, len(plugins))
		into[name] = table
	}
	for alias, plugin := range plugins {
		table[alias] = plugin
	}
}

type sourcePluginsEntry struct {
	stamp   catalogSourceStamp
	plugins map[string]map[string]CatalogPlugin
}

var (
	sourcePluginsMu sync.Mutex
	sourcePlugins   = make(map[string]sourcePluginsEntry)
)

// catalogSourcePlugins returns the plugin tables of one source: under "" for a TOML file, and by block name for a
// programmatic source. It reuses the previous parse while the file keeps its mtime and size. Callers must treat the
// result as read-only.
func catalogSourcePlugins(source CatalogSource) map[string]map[string]CatalogPlugin {
	key := string(source.Kind) + "=" + source.Path
	stamp := catalogStamps([]CatalogSource{source})[0]
	sourcePluginsMu.Lock()
	cached, hit := sourcePlugins[key]
	sourcePluginsMu.Unlock()
	if hit && cached.stamp == stamp {
		return cached.plugins
	}
	plugins := parseCatalogSourcePlugins(source)
	sourcePluginsMu.Lock()
	sourcePlugins[key] = sourcePluginsEntry{stamp: stamp, plugins: plugins}
	sourcePluginsMu.Unlock()
	return plugins
}

func parseCatalogSourcePlugins(source CatalogSource) map[string]map[string]CatalogPlugin {
	content, err := os.ReadFile(source.Path)
	if err != nil {
		return nil
	}
	if source.Kind != CatalogSourceSettingsProgrammatic {
		return map[string]map[string]CatalogPlugin{"": ParseVersionCatalogContent(string(content)).Plugins}
	}
	stripped := stripGradleComments(string(content))
	tables := map[string]map[string]CatalogPlugin{}
	for _, block := range settingsCatalogBlocks(stripped) {
		mergeCatalogPlugins(tables, block.name, programmaticCatalogPlugins(stripped[block.start:block.end]))
	}
	return tables
}

// programmaticCatalogPlugins reads the plugin("alias", "id") declarations of a comment-stripped catalog block.
func programmaticCatalogPlugins(body string) map[string]CatalogPlugin {
	plugins := map[string]CatalogPlugin{}
	for _, match := range settingsPluginRe.FindAllStringSubmatch(body, -1) {
		plugins[match[1]] = CatalogPlugin{ID: match[2], Version: cmp.Or(match[3], match[4])}
	}
	return plugins
}
