package rules

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// manifestGradleInfo holds the relevant build.gradle(.kts) facts for the
// module that owns an AndroidManifest.xml. Used by UsesSdkManifest and
// MissingVersionManifest to defer to gradle when the modern AGP project
// structure moves these values out of the manifest.
type manifestGradleInfo struct {
	found           bool
	hasMinSdk       bool
	hasVersionCode  bool
	hasVersionName  bool
	isApplication   bool
	isLibrary       bool
	isTest          bool
}

var (
	manifestGradleCacheMu sync.RWMutex
	manifestGradleCache   = map[string]manifestGradleInfo{}
)

// lookupManifestGradleInfo walks up from the manifest path looking for a
// build.gradle(.kts) file in the module directory (typically
// `<module>/src/main/AndroidManifest.xml` → `<module>/build.gradle(.kts)`)
// and returns cached module facts. Returns a zero-value struct if no gradle
// file is found.
func lookupManifestGradleInfo(manifestPath string) manifestGradleInfo {
	dir := filepath.Dir(manifestPath)
	// Walk up at most 4 levels to find module root.
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
	manifestGradleCacheMu.RLock()
	if cached, ok := manifestGradleCache[dir]; ok {
		manifestGradleCacheMu.RUnlock()
		return cached, cached.found
	}
	manifestGradleCacheMu.RUnlock()

	var path string
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			path = p
			break
		}
	}
	if path == "" {
		return manifestGradleInfo{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return manifestGradleInfo{}, false
	}
	content := string(data)
	info := manifestGradleInfo{found: true}
	// Regex-free substring scans — tolerant of Kotlin/Groovy DSL variance.
	lower := strings.ToLower(content)
	info.hasMinSdk = strings.Contains(lower, "minsdk")
	info.hasVersionCode = strings.Contains(lower, "versioncode")
	info.hasVersionName = strings.Contains(lower, "versionname")
	info.isApplication = strings.Contains(content, "com.android.application") ||
		strings.Contains(content, "applicationId") ||
		containsPluginSuffix(content, "sample-app") ||
		containsPluginSuffix(content, "application")
	info.isLibrary = strings.Contains(content, "com.android.library") ||
		containsPluginSuffix(content, "library")
	info.isTest = strings.Contains(content, "com.android.test")

	manifestGradleCacheMu.Lock()
	manifestGradleCache[dir] = info
	manifestGradleCacheMu.Unlock()
	return info, true
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
// id that ends in the given suffix (e.g. `id("signal-library")`,
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
