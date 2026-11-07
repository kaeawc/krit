package rules

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/filefacts"
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
}

// lookupManifestGradleInfo walks up from the manifest path looking for a
// build.gradle(.kts) file in the module directory (typically
// `<module>/src/main/AndroidManifest.xml` → `<module>/build.gradle(.kts)`)
// and returns cached module facts via the shared filefacts cache.
// Returns a zero-value struct if no gradle file is found.
func lookupManifestGradleInfo(manifestPath string) manifestGradleInfo {
	dir := filepath.Dir(manifestPath)
	for i := 0; i < 4; i++ {
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
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
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
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
	if cfg, err := android.ParseBuildGradleContent(content); err == nil && cfg != nil {
		info.targetSdk = cfg.TargetSdkVersion
		info.minSdk = cfg.MinSdkVersion
	}
	if m := gradleVersionCodePattern.FindStringSubmatch(content); m != nil {
		info.versionCode = m[1]
	}
	if m := gradleVersionNamePattern.FindStringSubmatch(content); m != nil {
		info.versionName = m[1]
	}
	info.hasDensityResConfig = gradleDensityResConfigPattern.MatchString(content)
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
	gradleVersionCodePattern = regexp.MustCompile(`(?m)^\s*versionCode\s*=?\s*(\d+)\s*$`)
	// A density bucket named in a resConfigs or resourceConfigurations declaration.
	gradleDensityResConfigPattern = regexp.MustCompile(`(?s)(?:resConfigs?|resourceConfigurations)[^\n]*["'](?:ldpi|mdpi|hdpi|xhdpi|xxhdpi|xxxhdpi|tvdpi)["']`)
	gradleVersionNamePattern      = regexp.MustCompile(`(?m)^\s*versionName\s*=?\s*"([^"]*)"\s*$`)
	pluginAliasCallPattern        = regexp.MustCompile(`alias\(\s*([A-Za-z_][\w.]*)\s*\)`)
	catalogPluginEntryPattern     = regexp.MustCompile(`(?m)^\s*([A-Za-z0-9_.-]+)\s*=\s*\{[^}]*\bid\s*=\s*"([^"]+)"`)
)

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
	dir := moduleDir
	for range 12 {
		matches, _ := filepath.Glob(filepath.Join(dir, "gradle", "*.versions.toml"))
		if len(matches) > 0 {
			sort.Strings(matches)
			catalogs := make([]map[string]string, 0, len(matches))
			for _, path := range matches {
				catalogs = append(catalogs, loadCatalogPlugins(path))
			}
			return catalogs
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil
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

var manifestMinSdkRe = regexp.MustCompile(`android:minSdkVersion\s*=\s*"(\d+)"`)

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
