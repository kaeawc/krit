package fileignore

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// buildFileNames mark a directory as a build module root; sample code beside one may be compiled.
var buildFileNames = [...]string{
	"build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts",
	"pom.xml", "BUILD", "BUILD.bazel", "module.yaml",
}

var (
	buildFileDirMu    sync.Mutex
	buildFileDirCache = map[string]bool{}
	gitRootCache      = map[string]string{}
)

// DocSamplePath reports whether an absolute source path is a documentation
// sample: it sits under an `examples` or `references` directory, outside any
// `src` tree, with no build file in any directory between it and the Git root.
// Such files are never compiled, so project rules would only see duplicate
// declarations. A path outside a Git checkout is never a sample.
func DocSamplePath(path string) bool {
	slash := "/" + filepath.ToSlash(path)
	if !strings.Contains(slash, "/examples/") && !strings.Contains(slash, "/references/") {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	dir := filepath.Dir(abs)
	root := cachedGitRoot(dir)
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return false
	}
	segments := strings.Split(filepath.ToSlash(rel), "/")
	sample := false
	for _, s := range segments {
		switch s {
		case "src":
			return false
		case "examples", "references":
			sample = true
		}
	}
	if !sample {
		return false
	}
	for d := dir; d != root && len(d) > len(root); d = filepath.Dir(d) {
		if dirHasBuildFile(d) {
			return false
		}
	}
	return true
}

func cachedGitRoot(dir string) string {
	buildFileDirMu.Lock()
	root, ok := gitRootCache[dir]
	buildFileDirMu.Unlock()
	if ok {
		return root
	}
	root = FindGitRoot(dir)
	buildFileDirMu.Lock()
	gitRootCache[dir] = root
	buildFileDirMu.Unlock()
	return root
}

func dirHasBuildFile(dir string) bool {
	buildFileDirMu.Lock()
	has, ok := buildFileDirCache[dir]
	buildFileDirMu.Unlock()
	if ok {
		return has
	}
	for _, name := range buildFileNames {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			has = true
			break
		}
	}
	buildFileDirMu.Lock()
	buildFileDirCache[dir] = has
	buildFileDirMu.Unlock()
	return has
}
