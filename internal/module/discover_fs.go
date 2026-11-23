package module

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// maxBuildScanDepth bounds the build-script scan below a build root.
const maxBuildScanDepth = 8

// buildTreeScan is what scanBuildTree finds below one Gradle build root.
type buildTreeScan struct {
	// buildDirs are directories below the root with a build script.
	buildDirs []string
	// nestedBuilds are directories below the root with their own
	// settings file; the scan does not descend into them.
	nestedBuilds []string
}

// scanBuildTree lists build-script directories below root. It skips
// hidden directories, build outputs, source sets (test fixtures there are
// projects under test, not modules), node_modules, and buildSrc.
func scanBuildTree(root string) buildTreeScan {
	var scan buildTreeScan
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if depth > maxBuildScanDepth {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || skipBuildScanDir(e.Name()) {
				continue
			}
			child := filepath.Join(dir, e.Name())
			if hasSettingsFile(child) {
				scan.nestedBuilds = append(scan.nestedBuilds, child)
				continue
			}
			if hasBuildScript(child) {
				scan.buildDirs = append(scan.buildDirs, child)
			}
			walk(child, depth+1)
		}
	}
	walk(root, 1)
	sort.Strings(scan.buildDirs)
	sort.Strings(scan.nestedBuilds)
	return scan
}

func skipBuildScanDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "build", "out", "src", "node_modules", "buildSrc":
		return true
	}
	return false
}

func hasSettingsFile(dir string) bool {
	for _, name := range settingsFiles {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}
