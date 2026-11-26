package fileignore

import (
	"path/filepath"
	"strings"
)

// testDataDirMarkers name directories of deliberately malformed Kotlin and
// Java used to exercise compiler and IDE behavior. They are not user code
// and are skipped by directory scans. Each marker is a run of path segments.
var testDataDirMarkers = [...][]string{
	{"test", "data"}, {"testData"}, {"testdata"}, {"test-data"},
	{"compiler-tests"}, {"compilerTests"},
}

// TestDataPathUnder reports whether path, found while walking root, lies in
// a built-in test-data or test-resources directory. The whole path is
// matched, so markers that span the root (src/test/resources scanned from
// src/) still count, but a hit needs the marker's last directory strictly
// below root: a project checked out under /home/runner/work/testdata/, or a
// scan of a testdata/ directory itself, is still analysed (#1549). A path
// outside root is checked in full.
func TestDataPathUnder(root, path string) bool {
	segments := slashSegments(path)
	below := 0
	if root != "" {
		if rel, err := filepath.Rel(root, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			below = len(segments) - len(slashSegments(rel))
		}
	}
	return testDataSegments(segments, below)
}

// TestDataPath reports whether path lies in a built-in test-data directory
// or in the resources directory of a test source set, matching anywhere in
// the path.
func TestDataPath(path string) bool {
	return testDataSegments(slashSegments(path), 0)
}

func slashSegments(path string) []string {
	slash := filepath.ToSlash(filepath.Clean(path))
	// filepath.ToSlash only replaces the current platform's separator. Also
	// normalize Windows paths when running on Unix so path checks are stable.
	slash = strings.ReplaceAll(slash, `\`, "/")
	if slash == "." {
		return nil
	}
	return strings.Split(slash, "/")
}

// testDataSegments reports whether segments hold a test-data marker or a
// test source set's resources directory whose last directory has an index
// of at least minDir. The final segment is the file name and never matches.
func testDataSegments(segments []string, minDir int) bool {
	dirs := len(segments) - 1
	for _, marker := range testDataDirMarkers {
		for i := 0; i+len(marker) <= dirs; i++ {
			if i+len(marker)-1 >= minDir && segmentsEqual(segments[i:i+len(marker)], marker) {
				return true
			}
		}
	}
	// src/<test set>/resources/...
	for i := 0; i+2 < dirs; i++ {
		if i+2 < minDir || segments[i] != "src" || segments[i+2] != "resources" {
			continue
		}
		set := segments[i+1]
		if set == "test" || set == "androidTest" || set == "testFixtures" || strings.HasSuffix(set, "Test") {
			return true
		}
	}
	return false
}

func segmentsEqual(a, b []string) bool {
	for i := range b {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
