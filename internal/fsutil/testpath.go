package fsutil

import (
	"path/filepath"
	"strings"
)

var basicTestMarkers = [...]string{"/test/", "/androidTest/", "/commonTest/", "/jvmTest/", "/testFixtures/"}

// IsBasicTestPath reports whether path has a common test source-set directory segment.
// It is a fixed, config-independent check; use scanner.IsTestFile for the configurable one.
func IsBasicTestPath(path string) bool {
	slash := filepath.ToSlash(path)
	for _, marker := range basicTestMarkers {
		if strings.Contains(slash, marker) {
			return true
		}
	}
	return false
}
