package android

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/trackedfiles"
)

// Project holds the paths to Android project files discovered during scanning.
type Project struct {
	ManifestPaths []string // paths to AndroidManifest.xml files
	ResDirs       []string // paths to res/ directories
	GradlePaths   []string // paths to build.gradle or build.gradle.kts files
}

// IsEmpty returns true if no Android project files were found.
func (p *Project) IsEmpty() bool {
	return len(p.ManifestPaths) == 0 && len(p.ResDirs) == 0 && len(p.GradlePaths) == 0
}

// DetectProject walks the given scan paths looking for Android project
// files: AndroidManifest.xml, src/main/res/, build.gradle, build.gradle.kts.
// It returns an Project with all discovered paths.
func DetectProject(scanPaths []string) *Project {
	return DetectProjectWithIndex(scanPaths, trackedfiles.NewGitIndex())
}

// DetectProjectWithIndex is DetectProject with injectable tracked-file
// discovery for tests and callers that already have a shared listing.
func DetectProjectWithIndex(scanPaths []string, index trackedfiles.Index) *Project {
	proj := &Project{}
	seen := make(map[string]bool)

	for _, root := range scanPaths {
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			detectSingleFile(root, proj, seen)
			continue
		}
		if detectProjectWithIndex(root, index, proj, seen) {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			return detectWalkEntry(path, d, err, proj, seen)
		})
	}

	return proj
}

func detectProjectWithIndex(root string, index trackedfiles.Index, proj *Project, seen map[string]bool) bool {
	if index == nil {
		return false
	}
	files, ok := index.Files(root)
	if !ok {
		return false
	}
	for _, rel := range files {
		if rel == "" {
			continue
		}
		if !isAndroidProjectRelPath(rel) {
			continue
		}
		path := filepath.Join(root, rel)
		detectSingleFile(path, proj, seen)
		if resDir := srcMainResDir(path); resDir != "" && !seen[resDir] {
			seen[resDir] = true
			proj.ResDirs = append(proj.ResDirs, resDir)
		}
	}
	return true
}

func isAndroidProjectRelPath(path string) bool {
	base := filepath.Base(path)
	return base == "AndroidManifest.xml" ||
		base == "build.gradle" ||
		base == "build.gradle.kts" ||
		srcMainResDir(path) != ""
}

func srcMainResDir(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "src" && parts[i+1] == "main" && parts[i+2] == "res" {
			return filepath.FromSlash(strings.Join(parts[:i+3], "/"))
		}
	}
	return ""
}

func detectSingleFile(path string, proj *Project, seen map[string]bool) {
	base := filepath.Base(path)
	switch base {
	case "AndroidManifest.xml":
		if !seen[path] {
			seen[path] = true
			proj.ManifestPaths = append(proj.ManifestPaths, path)
		}
	case "build.gradle", "build.gradle.kts":
		if !seen[path] {
			seen[path] = true
			proj.GradlePaths = append(proj.GradlePaths, path)
		}
	}
}

func detectWalkEntry(path string, d os.DirEntry, err error, proj *Project, seen map[string]bool) error {
	if err != nil {
		return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
	}
	if d.IsDir() {
		return detectWalkDir(path, d, proj, seen)
	}
	detectWalkFile(path, d, proj, seen)
	return nil
}

func detectWalkDir(path string, d os.DirEntry, proj *Project, seen map[string]bool) error {
	name := d.Name()
	if strings.HasPrefix(name, ".") || name == "node_modules" || name == "build" || name == "__pycache__" {
		return filepath.SkipDir
	}
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

func detectWalkFile(path string, d os.DirEntry, proj *Project, seen map[string]bool) {
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
}
