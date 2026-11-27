package rules

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/lazyre"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
)

// manifestGradleInfo holds the relevant build.gradle(.kts) facts for the
// module that owns an AndroidManifest.xml. Used by UsesSdkManifest and
// MissingVersionManifest to defer to gradle when the modern AGP project
// structure moves these values out of the manifest.
type manifestGradleInfo struct {
	found          bool
	hasMinSdk      bool
	hasTargetSdk   bool
	hasVersionCode bool
	hasVersionName bool
	isApplication  bool
	isLibrary      bool
	isTest         bool
	// hasDensityResConfig is true when resConfigs/resourceConfigurations names a density bucket.
	hasDensityResConfig bool
	targetSdk           int // targetSdk read from the build script; 0 when unset or defined elsewhere
	minSdk              int // minSdk read from the build script; 0 when unset or defined elsewhere
	versionCode         string
	versionName         string
	// namespace is the build script's `android { namespace = "..." }`, which supplies the package of a manifest
	// without one; "" when unset or computed.
	namespace string
	// usesSplashScreenLibrary is true when the build script mentions the SplashScreen library or declares it through
	// a version-catalog alias.
	usesSplashScreenLibrary bool

	permissionTargetSdkProven bool
}

// manifestBuildScriptNames are the build script names a manifest's module directory is searched for, in order.
var manifestBuildScriptNames = [...]string{"build.gradle.kts", "build.gradle"}

// manifestModuleSearchDirs returns, nearest first, the ancestor directories of a manifest searched for its module's
// build script: up to four, stopping at the filesystem root.
func manifestModuleSearchDirs(manifestPath string) []string {
	const depth = 4
	dirs := make([]string, 0, depth)
	dir := filepath.Dir(manifestPath)
	for range depth {
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
		dirs = append(dirs, dir)
	}
	return dirs
}

// ManifestBuildScriptCandidates returns every path lookupManifestGradleInfo can read a manifest's build script from,
// whether or not it exists, nearest first. The first one that exists is read (ManifestBuildScript), so a cache key
// over the rules that read it keys which one that is, searched again every run, and its content: creating a nearer
// one changes what is read.
func ManifestBuildScriptCandidates(manifestPath string) []string {
	dirs := manifestModuleSearchDirs(manifestPath)
	paths := make([]string, 0, len(dirs)*len(manifestBuildScriptNames))
	for _, dir := range dirs {
		for _, name := range manifestBuildScriptNames {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	return paths
}

// ManifestBuildScript returns the build script lookupManifestGradleInfo reads for a manifest: the first of its
// ManifestBuildScriptCandidates that exists, or "" when none does.
func ManifestBuildScript(manifestPath string) string {
	for _, path := range ManifestBuildScriptCandidates(manifestPath) {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// ReadsManifestGradleInfo reports whether any of the rules is a manifest or manifest-source rule. Those rules can
// read each manifest's build script through lookupManifestGradleInfo, and through it the version catalogs the
// script's aliases resolve against (BuildScriptCatalogReads), so result caches over them must key on those files.
func ReadsManifestGradleInfo(active []*api.Rule) bool {
	for _, r := range active {
		if scope := RuleScope(r); scope == api.ScopeManifest || scope == api.ScopeManifestSource {
			return true
		}
	}
	return false
}

// BuildScriptCatalogReads reports which version catalogs readManifestGradleInfo reads for a build script with this
// content. pluginAliases: an alias(<catalog>.plugins.x) entry (catalogPluginIDs). dependencyAliases: a libs.
// accessor (catalogDeclaresDependency). Both resolve through librarymodel's catalog discovery for the script, which
// reads every ancestor's gradle/*.versions.toml and settings files. A script with neither reads no catalog, so cache
// keys can skip the discovery: the script's own content is keyed, so editing an alias in moves the key anyway.
func BuildScriptCatalogReads(content string) (pluginAliases, dependencyAliases bool) {
	return len(catalogPluginAliases(content)) > 0, strings.Contains(content, "libs.")
}

// lookupManifestGradleInfo walks up from the manifest path looking for a
// build.gradle(.kts) file in the module directory (typically
// `<module>/src/main/AndroidManifest.xml` → `<module>/build.gradle(.kts)`)
// and returns cached module facts via the shared filefacts cache.
// Returns a zero-value struct if no gradle file is found.
func lookupManifestGradleInfo(manifestPath string) manifestGradleInfo {
	for _, dir := range manifestModuleSearchDirs(manifestPath) {
		if info, ok := tryReadGradleInfo(dir); ok {
			return info
		}
	}
	return manifestGradleInfo{}
}

func tryReadGradleInfo(dir string) (manifestGradleInfo, bool) {
	info := filefacts.RequestFact(fileFactsCache(), dir, slotManifestGradle, func() manifestGradleInfo {
		return readManifestGradleInfo(dir)
	})
	return info, info.found
}

func readManifestGradleInfo(dir string) manifestGradleInfo {
	var path string
	for _, name := range manifestBuildScriptNames {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			path = p
			break
		}
	}
	if path == "" {
		return manifestGradleInfo{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return manifestGradleInfo{}
	}
	content := string(data)
	info := manifestGradleInfo{found: true}
	info.permissionTargetSdkProven = lintPermissionLiteralTargetSdk(content)
	if cfg, err := android.ParseBuildGradleContent(content); err == nil && cfg != nil {
		info.targetSdk = cfg.TargetSdkVersion
		info.minSdk = cfg.MinSdkVersion
		info.namespace = cfg.Namespace
	}
	if m := gradleVersionCodePattern.FindStringSubmatch(content); m != nil {
		info.versionCode = m[1]
	}
	if m := gradleVersionNamePattern.FindStringSubmatch(content); m != nil {
		info.versionName = m[1]
	}
	info.hasDensityResConfig = gradleDensityResConfigPattern.MatchString(content)
	info.usesSplashScreenLibrary = strings.Contains(strings.ToLower(content), "splashscreen") ||
		catalogDeclaresDependency(path, content, "androidx.core", "core-splashscreen")
	lower := strings.ToLower(content)
	info.hasMinSdk = strings.Contains(lower, "minsdk")
	info.hasTargetSdk = strings.Contains(lower, "targetsdk")
	info.hasVersionCode = strings.Contains(lower, "versioncode")
	info.hasVersionName = strings.Contains(lower, "versionname")
	info.isApplication = strings.Contains(content, "com.android.application") ||
		strings.Contains(content, "applicationId") ||
		containsPluginSuffix(content, "sample-app") ||
		containsPluginSuffix(content, "application")
	info.isLibrary = strings.Contains(content, "com.android.library") ||
		containsPluginSuffix(content, "library")
	info.isTest = strings.Contains(content, "com.android.test")
	for _, id := range catalogPluginIDs(dir, content) {
		switch {
		case id == "com.android.application" || strings.HasPrefix(id, "com.android.application."):
			info.isApplication = true
		case id == "com.android.library" || strings.HasPrefix(id, "com.android.library."):
			info.isLibrary = true
		case id == "com.android.test":
			info.isTest = true
		}
	}
	return info
}

var (
	gradleVersionCodePattern = lazyre.New(`(?m)^\s*versionCode\s*=?\s*(\d+)\s*$`)
	// A density bucket named in a resConfigs or resourceConfigurations declaration.
	gradleDensityResConfigPattern = lazyre.New(`(?s)(?:resConfigs?|resourceConfigurations)[^\n]*["'](?:ldpi|mdpi|hdpi|xhdpi|xxhdpi|xxxhdpi|tvdpi)["']`)
	gradleVersionNamePattern      = lazyre.New(`(?m)^\s*versionName\s*=?\s*"([^"]*)"\s*$`)
	pluginAliasCallPattern        = lazyre.New(`alias\(\s*([A-Za-z_][\w.]*)\s*\)`)
)

// catalogDeclaresDependency reports a dependency on group:name declared through a version-catalog alias or bundle,
// such as `implementation(libs.core.splash)`, which Lint sees in the resolved dependency graph.
func catalogDeclaresDependency(gradlePath, content, group, name string) bool {
	if _, dependencyAliases := BuildScriptCatalogReads(content); !dependencyAliases {
		return false
	}
	// The result is cached in the slotManifestGradle fact of the same cache, so the catalog lives as long as it does.
	catalog, ok := loadGradleVersionCatalog(fileFactsCache(), gradlePath)
	if !ok {
		return false
	}
	for _, dep := range librarymodel.ProfileFromGradleContentWithCatalog(gradlePath, content, catalog).Dependencies {
		if dep.Group == group && dep.Name == name {
			return true
		}
	}
	return false
}

// catalogPluginIDs resolves `alias(<accessor>.plugins.x.y)` entries in a build script to plugin ids through the
// catalogs librarymodel's discovery finds for the module, the settings-declared ones included, so
// `alias(libs.plugins.android.library)` counts as com.android.library. An alias reads only the catalog its accessor
// names (librarymodel.CatalogPluginsByAccessor), as Gradle generates one extension per catalog (#1872).
func catalogPluginIDs(moduleDir, buildScript string) []string {
	aliases := catalogPluginAliases(buildScript)
	if len(aliases) == 0 {
		return nil
	}
	// Discovery searches from the script's directory; the file name doesn't matter.
	sources, ok := catalogDiscovery(fileFactsCache()).FindVersionCatalogSourcesForGradlePath(filepath.Join(moduleDir, manifestBuildScriptNames[0]))
	if !ok {
		return nil
	}
	catalogs := librarymodel.LoadCatalogPluginsByAccessor(sources)
	tables := map[string]map[string]string{}
	var ids []string
	for _, alias := range aliases {
		table, seen := tables[alias.accessor]
		if !seen {
			table = normalizedCatalogPlugins(catalogs.Plugins(alias.accessor))
			tables[alias.accessor] = table
		}
		if id, ok := table[alias.key]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// normalizedCatalogPlugins maps each alias, normalized as normalizeCatalogKey does, to its plugin id. Of aliases that
// normalize alike, which Gradle rejects as clashing accessors, the first in sorted order wins.
func normalizedCatalogPlugins(plugins map[string]librarymodel.CatalogPlugin) map[string]string {
	aliases := make([]string, 0, len(plugins))
	for alias := range plugins {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	table := make(map[string]string, len(aliases))
	for _, alias := range aliases {
		key := normalizeCatalogKey(alias)
		if _, dup := table[key]; !dup && plugins[alias].ID != "" {
			table[key] = plugins[alias].ID
		}
	}
	return table
}

// catalogPluginAlias is one alias(<accessor>.plugins.<key>) entry of a build script, its key normalized.
type catalogPluginAlias struct {
	accessor, key string
}

// catalogPluginAliases returns the build script's alias(<accessor>.plugins.x) entries.
func catalogPluginAliases(buildScript string) []catalogPluginAlias {
	var aliases []catalogPluginAlias
	for _, match := range pluginAliasCallPattern.FindAllStringSubmatch(buildScript, -1) {
		if accessor, key, found := strings.Cut(match[1], ".plugins."); found {
			aliases = append(aliases, catalogPluginAlias{accessor: accessor, key: normalizeCatalogKey(key)})
		}
	}
	return aliases
}

func normalizeCatalogKey(key string) string {
	return strings.ToLower(strings.NewReplacer("-", ".", "_", ".").Replace(key))
}

// isLibraryOrTestModuleManifest reports whether the manifest lives in a
// module whose build.gradle applies a library or test plugin. Used by
// application-level manifest rules (allowBackup, supportsRtl, etc.) to
// suppress findings on modules whose application attributes merge in
// from the parent app manifest.
func isLibraryOrTestModuleManifest(manifestPath string) bool {
	gi := lookupManifestGradleInfo(manifestPath)
	if !gi.found {
		return false
	}
	// A module is "application-like" if it is an Android application or
	// its build.gradle declares `applicationId`. Library / test / unknown
	// modules default to "inherits from parent".
	if gi.isApplication {
		return false
	}
	return gi.isLibrary || gi.isTest
}

// containsPluginSuffix reports whether the build file references a plugin
// id that ends in the given suffix (e.g. `id("analytics-library")`,
// `id("my-android-application")`). This catches convention plugins that
// wrap com.android.application / com.android.library without naming them
// directly.
func containsPluginSuffix(content, suffix string) bool {
	needles := []string{
		`"` + suffix + `"`,
		`'` + suffix + `'`,
		`-` + suffix + `"`,
		`-` + suffix + `'`,
	}
	for _, n := range needles {
		if strings.Contains(content, n) {
			return true
		}
	}
	return false
}

// manifestTargetSDK returns the manifest's targetSdkVersion, or the module build script's targetSdk when the
// manifest declares none. It is 0 when neither is known.
func manifestTargetSDK(m *Manifest) int {
	if m.TargetSDK > 0 {
		return m.TargetSDK
	}
	if gi := lookupManifestGradleInfo(m.Path); gi.found {
		return gi.targetSdk
	}
	return 0
}

// manifestEffectiveMinSdk is the minSdk Lint judges the module by: the build script wins over the
// manifest, and a module that declares neither counts as API 1.
func manifestEffectiveMinSdk(m *Manifest) int {
	if info := lookupManifestGradleInfo(m.Path); info.minSdk > 0 {
		return info.minSdk
	}
	if m.MinSDK > 0 {
		return m.MinSDK
	}
	return 1
}

var manifestMinSdkRe = lazyre.New(`android:minSdkVersion\s*=\s*"(\d+)"`)

// resourceModuleMinSdk returns the minSdk of the module owning a res file: the build script value, then the
// manifest's uses-sdk, then 0 when the module declares neither (a convention plugin may set it elsewhere).
func resourceModuleMinSdk(resFile string) int {
	resRoot := filepath.Dir(filepath.Dir(resFile))
	manifest := filepath.Join(filepath.Dir(resRoot), "AndroidManifest.xml")
	if info := lookupManifestGradleInfo(manifest); info.minSdk > 0 {
		return info.minSdk
	}
	if data, err := os.ReadFile(manifest); err == nil {
		if m := manifestMinSdkRe.FindSubmatch(data); m != nil {
			n, _ := strconv.Atoi(string(m[1]))
			return n
		}
	}
	return 0
}
