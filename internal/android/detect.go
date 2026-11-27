package android

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/bazel"
	"github.com/kaeawc/krit/internal/trackedfiles"
)

// Project holds the paths to Android project files discovered during scanning.
type Project struct {
	ManifestPaths []string // paths to AndroidManifest.xml files
	ResDirs       []string // paths to res/ directories
	GradlePaths   []string // paths to build.gradle or build.gradle.kts files
	// BazelPaths are Bazel BUILD files, workspace files (MODULE.bazel,
	// WORKSPACE) and rules_jvm_external lock files.
	BazelPaths []string
}

// LibraryInputPaths returns the files the library profile is built from:
// Gradle build and settings files plus Bazel workspace and lock files.
func (p *Project) LibraryInputPaths() []string {
	if p == nil {
		return nil
	}
	paths := append([]string(nil), p.GradlePaths...)
	for _, path := range p.BazelPaths {
		if base := filepath.Base(path); bazel.IsWorkspaceFile(base) || bazel.IsLockFile(base) {
			paths = append(paths, path)
		}
	}
	return paths
}

// IsEmpty returns true if no Android project files were found.
func (p *Project) IsEmpty() bool {
	return len(p.ManifestPaths) == 0 && len(p.ResDirs) == 0 && len(p.GradlePaths) == 0 && len(p.BazelPaths) == 0
}

// ShapeChangedBy reports whether a file created or removed at path changes
// what DetectProject finds: a manifest it does not list, a listed manifest
// that is gone, or a file in a src/main/res directory it does not list. exists
// says whether the file is there now. An editor's atomic save (rename, then
// create) of a file the project already accounts for changes nothing. A nil
// project reports true.
func (p *Project) ShapeChangedBy(path string, exists bool) bool {
	if p == nil {
		return true
	}
	path = filepath.Clean(path)
	if filepath.Base(path) == "AndroidManifest.xml" {
		listed := false
		for _, m := range p.ManifestPaths {
			if filepath.Clean(m) == path {
				listed = true
				break
			}
		}
		return listed != exists
	}
	resDir := srcMainResDir(path)
	if resDir == "" || !exists {
		return false
	}
	for _, dir := range p.ResDirs {
		if filepath.Clean(dir) == resDir {
			return false
		}
	}
	return true
}

// HasPathsUnder reports whether a manifest, res directory or build script the
// project lists lies in dir or below it, so that removing dir changes the
// project. A nil project reports true.
func (p *Project) HasPathsUnder(dir string) bool {
	if p == nil {
		return true
	}
	prefix := filepath.Clean(dir) + string(filepath.Separator)
	for _, list := range [][]string{p.ManifestPaths, p.ResDirs, p.GradlePaths, p.BazelPaths} {
		for _, path := range list {
			if strings.HasPrefix(filepath.Clean(path)+string(filepath.Separator), prefix) {
				return true
			}
		}
	}
	return false
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
		// Resolve to absolute up front: detectWalkDir SkipDirs any
		// directory whose Name() starts with "." (catches .git,
		// .gradle, etc.). When the caller passes "." as the root —
		// which the CLI does when run from the project's CWD with
		// no explicit path — WalkDir's first callback receives a
		// DirEntry whose Name() is "." and the SkipDir fires on the
		// root itself, leaving GradlePaths / ManifestPaths empty
		// and silently dropping every Gradle/Android rule. abs is
		// always a real basename so the hidden-dir guard targets
		// only the dotfiles we mean to skip.
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
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
	addBazelResourceDirs(proj, seen)

	return proj
}

// addBazelResourceDirs adds the directories that BUILD files name in
// resource_files globs. Bazel Android targets keep res/ wherever their
// BUILD file says (java/com/app/res, module/res), not at src/main/res.
func addBazelResourceDirs(proj *Project, seen map[string]bool) {
	for _, path := range proj.BazelPaths {
		if !bazel.IsBuildFile(filepath.Base(path)) {
			continue
		}
		for _, dir := range bazel.ResourceDirsFromBuildFile(path) {
			if !seen[dir] {
				seen[dir] = true
				proj.ResDirs = append(proj.ResDirs, dir)
			}
		}
	}
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
		base == "settings.gradle" ||
		base == "settings.gradle.kts" ||
		base == "init.gradle" ||
		base == "init.gradle.kts" ||
		isBazelProjectFile(base) ||
		srcMainResDir(path) != ""
}

func isBazelProjectFile(base string) bool {
	return bazel.IsBuildFile(base) || bazel.IsWorkspaceFile(base) || bazel.IsLockFile(base)
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
	case "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "init.gradle", "init.gradle.kts":
		if !seen[path] {
			seen[path] = true
			proj.GradlePaths = append(proj.GradlePaths, path)
		}
	default:
		if isBazelProjectFile(base) && !seen[path] {
			seen[path] = true
			proj.BazelPaths = append(proj.BazelPaths, path)
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
	case "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "init.gradle", "init.gradle.kts":
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		if !seen[abs] {
			seen[abs] = true
			proj.GradlePaths = append(proj.GradlePaths, abs)
		}
	default:
		if isBazelProjectFile(base) {
			abs, err := filepath.Abs(path)
			if err != nil {
				abs = path
			}
			if !seen[abs] {
				seen[abs] = true
				proj.BazelPaths = append(proj.BazelPaths, abs)
			}
		}
	}
}
