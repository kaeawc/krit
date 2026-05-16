package lsp

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/config"
)

func lspClasspath(root string, cfg *config.Config, initClasspath []string) []string {
	var out []string
	out = append(out, initClasspath...)
	if cfg != nil {
		out = append(out, cfg.LSP().Classpath...)
		out = append(out, cfg.Oracle().Classpath...)
	}
	if env := os.Getenv("CLASSPATH"); env != "" {
		out = append(out, filepath.SplitList(env)...)
	}
	out = append(out, discoverGradleClasspath(root)...)
	return existingUniquePaths(out)
}

func discoverGradleClasspath(root string) []string {
	if root == "" {
		return nil
	}
	var coords []android.Dependency
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && shouldSkipClasspathDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if name != "build.gradle" && name != "build.gradle.kts" {
			return nil
		}
		cfg, err := android.ParseBuildGradle(path)
		if err == nil && cfg != nil {
			coords = append(coords, cfg.Dependencies...)
		}
		return nil
	})
	var out []string
	for _, dep := range coords {
		out = append(out, gradleCacheJARs(dep)...)
	}
	out = append(out, kotlinStdlibJARs()...)
	return out
}

func shouldSkipClasspathDir(name string) bool {
	switch name {
	case ".git", ".gradle", "build", ".idea", "node_modules":
		return true
	default:
		return false
	}
}

func gradleCacheJARs(dep android.Dependency) []string {
	if dep.Group == "" || dep.Name == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	version := dep.Version
	if version == "" {
		version = "*"
	}
	pattern := filepath.Join(home, ".gradle", "caches", "modules-2", "files-2.1", dep.Group, dep.Name, version, "*", dep.Name+"-*.jar")
	matches, _ := filepath.Glob(pattern)
	sort.Strings(matches)
	return matches
}

func kotlinStdlibJARs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	pattern := filepath.Join(home, ".gradle", "caches", "modules-2", "files-2.1", "org.jetbrains.kotlin", "kotlin-stdlib", "*", "*", "kotlin-stdlib-*.jar")
	matches, _ := filepath.Glob(pattern)
	sort.Strings(matches)
	return matches
}

func existingUniquePaths(paths []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		for _, part := range strings.Split(p, string(os.PathListSeparator)) {
			part = strings.TrimSpace(part)
			if part == "" || seen[part] {
				continue
			}
			if _, err := os.Stat(part); err != nil {
				continue
			}
			seen[part] = true
			out = append(out, part)
		}
	}
	sort.Strings(out)
	return out
}
