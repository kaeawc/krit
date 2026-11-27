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
	// versionCatalogs { ... }, or a chained versionCatalogs.create("name") { ... } or versionCatalogs.create("name")
	// .from(...). Group 1 is the name, group 2 the from of the brace-less form.
	versionCatalogsBlockRe = lazyre.New(`\bversionCatalogs\s*(?:\{|\.\s*` + namedCatalogHead + `)`)
	// The head of one catalog inside versionCatalogs { }: create("name") {, register("name") {, maybeCreate("name")
	// .apply {, create("name").from(...), or Groovy's name {. Groups 1 and 2 are namedCatalogHead's, 3 the bare name.
	catalogBlockHeadRe = lazyre.New(`^(?:` + namedCatalogHead + `|([A-Za-z_][A-Za-z0-9_]*)\s*\{)`)
)

// namedCatalogHead matches create("name"), register("name") or maybeCreate("name"), MutableVersionCatalogContainer's
// NamedDomainObjectContainer methods, followed by a configuring block, optionally through Kotlin's .apply, or by a
// chained .from(...), as in create("tools").from(files("gradle/shared.versions.toml")) (#1910 review round 3). Group 1
// is the name; group 2 is the from of the chained form.
const namedCatalogHead = `(?:create|register|maybeCreate)\s*\(\s*["']([A-Za-z_][A-Za-z0-9_]*)["']\s*\)\s*(?:(?:\.\s*apply\s*)?\{|\.\s*(from)\b)`

// notCatalogNames are Groovy block heads inside versionCatalogs { } that configure something other than a catalog
// of that name. They don't apply to an explicit create("all") and the like, which names a catalog (#1910 review round
// 3).
var notCatalogNames = map[string]bool{
	"all": true, "configureEach": true, "else": true, "try": true, "finally": true, "do": true,
	"apply": true, "run": true, "with": true, "also": true, "let": true,
}

// settingsCatalogBlock is the body of one catalog a settings file declares, as a span of its comment-stripped
// content: the inside of its braces, or the from(...) call that configures it.
type settingsCatalogBlock struct {
	name       string
	start, end int
}

// settingsCatalogBlocks finds the catalogs a comment-stripped settings file (or settings plugin) declares:
// each create("name") { }, register("name") { }, create("name").from(...) or Groovy name { } directly inside a
// versionCatalogs { } block, and each versionCatalogs.create("name") { } or .from(...) chain. code marks the bytes of
// content that are code (gradleCode): a match must start in code, and only braces in code nest, so text in a string
// literal declares nothing and a string or template holding a brace leaves the blocks balanced.
func settingsCatalogBlocks(content string, code []bool) []settingsCatalogBlock {
	var blocks []settingsCatalogBlock
	for _, m := range versionCatalogsBlockRe.FindAllStringSubmatchIndex(content, -1) {
		if !gradleCodeAt(code, m[0]) {
			continue
		}
		if m[4] >= 0 {
			if end := fromCallEnd(content, code, m[4]); end >= 0 {
				blocks = append(blocks, settingsCatalogBlock{name: content[m[2]:m[3]], start: m[4], end: end})
			}
			continue
		}
		open := m[1] - 1
		end := matchingBrace(content, code, open)
		if end < 0 {
			continue
		}
		if m[2] >= 0 {
			blocks = append(blocks, settingsCatalogBlock{name: content[m[2]:m[3]], start: open + 1, end: end})
			continue
		}
		blocks = append(blocks, catalogBlocksIn(content, code, open+1, end)...)
	}
	return blocks
}

// settingsDeclaredCatalogNames returns the names of every catalog the settings files declare (settingsCatalogBlocks).
func settingsDeclaredCatalogNames(settings []settingsFile) map[string]bool {
	names := map[string]bool{}
	for _, file := range settings {
		for _, block := range settingsCatalogBlocks(file.content, file.code) {
			names[block.name] = true
		}
	}
	return names
}

// catalogBlocksIn returns the catalog blocks at the top level of content[start:end], the body of versionCatalogs { }.
func catalogBlocksIn(content string, code []bool, start, end int) []settingsCatalogBlock {
	var blocks []settingsCatalogBlock
	depth := 0
	for i := start; i < end; i++ {
		if !gradleCodeAt(code, i) {
			continue
		}
		switch ch := content[i]; {
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
			// An explicit create("name") and the like always names a catalog; a bare Groovy name { } head may
			// configure something else.
			name, isCatalog := "", true
			if m[2] >= 0 {
				name = content[i+m[2] : i+m[3]]
			} else {
				name = content[i+m[6] : i+m[7]]
				isCatalog = !notCatalogNames[name]
			}
			// next is the index after the declaration: after its closing brace, or its from(...) call.
			block, next := settingsCatalogBlock{name: name}, 0
			if m[4] >= 0 {
				// create("name").from(...): the call is the catalog's configuration.
				block.start, block.end = i+m[4], fromCallEnd(content, code, i+m[4])
				next = block.end
			} else {
				open := i + m[1] - 1
				block.start, block.end = open+1, matchingBrace(content, code, open)
				next = block.end + 1
			}
			if block.end < 0 || block.end > end {
				return blocks
			}
			if isCatalog {
				blocks = append(blocks, block)
			}
			i = next - 1
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

// matchingBrace returns the index of the brace closing the one at open, counting only braces in code, or -1.
func matchingBrace(content string, code []bool, open int) int {
	return matchingClose(content, code, open, '{', '}')
}

// matchingClose returns the index of the closer closing the opener at open, counting only those in code, or -1.
func matchingClose(content string, code []bool, open int, opener, closer byte) int {
	depth := 0
	for i := open; i < len(content); i++ {
		if !gradleCodeAt(code, i) {
			continue
		}
		switch content[i] {
		case opener:
			depth++
		case closer:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// fromCallEnd returns the end of the from call whose name starts at from: after the parenthesis closing from(...),
// or, for Groovy's paren-less from files('...'), at the end of the line. -1 when the parenthesis is never closed.
func fromCallEnd(content string, code []bool, from int) int {
	i := from + len("from")
	for i < len(content) && (content[i] == ' ' || content[i] == '\t') {
		i++
	}
	if i < len(content) && content[i] == '(' {
		closing := matchingClose(content, code, i, '(', ')')
		if closing < 0 {
			return -1
		}
		return closing + 1
	}
	if newline := strings.IndexByte(content[i:], '\n'); newline >= 0 {
		return i + newline
	}
	return len(content)
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
		case source.Shadowed:
			// The settings declare a catalog of this file's name from elsewhere, such as a published one.
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
// programmatic source. It reuses the previous parse while the file keeps its content (catalogStamps). Callers must
// treat the result as read-only.
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
	stripped, code := stripGradleComments(string(content), isGroovyScript(source.Path))
	tables := map[string]map[string]CatalogPlugin{}
	for _, block := range settingsCatalogBlocks(stripped, code) {
		mergeCatalogPlugins(tables, block.name, programmaticCatalogPlugins(stripped, code, block))
	}
	return tables
}

// programmaticCatalogPlugins reads the plugin("alias", "id") declarations in code of a catalog block of a
// comment-stripped settings file.
func programmaticCatalogPlugins(content string, code []bool, block settingsCatalogBlock) map[string]CatalogPlugin {
	plugins := map[string]CatalogPlugin{}
	body := content[block.start:block.end]
	for _, m := range settingsPluginRe.FindAllStringSubmatchIndex(body, -1) {
		if !gradleCodeAt(code, block.start+m[0]) {
			continue
		}
		group := submatches(body, m)
		plugins[group[1]] = CatalogPlugin{ID: group[2], Version: cmp.Or(group[3], group[4])}
	}
	return plugins
}
