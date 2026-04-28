package librarymodel

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
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
	catalogSectionRe          = regexp.MustCompile(`^\[([A-Za-z0-9_.-]+)]$`)
	catalogStringFieldRe      = regexp.MustCompile(`([A-Za-z0-9_.-]+)\s*=\s*"([^"]*)"`)
	catalogQuotedValueRe      = regexp.MustCompile(`^"([^"]*)"$`)
	catalogQuotedStringRe     = regexp.MustCompile(`"([^"]*)"`)
	catalogPluginAliasRe      = regexp.MustCompile(`alias\s*\(\s*libs\.plugins\.([A-Za-z0-9_.]+)\s*\)`)
	catalogVersionAccessorRe  = regexp.MustCompile(`libs\.versions\.([A-Za-z0-9_.]+)\.get\s*\(`)
	catalogDependencyAliasRe  = regexp.MustCompile(`["']?([A-Za-z_][A-Za-z0-9_]*)["']?\s*\(\s*(?:platform\s*\(\s*)?libs\.([A-Za-z0-9_.]+)`)
	unresolvedStructuralRefRe = regexp.MustCompile(`(?m)(?:["']?[A-Za-z_][A-Za-z0-9_]*["']?\s*\(\s*(?:platform\s*\(\s*)?(?:versions\.|deps\.|project\s*\(|projects\.)|apply\s*(?:\(|from\s*=|plugin\s*:)|id\s*\(\s*["'][^"']*(?:convention|build-logic)[^"']*["']|plugins?\s*\{[^}]*\b(?:convention|build-logic)\b)`)
	settingsCatalogFileRe     = regexp.MustCompile(`from\s*\(\s*files\s*\(\s*["']([^"']+)["']\s*\)\s*\)`)
	settingsLibraryRe         = regexp.MustCompile(`library\s*\(\s*["']([^"']+)["']\s*,\s*["']([^"']+)["']\s*,\s*["']([^"']+)["']\s*\)\s*(?:\.\s*(?:version\s*\(\s*(?:"([^"]+)"|'([^']+)'|[A-Za-z0-9_.]+)\s*\)|withoutVersion\s*\(\s*\)))?`)
	settingsPluginRe          = regexp.MustCompile(`plugin\s*\(\s*["']([^"']+)["']\s*,\s*["']([^"']+)["']\s*\)\s*(?:\.\s*version\s*\(\s*(?:"([^"]+)"|'([^']+)'|[A-Za-z0-9_.]+)\s*\))?`)
	settingsPluginIDRe        = regexp.MustCompile(`id\s*\(\s*["']([A-Za-z0-9_.-]+)["']\s*\)`)
	settingsIncludeBuildRe    = regexp.MustCompile(`includeBuild\s*\(\s*["']([^"']+)["']\s*\)`)
	gradlePluginIDFieldRe     = regexp.MustCompile(`\bid(?:\.set)?\s*(?:=|\()\s*["']([^"']+)["']`)
	gradlePluginImplClassRe   = regexp.MustCompile(`\bimplementationClass(?:\.set)?\s*(?:=|\()\s*["']([^"']+)["']`)
)

func ParseVersionCatalogContent(content string) VersionCatalog {
	catalog := VersionCatalog{
		Versions:  make(map[string]string),
		Plugins:   make(map[string]CatalogPlugin),
		Libraries: make(map[string]CatalogLibrary),
		Bundles:   make(map[string][]string),
	}
	section := ""
	pendingBundleKey := ""
	pendingBundleValue := ""
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(stripTomlComment(rawLine))
		if line == "" {
			continue
		}
		if pendingBundleKey != "" {
			pendingBundleValue += " " + line
			if strings.Contains(line, "]") {
				if aliases := parseTomlStringArray(pendingBundleValue); len(aliases) > 0 {
					catalog.Bundles[pendingBundleKey] = aliases
				}
				pendingBundleKey = ""
				pendingBundleValue = ""
			}
			continue
		}
		if match := catalogSectionRe.FindStringSubmatch(line); len(match) == 2 {
			section = match[1]
			continue
		}
		key, value, ok := splitTomlAssignment(line)
		if !ok {
			continue
		}
		switch section {
		case "versions":
			if version := parseQuotedTomlValue(value); version != "" {
				catalog.Versions[key] = version
			}
		case "plugins":
			if coordinate := parseQuotedTomlValue(value); coordinate != "" {
				if plugin, ok := parseCatalogPluginCoordinate(coordinate); ok {
					catalog.Plugins[key] = plugin
				}
				continue
			}
			fields := parseInlineTomlFields(value)
			id := fields["id"]
			if id == "" {
				continue
			}
			catalog.Plugins[key] = CatalogPlugin{ID: id, Version: catalog.resolveVersion(fields)}
		case "libraries":
			if coordinate := parseQuotedTomlValue(value); coordinate != "" {
				if library, ok := parseCatalogLibraryCoordinate(coordinate); ok {
					catalog.Libraries[key] = library
				}
				continue
			}
			fields := parseInlineTomlFields(value)
			group := fields["group"]
			name := fields["name"]
			if module := fields["module"]; module != "" {
				parts := strings.SplitN(module, ":", 2)
				if len(parts) == 2 {
					group = parts[0]
					name = parts[1]
				}
			}
			if group == "" || name == "" {
				continue
			}
			catalog.Libraries[key] = CatalogLibrary{Group: group, Name: name, Version: catalog.resolveVersion(fields)}
		case "bundles":
			if strings.Contains(value, "[") && !strings.Contains(value, "]") {
				pendingBundleKey = key
				pendingBundleValue = value
				continue
			}
			if aliases := parseTomlStringArray(value); len(aliases) > 0 {
				catalog.Bundles[key] = aliases
			}
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
		version := firstNonEmpty(match[4], match[5])
		catalog.Libraries[match[1]] = CatalogLibrary{
			Group:   match[2],
			Name:    match[3],
			Version: version,
		}
	}
	for _, match := range settingsPluginRe.FindAllStringSubmatch(content, -1) {
		catalog.Plugins[match[1]] = CatalogPlugin{
			ID:      match[2],
			Version: firstNonEmpty(match[3], match[4]),
		}
	}
	return catalog
}

func LoadVersionCatalogForGradlePath(gradlePath string) (VersionCatalog, bool) {
	sources, ok := FindVersionCatalogSourcesForGradlePath(gradlePath)
	if !ok {
		return VersionCatalog{}, false
	}
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

func FindVersionCatalogPathForGradlePath(gradlePath string) (string, bool) {
	catalogPaths, ok := FindVersionCatalogPathsForGradlePath(gradlePath)
	if !ok {
		return "", false
	}
	return catalogPaths[0], true
}

func FindVersionCatalogPathsForGradlePath(gradlePath string) ([]string, bool) {
	sources, ok := FindVersionCatalogSourcesForGradlePath(gradlePath)
	if !ok {
		return nil, false
	}
	paths := make([]string, 0, len(sources))
	seen := make(map[string]bool)
	for _, source := range sources {
		if source.Kind == CatalogSourceSettingsProgrammatic || seen[source.Path] {
			continue
		}
		seen[source.Path] = true
		paths = append(paths, source.Path)
	}
	return paths, len(paths) > 0
}

func FindVersionCatalogSourcesForGradlePath(gradlePath string) ([]CatalogSource, bool) {
	if absPath, err := filepath.Abs(gradlePath); err == nil {
		gradlePath = absPath
	}
	type sourceGroup struct {
		dir     string
		sources []CatalogSource
	}
	var groups []sourceGroup
	for dir := filepath.Dir(gradlePath); dir != "." && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
		var sources []CatalogSource
		catalogPaths, err := filepath.Glob(filepath.Join(dir, "gradle", "*.versions.toml"))
		if err == nil && len(catalogPaths) > 0 {
			for _, catalogPath := range orderVersionCatalogPaths(catalogPaths) {
				sources = append(sources, CatalogSource{Path: catalogPath, Kind: CatalogSourceMergedTOML})
			}
		}
		sources = append(sources, settingsCatalogSources(dir)...)
		if len(sources) > 0 {
			groups = append(groups, sourceGroup{dir: dir, sources: sources})
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
		ordered = append(ordered, groups[i].sources...)
	}
	normalizeCatalogSourceKinds(ordered)
	return ordered, true
}

func settingsCatalogSources(dir string) []CatalogSource {
	var sources []CatalogSource
	for _, name := range []string{"settings.gradle", "settings.gradle.kts"} {
		settingsPath := filepath.Join(dir, name)
		content, err := os.ReadFile(settingsPath)
		if err != nil {
			continue
		}
		settingsContent := stripGradleComments(string(content))
		for _, match := range settingsCatalogFileRe.FindAllStringSubmatch(settingsContent, -1) {
			catalogPath := filepath.Clean(filepath.Join(dir, match[1]))
			sources = append(sources, CatalogSource{Path: catalogPath, Kind: CatalogSourceSettingsTOML})
		}
		if isProgrammaticVersionCatalogContent(settingsContent) {
			sources = append(sources, CatalogSource{Path: settingsPath, Kind: CatalogSourceSettingsProgrammatic})
		}
		sources = append(sources, includedSettingsPluginSources(dir, settingsContent)...)
	}
	return sources
}

func includedSettingsPluginSources(settingsDir, settingsContent string) []CatalogSource {
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
		sources = append(sources, binarySettingsPluginSources(includeDir, pluginIDs)...)
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
				return nil
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

func binarySettingsPluginSources(includeDir string, pluginIDs map[string]bool) []CatalogSource {
	var sources []CatalogSource
	for _, buildPath := range gradleBuildFilesUnder(includeDir) {
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

func gradleBuildFilesUnder(root string) []string {
	var paths []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
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
				return nil
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

func (c VersionCatalog) resolveVersion(fields map[string]string) string {
	if version := fields["version"]; version != "" {
		return version
	}
	if ref := fields["version.ref"]; ref != "" {
		return c.Versions[ref]
	}
	return ""
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

func applyCatalogVersionAccessors(profile *ProjectProfile, content string, catalog VersionCatalog) {
	applyAssignmentVersion := func(pattern string, apply func(string)) {
		re := regexp.MustCompile(pattern + `[^\n]*libs\.versions\.([A-Za-z0-9_.]+)\.get\s*\(`)
		for _, match := range re.FindAllStringSubmatch(content, -1) {
			if version := catalog.Versions[catalogAccessorToAlias(match[1])]; version != "" {
				apply(normalizeVersion(version))
			}
		}
	}
	applyNearbyKotlinVersion := func(pattern string, apply func(string)) {
		re := regexp.MustCompile(pattern + `(?s:.{0,300}?)KotlinVersion(?s:.{0,200}?)libs\.versions\.([A-Za-z0-9_.]+)\.get\s*\(`)
		for _, match := range re.FindAllStringSubmatch(content, -1) {
			if version := catalog.Versions[catalogAccessorToAlias(match[1])]; version != "" {
				apply(normalizeVersion(version))
			}
		}
	}
	applyAssignmentVersion(`compileSdk(?:Version)?\s*(?:=|\()`, func(version string) {
		profile.Android.CompileSDK = KnownVersion(version)
		profile.CompileSdkVersion = parsePositiveInt(version, profile.CompileSdkVersion)
	})
	applyAssignmentVersion(`targetSdk(?:Version)?\s*(?:=|\()`, func(version string) {
		profile.Android.TargetSDK = KnownVersion(version)
		profile.TargetSdkVersion = parsePositiveInt(version, profile.TargetSdkVersion)
	})
	applyAssignmentVersion(`minSdk(?:Version)?\s*(?:=|\()`, func(version string) {
		profile.Android.MinSDK = KnownVersion(version)
		profile.MinSdkVersion = parsePositiveInt(version, profile.MinSdkVersion)
	})
	applyAssignmentVersion(`jvmTarget(?:\.set)?\s*(?:=|\()`, func(version string) {
		profile.JVM.KotlinJvmTarget = KnownVersion(version)
	})
	applyAssignmentVersion(`sourceCompatibility\s*(?:=|\()`, func(version string) {
		profile.JVM.SourceCompatibility = KnownVersion(version)
	})
	applyAssignmentVersion(`targetCompatibility\s*(?:=|\()`, func(version string) {
		profile.JVM.TargetCompatibility = KnownVersion(version)
	})
	applyNearbyKotlinVersion(`languageVersion(?:\.set)?\s*(?:=|\()`, func(version string) {
		profile.Kotlin.LanguageVersion = KnownVersion(version)
		if strings.HasPrefix(version, "2.") {
			profile.Kotlin.K2 = PresencePresent
		}
	})
	applyNearbyKotlinVersion(`apiVersion(?:\.set)?\s*(?:=|\()`, func(version string) {
		profile.Kotlin.ApiVersion = KnownVersion(version)
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

func hasUnresolvedGradleRefs(content string, catalog VersionCatalog) bool {
	if unresolvedStructuralRefRe.MatchString(content) {
		return true
	}
	return len(unresolvedCatalogAliases(content, catalog, "")) > 0
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

func splitTomlAssignment(line string) (string, string, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func stripTomlComment(line string) string {
	inQuote := false
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inQuote {
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if r == '#' && !inQuote {
			return line[:i]
		}
	}
	return line
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

func parseQuotedTomlValue(value string) string {
	match := catalogQuotedValueRe.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func parseInlineTomlFields(value string) map[string]string {
	fields := make(map[string]string)
	for _, match := range catalogStringFieldRe.FindAllStringSubmatch(value, -1) {
		fields[match[1]] = match[2]
	}
	return fields
}

func parseTomlStringArray(value string) []string {
	start := strings.Index(value, "[")
	end := strings.LastIndex(value, "]")
	if start < 0 || end <= start {
		return nil
	}
	var values []string
	for _, match := range catalogQuotedStringRe.FindAllStringSubmatch(value[start:end+1], -1) {
		values = append(values, match[1])
	}
	return values
}

func parseCatalogLibraryCoordinate(coordinate string) (CatalogLibrary, bool) {
	parts := strings.SplitN(coordinate, ":", 3)
	if len(parts) < 2 {
		return CatalogLibrary{}, false
	}
	library := CatalogLibrary{Group: parts[0], Name: parts[1]}
	if len(parts) == 3 {
		library.Version = parts[2]
	}
	return library, true
}

func parseCatalogPluginCoordinate(coordinate string) (CatalogPlugin, bool) {
	index := strings.LastIndex(coordinate, ":")
	if index <= 0 || index == len(coordinate)-1 {
		return CatalogPlugin{}, false
	}
	return CatalogPlugin{ID: coordinate[:index], Version: coordinate[index+1:]}, true
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func parsePositiveInt(value string, fallback int) int {
	intValue, err := strconv.Atoi(value)
	if err != nil || intValue <= 0 {
		return fallback
	}
	return intValue
}
