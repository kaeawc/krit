package librarymodel

import (
	"cmp"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/lazyre"
	"github.com/kaeawc/krit/internal/versioncatalog"
)

// VersionCatalog is a best-effort Gradle version-catalog model. It is not a
// general TOML or Gradle evaluator; it intentionally supports the catalog forms
// needed by the library model and records source/completeness metadata so
// callers can stay conservative when discovery is partial.
type VersionCatalog struct {
	Versions  map[string]string
	Plugins   map[string]CatalogPlugin
	Libraries map[string]CatalogLibrary
	Bundles   map[string][]string

	Sources      []CatalogSource
	Completeness CatalogCompleteness
}

type CatalogPlugin struct {
	ID      string
	Version string
}

type CatalogSourceKind string

const (
	CatalogSourceStandardTOML         CatalogSourceKind = "standard_toml"
	CatalogSourceMergedTOML           CatalogSourceKind = "merged_toml"
	CatalogSourceSettingsTOML         CatalogSourceKind = "settings_toml"
	CatalogSourceSettingsProgrammatic CatalogSourceKind = "settings_programmatic"
)

type CatalogSource struct {
	Path string
	Kind CatalogSourceKind
	// Accessor is the name build scripts reach the catalog by, as `libs` in `alias(libs.plugins.x)` (#1872). Gradle
	// imports gradle/libs.versions.toml under the settings' defaultLibrariesExtensionName, `libs` unless renamed, and
	// a file a settings `versionCatalogs { create("<name>") { from(files(...)) } }` block lists under that name. It is
	// "" for any other gradle/*.versions.toml, which Gradle doesn't import, for a from(files(...)) outside a named
	// block, and for a programmatic source, whose blocks each name their own catalog (LoadCatalogPluginsByAccessor).
	// Discovery, merging and the catalog contents don't depend on it.
	Accessor string
	// Shadowed is set on a gradle/<name>.versions.toml other than libs when a settings file beside it declares a
	// catalog called <name>, as `create("<name>") { from("com.example:catalog:1.0") }` does for a published one:
	// build scripts reach that catalog, not this file, as <name>, so CatalogPluginsByAccessor serves no alias of
	// <name> from this file (#1872 review). Discovery, merging and the catalog contents don't depend on it either.
	Shadowed bool
}

type CatalogCompleteness int

const (
	CatalogCompletenessNone CatalogCompleteness = iota
	CatalogCompletenessStandardTOML
	CatalogCompletenessMergedTOML
	CatalogCompletenessSettingsTOML
	CatalogCompletenessSettingsProgrammatic
)

type CatalogLibrary struct {
	Group   string
	Name    string
	Version string
}

var (
	catalogPluginAliasRe      = lazyre.New(`alias\s*\(\s*libs\.plugins\.([A-Za-z0-9_.]+)\s*\)`)
	catalogVersionAccessorRe  = lazyre.New(`libs\.versions\.([A-Za-z0-9_.]+)\.get\s*\(`)
	catalogDependencyAliasRe  = lazyre.New(`["']?([A-Za-z_][A-Za-z0-9_]*)["']?\s*\(\s*(?:platform\s*\(\s*)?libs\.([A-Za-z0-9_.]+)`)
	unresolvedStructuralRefRe = lazyre.New(`(?m)(?:["']?[A-Za-z_][A-Za-z0-9_]*["']?\s*\(\s*(?:platform\s*\(\s*)?(?:versions\.|deps\.|project\s*\(|projects\.)|apply\s*(?:\(|from\s*=|plugin\s*:)|id\s*\(\s*["'][^"']*(?:convention|build-logic)[^"']*["']|plugins?\s*\{[^}]*\b(?:convention|build-logic)\b)`)
	settingsCatalogFileRe     = lazyre.New(`\bfrom\s*(?:\(\s*files\s*\(\s*["']([^"']+)["']\s*\)\s*\)|files\s*\(\s*["']([^"']+)["']\s*\))`)
	settingsLibraryRe         = lazyre.New(`library\s*\(\s*["']([^"']+)["']\s*,\s*["']([^"']+)["']\s*,\s*["']([^"']+)["']\s*\)\s*(?:\.\s*(?:version\s*\(\s*(?:"([^"]+)"|'([^']+)'|[A-Za-z0-9_.]+)\s*\)|withoutVersion\s*\(\s*\)))?`)
	settingsPluginRe          = lazyre.New(`plugin\s*\(\s*["']([^"']+)["']\s*,\s*["']([^"']+)["']\s*\)\s*(?:\.\s*version\s*\(\s*(?:"([^"]+)"|'([^']+)'|[A-Za-z0-9_.]+)\s*\))?`)
	settingsPluginIDRe        = lazyre.New(`id\s*\(\s*["']([A-Za-z0-9_.-]+)["']\s*\)`)
	settingsIncludeBuildRe    = lazyre.New(`includeBuild\s*\(\s*["']([^"']+)["']\s*\)`)
	gradlePluginIDFieldRe     = lazyre.New(`\bid(?:\.set)?\s*(?:=|\()\s*["']([^"']+)["']`)
	gradlePluginImplClassRe   = lazyre.New(`\bimplementationClass(?:\.set)?\s*(?:=|\()\s*["']([^"']+)["']`)
)

func ParseVersionCatalogContent(content string) VersionCatalog {
	catalog := VersionCatalog{
		Versions:  make(map[string]string),
		Plugins:   make(map[string]CatalogPlugin),
		Libraries: make(map[string]CatalogLibrary),
		Bundles:   make(map[string][]string),
	}
	parsed, err := versioncatalog.Parse([]byte(content))
	if err != nil {
		return catalog
	}
	for _, e := range parsed.Versions {
		if value := e.Version.Effective(); value != "" {
			catalog.Versions[e.Alias] = value
		}
	}
	for _, lib := range parsed.Libraries {
		catalog.Libraries[lib.Alias] = CatalogLibrary{Group: lib.Group, Name: lib.Name, Version: lib.Resolved}
	}
	for _, p := range parsed.Plugins {
		catalog.Plugins[p.Alias] = CatalogPlugin{ID: p.ID, Version: p.Resolved}
	}
	for _, b := range parsed.Bundles {
		if len(b.Libraries) > 0 {
			catalog.Bundles[b.Alias] = b.Libraries
		}
	}
	return catalog
}

func ParseSettingsVersionCatalogContent(content string) VersionCatalog {
	catalog := VersionCatalog{
		Versions:  make(map[string]string),
		Plugins:   make(map[string]CatalogPlugin),
		Libraries: make(map[string]CatalogLibrary),
		Bundles:   make(map[string][]string),
	}
	content = stripGradleComments(content)
	for _, match := range settingsLibraryRe.FindAllStringSubmatch(content, -1) {
		version := cmp.Or(match[4], match[5])
		catalog.Libraries[match[1]] = CatalogLibrary{
			Group:   match[2],
			Name:    match[3],
			Version: version,
		}
	}
	for _, match := range settingsPluginRe.FindAllStringSubmatch(content, -1) {
		catalog.Plugins[match[1]] = CatalogPlugin{
			ID:      match[2],
			Version: cmp.Or(match[3], match[4]),
		}
	}
	return catalog
}

func LoadVersionCatalogForGradlePath(gradlePath string) (VersionCatalog, bool) {
	sources, ok := FindVersionCatalogSourcesForGradlePath(gradlePath)
	if !ok {
		return VersionCatalog{}, false
	}
	return loadVersionCatalogFromSources(sources)
}

// loadedCatalog is a parsed catalog plus the stat signature of every source it was built from.
type loadedCatalog struct {
	stamps  []catalogSourceStamp
	catalog VersionCatalog
	ok      bool
}

type catalogSourceStamp struct {
	modTime int64
	size    int64
	exists  bool
}

var (
	loadedCatalogsMu sync.Mutex
	loadedCatalogs   = make(map[string]loadedCatalog)
)

func catalogStamps(sources []CatalogSource) []catalogSourceStamp {
	stamps := make([]catalogSourceStamp, len(sources))
	for i, source := range sources {
		if info, err := os.Stat(source.Path); err == nil {
			stamps[i] = catalogSourceStamp{modTime: info.ModTime().UnixNano(), size: info.Size(), exists: true}
		}
	}
	return stamps
}

func sameCatalogStamps(a, b []catalogSourceStamp) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// loadVersionCatalogFromSources parses and merges the sources, reusing the previous result while every source
// keeps the same mtime and size. Callers must treat the returned maps as read-only.
func loadVersionCatalogFromSources(sources []CatalogSource) (VersionCatalog, bool) {
	key := catalogSourceCacheKey(sources)
	stamps := catalogStamps(sources)
	loadedCatalogsMu.Lock()
	cached, hit := loadedCatalogs[key]
	loadedCatalogsMu.Unlock()
	if hit && sameCatalogStamps(cached.stamps, stamps) {
		return cached.catalog, cached.ok
	}
	catalog, ok := parseVersionCatalogSources(sources)
	loadedCatalogsMu.Lock()
	loadedCatalogs[key] = loadedCatalog{stamps: stamps, catalog: catalog, ok: ok}
	loadedCatalogsMu.Unlock()
	return catalog, ok
}

func parseVersionCatalogSources(sources []CatalogSource) (VersionCatalog, bool) {
	var merged VersionCatalog
	for _, source := range sources {
		content, err := os.ReadFile(source.Path)
		if err != nil {
			continue
		}
		var catalog VersionCatalog
		switch source.Kind {
		case CatalogSourceSettingsProgrammatic:
			catalog = ParseSettingsVersionCatalogContent(string(content))
		default:
			catalog = ParseVersionCatalogContent(string(content))
		}
		catalog.Sources = []CatalogSource{source}
		catalog.Completeness = completenessForCatalogSource(source.Kind)
		merged.Merge(catalog)
	}
	return merged, !merged.Empty()
}

// CatalogDiscovery memoizes version-catalog source discovery per directory for
// one analysis run. Discovery reads each ancestor's settings files and, for a
// settings plugin that an included build provides, walks every build file under
// that build; a nested build with includeBuild("..") walks the whole repository.
// Without the memo that walk repeats for every Gradle file and every catalog
// lookup. Catalog contents are still revalidated by stat stamps on load. Safe for
// concurrent use; a nil *CatalogDiscovery does not memoize.
type CatalogDiscovery struct {
	dirSources sync.Map // dir -> []CatalogSource
	buildFiles sync.Map // included build root -> []string
}

// NewCatalogDiscovery returns an empty discovery memo. Use one per analysis run:
// it does not notice settings files or included builds that change afterwards.
func NewCatalogDiscovery() *CatalogDiscovery { return &CatalogDiscovery{} }

func (c *CatalogDiscovery) sourcesForDir(dir string) []CatalogSource {
	if c == nil {
		return c.catalogSourcesForDir(dir)
	}
	if cached, ok := c.dirSources.Load(dir); ok {
		return cached.([]CatalogSource)
	}
	sources, _ := c.dirSources.LoadOrStore(dir, c.catalogSourcesForDir(dir))
	return sources.([]CatalogSource)
}

// LoadVersionCatalogForGradlePath is the package-level function of the same name,
// with directory discovery memoized.
func (c *CatalogDiscovery) LoadVersionCatalogForGradlePath(gradlePath string) (VersionCatalog, bool) {
	sources, ok := c.FindVersionCatalogSourcesForGradlePath(gradlePath)
	if !ok {
		return VersionCatalog{}, false
	}
	return loadVersionCatalogFromSources(sources)
}

// FindVersionCatalogSourcesForGradlePath is the package-level function of the
// same name, with directory discovery memoized.
func (c *CatalogDiscovery) FindVersionCatalogSourcesForGradlePath(gradlePath string) ([]CatalogSource, bool) {
	if absPath, err := filepath.Abs(gradlePath); err == nil {
		gradlePath = absPath
	}
	var groups [][]CatalogSource
	for dir := filepath.Dir(gradlePath); dir != "." && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
		if sources := c.sourcesForDir(dir); len(sources) > 0 {
			groups = append(groups, sources)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	if len(groups) == 0 {
		return nil, false
	}
	var ordered []CatalogSource
	for i := len(groups) - 1; i >= 0; i-- {
		ordered = append(ordered, groups[i]...)
	}
	normalizeCatalogSourceKinds(ordered)
	return ordered, true
}

func FindVersionCatalogSourcesForGradlePath(gradlePath string) ([]CatalogSource, bool) {
	return (*CatalogDiscovery)(nil).FindVersionCatalogSourcesForGradlePath(gradlePath)
}

// settingsFile is a settings file of a directory, with its comments stripped, and which bytes of that content are
// code (gradleCode).
type settingsFile struct {
	path, content string
	code          []bool
}

func readSettingsFiles(dir string) []settingsFile {
	var files []settingsFile
	for _, name := range []string{"settings.gradle", "settings.gradle.kts"} {
		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		stripped := stripGradleComments(string(content))
		files = append(files, settingsFile{path: path, content: stripped, code: gradleCode(stripped, isGroovyScript(path))})
	}
	return files
}

// defaultCatalogName is the name Gradle imports gradle/libs.versions.toml under for a build with these settings
// files: the last literal defaultLibrariesExtensionName they set, else `libs`.
func defaultCatalogName(settings []settingsFile) string {
	name := defaultCatalogAccessor
	for _, file := range settings {
		for _, match := range defaultLibrariesExtensionNameRe.FindAllStringSubmatchIndex(file.content, -1) {
			if gradleCodeAt(file.code, match[0]) {
				name = file.content[match[2]:match[3]]
			}
		}
	}
	return name
}

// defaultCatalogAccessor is DependencyResolutionManagement.getDefaultLibrariesExtensionName's default.
const defaultCatalogAccessor = "libs"

func (c *CatalogDiscovery) catalogSourcesForDir(dir string) []CatalogSource {
	var sources []CatalogSource
	settings := readSettingsFiles(dir)
	catalogPaths, err := filepath.Glob(filepath.Join(dir, "gradle", "*.versions.toml"))
	if err == nil && len(catalogPaths) > 0 {
		declared := settingsDeclaredCatalogNames(settings)
		for _, catalogPath := range orderVersionCatalogPaths(catalogPaths) {
			source := CatalogSource{Path: catalogPath, Kind: CatalogSourceMergedTOML}
			// Gradle imports only this one by convention; with no settings file beside it, as in a partial checkout
			// or a fixture, it keeps the default name, as before #1872.
			if filepath.Base(catalogPath) == "libs.versions.toml" {
				source.Accessor = defaultCatalogName(settings)
			} else {
				source.Shadowed = declared[strings.TrimSuffix(filepath.Base(catalogPath), ".versions.toml")]
			}
			sources = append(sources, source)
		}
	}
	sources = append(sources, c.settingsCatalogSources(dir, settings)...)
	return sources
}

func (c *CatalogDiscovery) settingsCatalogSources(dir string, settings []settingsFile) []CatalogSource {
	var sources []CatalogSource
	for _, file := range settings {
		settingsPath, settingsContent := file.path, file.content
		blocks := settingsCatalogBlocks(settingsContent, file.code)
		for _, match := range settingsCatalogFileRe.FindAllStringSubmatchIndex(settingsContent, -1) {
			if !gradleCodeAt(file.code, match[0]) {
				continue
			}
			// from(files("path")), or Groovy's paren-less from files('path').
			path := match[2:4]
			if path[0] < 0 {
				path = match[4:6]
			}
			catalogPath := filepath.Clean(filepath.Join(dir, settingsContent[path[0]:path[1]]))
			sources = append(sources, CatalogSource{
				Path:     catalogPath,
				Kind:     CatalogSourceSettingsTOML,
				Accessor: catalogBlockAt(blocks, match[0]),
			})
		}
		if isProgrammaticVersionCatalogContent(settingsContent) {
			sources = append(sources, CatalogSource{Path: settingsPath, Kind: CatalogSourceSettingsProgrammatic})
		}
		sources = append(sources, c.includedSettingsPluginSources(dir, settingsContent)...)
	}
	return sources
}

func (c *CatalogDiscovery) includedSettingsPluginSources(settingsDir, settingsContent string) []CatalogSource {
	var sources []CatalogSource
	pluginIDs := make(map[string]bool)
	for _, match := range settingsPluginIDRe.FindAllStringSubmatch(settingsContent, -1) {
		pluginIDs[match[1]] = true
	}
	if len(pluginIDs) == 0 {
		return nil
	}
	for _, includeMatch := range settingsIncludeBuildRe.FindAllStringSubmatch(settingsContent, -1) {
		includeDir := filepath.Clean(filepath.Join(settingsDir, includeMatch[1]))
		sources = append(sources, precompiledSettingsPluginSources(includeDir, pluginIDs)...)
		sources = append(sources, c.binarySettingsPluginSources(includeDir, pluginIDs)...)
	}
	return sources
}

func precompiledSettingsPluginSources(includeDir string, pluginIDs map[string]bool) []CatalogSource {
	var sources []CatalogSource
	wantNames := make(map[string]bool, len(pluginIDs)*2)
	for pluginID := range pluginIDs {
		wantNames[pluginID+".settings.gradle.kts"] = true
		wantNames[pluginID+".settings.gradle"] = true
	}
	for _, languageDir := range []string{"kotlin", "groovy"} {
		sourceRoot := filepath.Join(includeDir, "src", "main", languageDir)
		_ = filepath.WalkDir(sourceRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
			}
			if d.IsDir() {
				return nil
			}
			if !wantNames[d.Name()] || !isProgrammaticVersionCatalogFile(path) {
				return nil
			}
			sources = append(sources, CatalogSource{Path: path, Kind: CatalogSourceSettingsProgrammatic})
			return nil
		})
	}
	return sources
}

func (c *CatalogDiscovery) binarySettingsPluginSources(includeDir string, pluginIDs map[string]bool) []CatalogSource {
	var sources []CatalogSource
	for _, buildPath := range c.gradleBuildFilesUnder(includeDir) {
		content, err := os.ReadFile(buildPath)
		if err != nil {
			continue
		}
		for _, className := range settingsPluginImplementationClasses(stripGradleComments(string(content)), pluginIDs) {
			if sourcePath, ok := findImplementationSource(includeDir, className); ok && isProgrammaticVersionCatalogFile(sourcePath) {
				sources = append(sources, CatalogSource{Path: sourcePath, Kind: CatalogSourceSettingsProgrammatic})
			}
		}
	}
	return sources
}

// gradleBuildFilesUnder lists build files under an included build. Nested builds
// often include the same root (includeBuild("..")), so the walk is memoized by root.
func (c *CatalogDiscovery) gradleBuildFilesUnder(root string) []string {
	if c == nil {
		return walkGradleBuildFilesFn(root)
	}
	if cached, ok := c.buildFiles.Load(root); ok {
		return cached.([]string)
	}
	paths, _ := c.buildFiles.LoadOrStore(root, walkGradleBuildFilesFn(root))
	return paths.([]string)
}

// walkGradleBuildFilesFn is replaced in tests to count directory walks.
var walkGradleBuildFilesFn = walkGradleBuildFiles

func walkGradleBuildFiles(root string) []string {
	var paths []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".gradle", "build", "out":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		switch d.Name() {
		case "build.gradle", "build.gradle.kts":
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	return paths
}

func settingsPluginImplementationClasses(content string, pluginIDs map[string]bool) []string {
	if !strings.Contains(content, "gradlePlugin") {
		return nil
	}
	var classes []string
	idMatches := gradlePluginIDFieldRe.FindAllStringSubmatchIndex(content, -1)
	for i, match := range idMatches {
		if len(match) < 4 {
			continue
		}
		pluginID := content[match[2]:match[3]]
		if !pluginIDs[pluginID] {
			continue
		}
		start := match[0] - 500
		if start < 0 {
			start = 0
		}
		end := len(content)
		if i+1 < len(idMatches) {
			end = idMatches[i+1][0]
		} else if candidate := match[1] + 1000; candidate < end {
			end = candidate
		}
		for _, implMatch := range gradlePluginImplClassRe.FindAllStringSubmatch(content[start:end], -1) {
			classes = append(classes, implMatch[1])
		}
	}
	return classes
}

func findImplementationSource(includeDir, className string) (string, bool) {
	relativeClassPath := filepath.FromSlash(strings.ReplaceAll(className, ".", "/") + ".kt")
	for _, languageDir := range []string{"kotlin", "groovy"} {
		sourcePath := filepath.Join(includeDir, "src", "main", languageDir, relativeClassPath)
		if _, err := os.Stat(sourcePath); err == nil {
			return sourcePath, true
		}
	}
	baseName := className
	if index := strings.LastIndex(baseName, "."); index >= 0 {
		baseName = baseName[index+1:]
	}
	for _, languageDir := range []string{"kotlin", "groovy"} {
		sourceRoot := filepath.Join(includeDir, "src", "main", languageDir)
		var found string
		_ = filepath.WalkDir(sourceRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil || found != "" {
				return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
			}
			if d.IsDir() {
				return nil
			}
			if d.Name() == baseName+".kt" || d.Name() == baseName+".groovy" {
				found = path
			}
			return nil
		})
		if found != "" {
			return found, true
		}
	}
	return "", false
}

func isProgrammaticVersionCatalogFile(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return isProgrammaticVersionCatalogContent(stripGradleComments(string(content)))
}

func isProgrammaticVersionCatalogContent(content string) bool {
	return strings.Contains(content, "versionCatalogs") &&
		(settingsLibraryRe.MatchString(content) || settingsPluginRe.MatchString(content))
}

func normalizeCatalogSourceKinds(sources []CatalogSource) {
	nonSettingsTOMLCount := 0
	for _, source := range sources {
		if source.Kind == CatalogSourceMergedTOML {
			nonSettingsTOMLCount++
		}
	}
	if nonSettingsTOMLCount != 1 {
		return
	}
	for i := range sources {
		if sources[i].Kind == CatalogSourceMergedTOML && filepath.Base(sources[i].Path) == "libs.versions.toml" {
			sources[i].Kind = CatalogSourceStandardTOML
			return
		}
	}
}

func orderVersionCatalogPaths(paths []string) []string {
	ordered := append([]string(nil), paths...)
	sort.Strings(ordered)
	for i, path := range ordered {
		if filepath.Base(path) == "libs.versions.toml" {
			copy(ordered[1:i+1], ordered[:i])
			ordered[0] = path
			break
		}
	}
	return ordered
}

func (c VersionCatalog) Empty() bool {
	return len(c.Versions) == 0 && len(c.Plugins) == 0 && len(c.Libraries) == 0 && len(c.Bundles) == 0
}

func (c *VersionCatalog) Merge(other VersionCatalog) {
	if c.Versions == nil {
		c.Versions = make(map[string]string)
	}
	if c.Plugins == nil {
		c.Plugins = make(map[string]CatalogPlugin)
	}
	if c.Libraries == nil {
		c.Libraries = make(map[string]CatalogLibrary)
	}
	if c.Bundles == nil {
		c.Bundles = make(map[string][]string)
	}
	for key, value := range other.Versions {
		c.Versions[key] = value
	}
	for key, value := range other.Plugins {
		c.Plugins[key] = value
	}
	for key, value := range other.Libraries {
		c.Libraries[key] = value
	}
	for key, value := range other.Bundles {
		c.Bundles[key] = append([]string(nil), value...)
	}
	c.Sources = appendCatalogSources(c.Sources, other.Sources...)
	if other.Completeness > c.Completeness {
		c.Completeness = other.Completeness
	}
}

func appendCatalogSources(current []CatalogSource, next ...CatalogSource) []CatalogSource {
	seen := make(map[string]bool, len(current)+len(next))
	for _, source := range current {
		seen[string(source.Kind)+"|"+source.Path] = true
	}
	for _, source := range next {
		key := string(source.Kind) + "|" + source.Path
		if source.Path == "" || seen[key] {
			continue
		}
		seen[key] = true
		current = append(current, source)
	}
	return current
}

func completenessForCatalogSource(kind CatalogSourceKind) CatalogCompleteness {
	switch kind {
	case CatalogSourceStandardTOML:
		return CatalogCompletenessStandardTOML
	case CatalogSourceMergedTOML:
		return CatalogCompletenessMergedTOML
	case CatalogSourceSettingsTOML:
		return CatalogCompletenessSettingsTOML
	case CatalogSourceSettingsProgrammatic:
		return CatalogCompletenessSettingsProgrammatic
	default:
		return CatalogCompletenessNone
	}
}

func catalogSourceCacheKey(sources []CatalogSource) string {
	var parts []string
	for _, source := range sources {
		parts = append(parts, string(source.Kind)+"="+source.Path)
	}
	return strings.Join(parts, "\x00")
}

func (c VersionCatalog) PluginByIDOrAlias(idOrAlias string) (CatalogPlugin, bool) {
	if plugin, ok := c.Plugins[idOrAlias]; ok {
		return plugin, true
	}
	for _, plugin := range c.Plugins {
		if plugin.ID == idOrAlias {
			return plugin, true
		}
	}
	return CatalogPlugin{}, false
}

func applyCatalogProfileFacts(profile *ProjectProfile, content string, cfgPluginIDs []string, catalog VersionCatalog, source string) {
	if catalog.Empty() {
		return
	}
	for _, match := range catalogPluginAliasRe.FindAllStringSubmatch(content, -1) {
		alias := catalogAccessorToAlias(match[1])
		if plugin, ok := catalog.Plugins[alias]; ok {
			applyCatalogPlugin(profile, plugin)
		}
	}
	for _, pluginID := range cfgPluginIDs {
		if plugin, ok := catalog.PluginByIDOrAlias(pluginID); ok {
			applyCatalogPlugin(profile, plugin)
		}
	}
	applyCatalogVersionAccessors(profile, content, catalog)
	applyCatalogLibraryDependencies(profile, content, catalog, source)
}

func applyCatalogPlugin(profile *ProjectProfile, plugin CatalogPlugin) {
	if plugin.ID == "" {
		return
	}
	switch {
	case plugin.ID == "com.google.devtools.ksp":
		profile.KSP.Tool = knownOrPresentLatest(plugin.Version)
	case isAndroidGradlePluginID(plugin.ID):
		profile.Android.AGP = knownOrPresentLatest(plugin.Version)
	case plugin.ID == "org.jetbrains.kotlin" || strings.HasPrefix(plugin.ID, "org.jetbrains.kotlin.") || strings.HasPrefix(plugin.ID, "kotlin-"):
		profile.Kotlin.Compiler = knownOrPresentLatest(plugin.Version)
	}
}

// catalogVersionAccessorSuffix / catalogKotlinVersionAccessorSuffix
// are the two trailing regex shapes used by applyCatalogVersionAccessors.
// Per-Gradle-file regex compilation was a measurable hot path on
// multi-module projects — the eight final patterns are now compiled
// once at package init.
const (
	catalogVersionAccessorSuffix       = `[^\n]*libs\.versions\.([A-Za-z0-9_.]+)\.get\s*\(`
	catalogKotlinVersionAccessorSuffix = `(?s:.{0,300}?)KotlinVersion(?s:.{0,200}?)libs\.versions\.([A-Za-z0-9_.]+)\.get\s*\(`
)

var (
	catalogAssignCompileSdkRe   = regexp.MustCompile(`compileSdk(?:Version)?\s*(?:=|\()` + catalogVersionAccessorSuffix)
	catalogAssignTargetSdkRe    = regexp.MustCompile(`targetSdk(?:Version)?\s*(?:=|\()` + catalogVersionAccessorSuffix)
	catalogAssignMinSdkRe       = regexp.MustCompile(`minSdk(?:Version)?\s*(?:=|\()` + catalogVersionAccessorSuffix)
	catalogAssignJvmTargetRe    = regexp.MustCompile(`jvmTarget(?:\.set)?\s*(?:=|\()` + catalogVersionAccessorSuffix)
	catalogAssignSourceCompatRe = regexp.MustCompile(`sourceCompatibility\s*(?:=|\()` + catalogVersionAccessorSuffix)
	catalogAssignTargetCompatRe = regexp.MustCompile(`targetCompatibility\s*(?:=|\()` + catalogVersionAccessorSuffix)
	catalogNearbyLanguageVerRe  = regexp.MustCompile(`languageVersion(?:\.set)?\s*(?:=|\()` + catalogKotlinVersionAccessorSuffix)
	catalogNearbyAPIVerRe       = regexp.MustCompile(`apiVersion(?:\.set)?\s*(?:=|\()` + catalogKotlinVersionAccessorSuffix)
)

func applyCatalogVersionAccessors(profile *ProjectProfile, content string, catalog VersionCatalog) {
	apply := func(re *regexp.Regexp, fn func(string)) {
		for _, match := range re.FindAllStringSubmatch(content, -1) {
			if version := catalog.Versions[catalogAccessorToAlias(match[1])]; version != "" {
				fn(normalizeVersion(version))
			}
		}
	}
	apply(catalogAssignCompileSdkRe, func(version string) {
		profile.Android.CompileSDK = KnownVersion(version)
		profile.CompileSdkVersion = parsePositiveInt(version, profile.CompileSdkVersion)
	})
	apply(catalogAssignTargetSdkRe, func(version string) {
		profile.Android.TargetSDK = KnownVersion(version)
		profile.TargetSdkVersion = parsePositiveInt(version, profile.TargetSdkVersion)
	})
	apply(catalogAssignMinSdkRe, func(version string) {
		profile.Android.MinSDK = KnownVersion(version)
		profile.MinSdkVersion = parsePositiveInt(version, profile.MinSdkVersion)
	})
	apply(catalogAssignJvmTargetRe, func(version string) {
		profile.JVM.KotlinJvmTarget = KnownVersion(version)
	})
	apply(catalogAssignSourceCompatRe, func(version string) {
		profile.JVM.SourceCompatibility = KnownVersion(version)
	})
	apply(catalogAssignTargetCompatRe, func(version string) {
		profile.JVM.TargetCompatibility = KnownVersion(version)
	})
	apply(catalogNearbyLanguageVerRe, func(version string) {
		profile.Kotlin.LanguageVersion = KnownVersion(version)
		if strings.HasPrefix(version, "2.") {
			profile.Kotlin.K2 = PresencePresent
		}
	})
	apply(catalogNearbyAPIVerRe, func(version string) {
		profile.Kotlin.APIVersion = KnownVersion(version)
	})
}

func applyCatalogLibraryDependencies(profile *ProjectProfile, content string, catalog VersionCatalog, source string) {
	seen := make(map[string]bool)
	for _, existing := range profile.Dependencies {
		seen[existing.Configuration+"|"+existing.Group+"|"+existing.Name] = true
	}
	for _, match := range catalogDependencyAliasRe.FindAllStringSubmatch(content, -1) {
		configuration := match[1]
		if isCatalogWrapperCall(configuration) {
			continue
		}
		if strings.HasPrefix(match[2], "versions.") || strings.HasPrefix(match[2], "plugins.") {
			continue
		}
		alias, bundle := catalogLibraryAccessorToAlias(match[2])
		if bundle {
			for _, libraryAlias := range catalog.Bundles[alias] {
				if library, ok := catalog.Libraries[libraryAlias]; ok {
					addCatalogDependency(profile, source, seen, configuration, library)
				}
			}
			continue
		}
		library, ok := catalog.Libraries[alias]
		if !ok {
			continue
		}
		addCatalogDependency(profile, source, seen, configuration, library)
	}
}

func addCatalogDependency(profile *ProjectProfile, source string, seen map[string]bool, configuration string, library CatalogLibrary) {
	key := configuration + "|" + library.Group + "|" + library.Name
	if seen[key] {
		return
	}
	seen[key] = true
	dep := Dependency{
		Group:         library.Group,
		Name:          library.Name,
		Version:       library.Version,
		Configuration: configuration,
		Source:        source,
	}
	profile.Dependencies = append(profile.Dependencies, dep)
	if configuration == "ksp" {
		profile.KSP.Processors = append(profile.KSP.Processors, dep)
		if profile.KSP.Tool.Presence == PresenceUnknown {
			profile.KSP.Tool = PresentAssumeLatestStable()
		}
	}
}

func unresolvedCatalogAliases(content string, catalog VersionCatalog, source string) []UnresolvedCatalogAlias {
	var aliases []UnresolvedCatalogAlias
	for _, match := range catalogPluginAliasRe.FindAllStringSubmatch(content, -1) {
		alias := catalogAccessorToAlias(match[1])
		if _, ok := catalog.Plugins[alias]; !ok {
			aliases = append(aliases, UnresolvedCatalogAlias{Kind: CatalogAliasPlugin, Alias: alias, Source: source})
		}
	}
	for _, match := range catalogVersionAccessorRe.FindAllStringSubmatch(content, -1) {
		alias := catalogAccessorToAlias(match[1])
		if _, ok := catalog.Versions[alias]; !ok {
			aliases = append(aliases, UnresolvedCatalogAlias{Kind: CatalogAliasVersion, Alias: alias, Source: source})
		}
	}
	for _, match := range catalogDependencyAliasRe.FindAllStringSubmatch(content, -1) {
		if isCatalogWrapperCall(match[1]) {
			continue
		}
		if strings.HasPrefix(match[2], "versions.") || strings.HasPrefix(match[2], "plugins.") {
			continue
		}
		alias, bundle := catalogLibraryAccessorToAlias(match[2])
		if bundle {
			if _, ok := catalog.Bundles[alias]; !ok {
				aliases = append(aliases, UnresolvedCatalogAlias{Kind: CatalogAliasBundle, Alias: alias, Source: source})
			}
			continue
		}
		if _, ok := catalog.Libraries[alias]; !ok {
			aliases = append(aliases, UnresolvedCatalogAlias{Kind: CatalogAliasLibrary, Alias: alias, Source: source})
		}
	}
	return aliases
}

func stripGradleComments(content string) string {
	var out strings.Builder
	out.Grow(len(content))
	inQuote := rune(0)
	escaped := false
	inLineComment := false
	inBlockComment := false
	for i := 0; i < len(content); i++ {
		ch := rune(content[i])
		next := rune(0)
		if i+1 < len(content) {
			next = rune(content[i+1])
		}
		if inLineComment {
			if ch == '\n' {
				inLineComment = false
				out.WriteRune(ch)
			}
			continue
		}
		if inBlockComment {
			if ch == '\n' {
				out.WriteRune(ch)
			}
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inQuote != 0 {
			out.WriteRune(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == inQuote {
				inQuote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			inQuote = ch
			out.WriteRune(ch)
			continue
		}
		if ch == '/' && next == '/' {
			inLineComment = true
			i++
			continue
		}
		if ch == '/' && next == '*' {
			inBlockComment = true
			i++
			continue
		}
		out.WriteRune(ch)
	}
	return out.String()
}

func catalogAccessorToAlias(accessor string) string {
	return strings.Join(strings.Split(accessor, "."), "-")
}

func catalogLibraryAccessorToAlias(accessor string) (string, bool) {
	accessor = strings.TrimSuffix(accessor, ".get")
	if strings.HasPrefix(accessor, "bundles.") {
		return catalogAccessorToAlias(strings.TrimPrefix(accessor, "bundles.")), true
	}
	return catalogAccessorToAlias(accessor), false
}

func isCatalogWrapperCall(name string) bool {
	switch name {
	case "alias", "files", "fileTree", "project", "platform", "enforcedPlatform", "npm":
		return true
	default:
		return false
	}
}

func isAndroidGradlePluginID(id string) bool {
	switch id {
	case "com.android.tools.build",
		"com.android.application",
		"com.android.library",
		"com.android.test",
		"com.android.dynamic-feature",
		"com.android.asset-pack",
		"com.android.kotlin.multiplatform.library":
		return true
	default:
		return false
	}
}

func knownOrPresentLatest(version string) VersionedPresence {
	if version != "" {
		return KnownVersion(normalizeVersion(version))
	}
	return PresentAssumeLatestStable()
}

func parsePositiveInt(value string, fallback int) int {
	intValue, err := strconv.Atoi(value)
	if err != nil || intValue <= 0 {
		return fallback
	}
	return intValue
}
