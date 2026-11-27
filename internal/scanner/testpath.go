package scanner

import (
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

var defaultTestPaths = [...]string{
	"/test/", "/androidTest/", "/commonTest/", "/jvmTest/", "/jvmAndroidTest/",
	"/commonJvmTest/", "/browserCommonTest/", "/jvmCommonTest/",
	"/androidUnitTest/", "/androidInstrumentedTest/", "/jsTest/", "/iosTest/",
	"/nativeTest/", "/nonJvmCommonTest/",
	"/testShared/", "/sharedTest/",
	"/benchmark/", "/canary/",
	"/test-utils/",
	"/javatests/", "/kotlintests/", "/javatest/", "/kotlintest/",
	"/functionalTest/", "/functionaltests/",
	"/test/resources/", "/testResources/", "/testFixtures/",
	"/integration-tests/", "/integrationTest/",
	"/nonEmulatorCommonTest/", "/nonEmulatorJvmTest/",
	"/testData/", "/testdata/", "/test-data/",
	"/test/data/", "/compiler-tests/", "/compilertests/",
}

var (
	testPathMu        sync.RWMutex
	testPaths         = defaultTestPathSlice()
	testPathsOverride bool
	// configuredTestPaths are the markers testPaths took from config (the
	// additions or the override), for IsTestFileIn.
	configuredTestPaths []string
)

func InitTestPaths(config []string, override []string) {
	next := defaultTestPathSlice()
	var configured []string
	if len(override) > 0 {
		configured = cleanTestPaths(override)
		next = configured
	} else if len(config) > 0 {
		configured = cleanTestPaths(config)
		next = append(next, configured...)
	}
	testPathMu.Lock()
	testPaths = next
	testPathsOverride = len(override) > 0
	configuredTestPaths = configured
	testPathMu.Unlock()
	// Swap after the markers change so a result computed from old markers is never served from the new memo.
	testPathMemo.Store(new(sync.Map))
}

// testPathMemo holds IsTestFile results by path; rules ask for the same file once per node.
var testPathMemo atomic.Pointer[sync.Map]

func init() { testPathMemo.Store(new(sync.Map)) }

// IsTestFile reports whether path is under a test source set or a configured test marker.
func IsTestFile(path string) bool {
	memo := testPathMemo.Load()
	if v, ok := memo.Load(path); ok {
		return v.(bool)
	}
	result := isTestFileUncached(path)
	memo.Store(path, result)
	return result
}

// IsTestFileIn is IsTestFile for a file of the project rooted at root: the
// markers match RootRelativePath(root, path), so a folder above the root
// cannot mark the file and the verdict survives the checkout moving
// (#1814). A configured marker spelled as an absolute path inside root
// ("/work/repo/spec/") still matches, in its root-relative form
// (RootRelativeMarker). An empty root matches path as given.
func IsTestFileIn(root, path string) bool {
	return IsTestFileAt(root, path, RootRelativePath(root, path))
}

// IsTestFileAt is IsTestFileIn for a caller that already holds
// rel = RootRelativePath(root, path).
func IsTestFileAt(root, path, rel string) bool {
	if IsTestFile(rel) {
		return true
	}
	if rel == path {
		return false
	}
	testPathMu.RLock()
	defer testPathMu.RUnlock()
	for _, marker := range configuredTestPaths {
		if inRoot := RootRelativeMarker(root, marker); inRoot != marker && strings.Contains(rel, inRoot) {
			return true
		}
	}
	return false
}

// RootRelativeMarker returns a configured path marker spelled as an
// absolute path inside root in the form RootRelativePath gives paths
// ("/work/repo/tools/" under root /work/repo is "/tools/"), so the marker
// still matches the root-relative path; any other marker is returned
// unchanged and keeps matching only inside the project.
func RootRelativeMarker(root, marker string) string {
	if root == "" || marker == "" {
		return marker
	}
	slashRoot := strings.TrimSuffix(filepath.ToSlash(root), "/")
	slash := filepath.ToSlash(marker)
	if rest, ok := strings.CutPrefix(slash, slashRoot+"/"); ok {
		return "/" + rest
	}
	if slash == slashRoot {
		return "/"
	}
	return marker
}

func isTestFileUncached(path string) bool {
	slash := filepath.ToSlash(path)
	testPathMu.RLock()
	defer testPathMu.RUnlock()
	for _, marker := range testPaths {
		if marker != "" && strings.Contains(slash, marker) {
			return true
		}
	}
	if testPathsOverride {
		return false
	}
	return isGradleTestSourceSet(slash)
}

func isGradleTestSourceSet(slash string) bool {
	const sourceSetPrefix = "/src/"
	if strings.HasPrefix(slash, "src/") {
		segment := slash[len("src/"):]
		if end := strings.IndexByte(segment, '/'); end >= 0 {
			segment = segment[:end]
		}
		if isTestSourceSetName(segment) {
			return true
		}
	}
	for offset := 0; ; {
		index := strings.Index(slash[offset:], sourceSetPrefix)
		if index < 0 {
			return false
		}
		start := offset + index + len(sourceSetPrefix)
		end := strings.IndexByte(slash[start:], '/')
		if end < 0 {
			end = len(slash)
		} else {
			end += start
		}
		segment := slash[start:end]
		if isTestSourceSetName(segment) {
			return true
		}
		offset = start
	}
}

func defaultTestPathSlice() []string {
	out := make([]string, len(defaultTestPaths))
	copy(out, defaultTestPaths[:])
	return out
}

func cleanTestPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out
}

// IsTestSourceSetName reports whether a Gradle source set name, such as testDebug or androidTest, holds tests.
func IsTestSourceSetName(name string) bool { return isTestSourceSetName(name) }

// isTestSourceSetName reports whether a Gradle source set directory holds tests: test, androidTest, a
// multiplatform *Test set, or a variant of the Android sets such as testDebug or androidTestDebug.
func isTestSourceSetName(segment string) bool {
	if segment == "test" || strings.HasSuffix(segment, "Test") {
		return true
	}
	for _, prefix := range [...]string{"test", "androidTest"} {
		if rest := strings.TrimPrefix(segment, prefix); rest != segment && rest != "" && rest[0] >= 'A' && rest[0] <= 'Z' {
			return true
		}
	}
	return false
}
