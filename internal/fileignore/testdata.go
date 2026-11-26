package fileignore

import (
	"path/filepath"
	"strings"
)

// testDataDirMarkers name directories of deliberately malformed Kotlin and
// Java used to exercise compiler and IDE behavior. They are not user code
// and are skipped by directory scans.
var testDataDirMarkers = [...]string{
	"/test/data/", "/testData/", "/testdata/", "/test-data/",
	"/compiler-tests/", "/compilerTests/",
}

// TestDataPathUnder reports whether path, found while walking root, lies in
// a built-in test-data or test-resources directory. Only the part of path
// below root is checked, so a project checked out under a directory such as
// /home/runner/work/testdata/ is still scanned (#1549). A path outside root
// is checked in full.
func TestDataPathUnder(root, path string) bool {
	rel := path
	if root != "" {
		if r, err := filepath.Rel(root, path); err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			rel = r
		}
	}
	return TestDataPath(rel)
}

// TestDataPath reports whether a root-relative path lies in a built-in
// test-data directory or in the resources directory of a test source set.
// Callers must pass a path relative to the scan root; an absolute path
// would also match directories above the project.
func TestDataPath(relative string) bool {
	slash := filepath.ToSlash(relative)
	// filepath.ToSlash only replaces the current platform's separator. Also
	// normalize Windows paths when running on Unix so path checks are stable.
	slash = "/" + strings.ReplaceAll(slash, `\`, "/")
	for _, marker := range testDataDirMarkers {
		if strings.Contains(slash, marker) {
			return true
		}
	}
	return testResourcePath(slash)
}

// testResourcePath reports whether a slash path is inside the resources
// directory of a test source set (src/test/resources, src/commonTest/resources, ...).
func testResourcePath(slash string) bool {
	segments := strings.Split(slash, "/")
	for i := 0; i+3 < len(segments); i++ {
		if segments[i] != "src" || segments[i+2] != "resources" {
			continue
		}
		set := segments[i+1]
		if set == "test" || set == "androidTest" || set == "testFixtures" || strings.HasSuffix(set, "Test") {
			return true
		}
	}
	return false
}
