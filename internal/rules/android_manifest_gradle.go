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
// whether or not it exists. The first one that exists is read, so a cache key over the rules that read it hashes all
// of them, a missing one included: creating it changes what is read.
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
	info := filefacts.StringFact(fileFactsCache(), dir, slotManifestGradle, func() manifestGradleInfo {
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
	catalogPluginEntryPattern     = lazyre.New(`(?m)^\s*([A-Za-z0-9_.-]+)\s*=\s*\{[^}]*\bid\s*=\s*"([^"]+)"`)
)

// catalogDeclaresDependency reports a dependency on group:name declared through a version-catalog alias or bundle,
// such as `implementation(libs.core.splash)`, which Lint sees in the resolved dependency graph.
func catalogDeclaresDependency(gradlePath, content, group, name string) bool {
	if !strings.Contains(content, "libs.") {
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

// catalogPluginIDs resolves `alias(libs.plugins.x.y)` entries in a build script to plugin ids through the
// project's Gradle version catalogs, so `alias(libs.plugins.android.library)` counts as com.android.library.
func catalogPluginIDs(moduleDir, buildScript string) []string {
	aliases := pluginAliasCallPattern.FindAllStringSubmatch(buildScript, -1)
	if len(aliases) == 0 {
		return nil
	}
	catalogs := versionCatalogsFor(moduleDir)
	var ids []string
	for _, match := range aliases {
		_, key, found := strings.Cut(match[1], ".plugins.")
		if !found {
			continue
		}
		key = normalizeCatalogKey(key)
		for _, catalog := range catalogs {
			if id, ok := catalog[key]; ok {
				ids = append(ids, id)
				break
			}
		}
	}
	return ids
}

func normalizeCatalogKey(key string) string {
	return strings.ToLower(strings.NewReplacer("-", ".", "_", ".").Replace(key))
}

// versionCatalogsFor loads the plugin tables of every gradle/*.versions.toml found at or above moduleDir.
func versionCatalogsFor(moduleDir string) []map[string]string {
	_, matches := VersionCatalogSearch(moduleDir, VersionCatalogGlob)
	if len(matches) == 0 {
		return nil
	}
	catalogs := make([]map[string]string, 0, len(matches))
	for _, path := range matches {
		catalogs = append(catalogs, loadCatalogPlugins(path))
	}
	return catalogs
}

// VersionCatalogSearch is the version-catalog search of a module directory: it globs, through glob, the
// directories at and above moduleDir, nearest first and at most 12, and stops at the first with a catalog. It
// returns the directories it globbed and that directory's catalogs, nil when none was found. A cache key over the
// rules that read catalogs hashes both: a catalog created in a globbed directory changes what is read.
func VersionCatalogSearch(moduleDir string, glob func(dir string) []string) (searched, matches []string) {
	dir := moduleDir
	for range 12 {
		searched = append(searched, dir)
		if matches := glob(dir); len(matches) > 0 {
			return searched, matches
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return searched, nil
}

// VersionCatalogGlob returns, sorted, the gradle/*.versions.toml catalogs of dir.
func VersionCatalogGlob(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, "gradle", "*.versions.toml"))
	sort.Strings(matches)
	return matches
}

func loadCatalogPlugins(path string) map[string]string {
	return filefacts.StringFact(fileFactsCache(), path, slotVersionCatalogPlugins, func() map[string]string {
		plugins := map[string]string{}
		data, err := os.ReadFile(path)
		if err != nil {
			return plugins
		}
		text := string(data)
		if start := strings.Index(text, "[plugins]"); start >= 0 {
			section := text[start+len("[plugins]"):]
			if end := strings.Index(section, "\n["); end >= 0 {
				section = section[:end]
			}
			for _, match := range catalogPluginEntryPattern.FindAllStringSubmatch(section, -1) {
				plugins[normalizeCatalogKey(match[1])] = match[2]
			}
		}
		return plugins
	})
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
