package android

import (
	"os"
	"path/filepath"
	"strings"
)

// AndroidProject holds the paths to Android project files discovered during scanning.
type AndroidProject struct {
	ManifestPaths []string // paths to AndroidManifest.xml files
	ResDirs       []string // paths to res/ directories
	GradlePaths   []string // paths to build.gradle or build.gradle.kts files
}

// IsEmpty returns true if no Android project files were found.
func (p *AndroidProject) IsEmpty() bool {
	return len(p.ManifestPaths) == 0 && len(p.ResDirs) == 0 && len(p.GradlePaths) == 0
}

// DetectAndroidProject walks the given scan paths looking for Android project
// files: AndroidManifest.xml, src/main/res/, build.gradle, build.gradle.kts.
// It returns an AndroidProject with all discovered paths.
func DetectAndroidProject(scanPaths []string) *AndroidProject {
	proj := &AndroidProject{}
	seen := make(map[string]bool)

	for _, root := range scanPaths {
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			// Single file: check if it's one of our target files
			base := filepath.Base(root)
			switch base {
			case "AndroidManifest.xml":
				if !seen[root] {
					seen[root] = true
					proj.ManifestPaths = append(proj.ManifestPaths, root)
				}
			case "build.gradle", "build.gradle.kts":
				if !seen[root] {
					seen[root] = true
					proj.GradlePaths = append(proj.GradlePaths, root)
				}
			}
			continue
		}

		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			// Skip hidden directories and common non-project dirs
			if d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".") || name == "node_modules" || name == "build" || name == "__pycache__" {
					return filepath.SkipDir
				}
				// Check for res/ directory under src/main/
				if name == "res" && strings.HasSuffix(filepath.Dir(path), filepath.Join("src", "main")) {
					abs, err := filepath.Abs(path)
					if err != nil {
						abs = path
					}
					if !seen[abs] {
						seen[abs] = true
						proj.ResDirs = append(proj.ResDirs, abs)
					}
				}
				return nil
			}

			base := d.Name()
			switch base {
			case "AndroidManifest.xml":
				abs, err := filepath.Abs(path)
				if err != nil {
					abs = path
				}
				if !seen[abs] {
					seen[abs] = true
					proj.ManifestPaths = append(proj.ManifestPaths, abs)
				}
			case "build.gradle", "build.gradle.kts":
				abs, err := filepath.Abs(path)
				if err != nil {
					abs = path
				}
				if !seen[abs] {
					seen[abs] = true
					proj.GradlePaths = append(proj.GradlePaths, abs)
				}
			}
			return nil
		})
	}

	return proj
}
