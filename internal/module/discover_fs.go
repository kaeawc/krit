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
	// walk visits the subdirectories of dir, whose listing is entries. Each
	// child is listed once, and the listing answers both whether it holds a
	// settings file or build script and what to descend into.
	var walk func(dir string, entries []os.DirEntry, depth int)
	walk = func(dir string, entries []os.DirEntry, depth int) {
		if depth > maxBuildScanDepth {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || skipBuildScanDir(e.Name()) {
				continue
			}
			child := filepath.Join(dir, e.Name())
			childEntries, err := os.ReadDir(child)
			settings, build := false, false
			if err != nil {
				// An unlistable directory can still be statted into.
				settings, build = hasSettingsFile(child), hasBuildScript(child)
			} else {
				settings = listsAny(childEntries, settingsFiles)
				build = listsAny(childEntries, buildFiles)
			}
			if settings {
				scan.nestedBuilds = append(scan.nestedBuilds, child)
				continue
			}
			if build {
				scan.buildDirs = append(scan.buildDirs, child)
			}
			walk(child, childEntries, depth+1)
		}
	}
	if entries, err := os.ReadDir(root); err == nil {
		walk(root, entries, 1)
	}
	sort.Strings(scan.buildDirs)
	sort.Strings(scan.nestedBuilds)
	return scan
}

// listsAny reports whether a directory listing holds an entry with one of
// the names, of any type, as os.Stat on the name would.
func listsAny(entries []os.DirEntry, names []string) bool {
	for _, e := range entries {
		for _, name := range names {
			if e.Name() == name {
				return true
			}
		}
	}
	return false
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
