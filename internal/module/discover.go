package module

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// settingsFiles lists the settings file names in priority order.
var settingsFiles = []string{"settings.gradle.kts", "settings.gradle"}

// DiscoverModules parses a Gradle settings file to discover all modules
// in the project rooted at rootDir. Returns nil if no settings file is found.
func DiscoverModules(rootDir string) (*ModuleGraph, error) {
	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}

	content, err := readSettingsFile(rootDir)
	if err != nil {
		return nil, err
	}
	if content == "" {
		return nil, nil
	}

	graph := NewModuleGraph(rootDir)

	// Parse include() calls.
	modulePaths := parseIncludes(content)

	// Parse projectDir overrides.
	dirOverrides := parseProjectDirOverrides(content)

	// Build modules.
	for _, modPath := range modulePaths {
		dir := modulePathToDir(rootDir, modPath, dirOverrides)
		m := &Module{
			Path: modPath,
			Dir:  dir,
		}
		m.SourceRoots = findSourceRoots(dir)
		graph.Modules[modPath] = m
	}

	return graph, nil
}

// readSettingsFile finds and reads the settings file content. Returns empty
// string if no settings file exists.
func readSettingsFile(rootDir string) (string, error) {
	for _, name := range settingsFiles {
		path := filepath.Join(rootDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		return string(data), nil
	}
	return "", nil
}

// includeKtsRe matches Kotlin DSL include() calls: include(":app", ":lib")
// It captures the full argument list inside the parentheses.
var includeKtsRe = regexp.MustCompile(`(?s)include\s*\(([^)]+)\)`)

// includeGroovyRe matches Groovy include calls: include ':app', ':lib'
var includeGroovyRe = regexp.MustCompile(`(?m)^[[:blank:]]*include[[:blank:]]+((?:['":][^\n]+))`)

// quotedStringRe extracts quoted strings.
var quotedStringRe = regexp.MustCompile(`["']([^"']+)["']`)

// groovyColonRe extracts Groovy-style unquoted colon-prefixed module paths.
var groovyColonRe = regexp.MustCompile(`:[\w\-:]+`)

// parseIncludes extracts all module paths from include() calls.
func parseIncludes(content string) []string {
	seen := make(map[string]bool)
	var result []string

	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if !strings.HasPrefix(path, ":") {
			path = ":" + path
		}
		if !seen[path] {
			seen[path] = true
			result = append(result, path)
		}
	}

	// Kotlin DSL: include(":app", ":lib")
	for _, match := range includeKtsRe.FindAllStringSubmatch(content, -1) {
		args := match[1]
		for _, qm := range quotedStringRe.FindAllStringSubmatch(args, -1) {
			add(qm[1])
		}
	}

	// Groovy DSL: include ':app', ':lib'
	for _, match := range includeGroovyRe.FindAllStringSubmatch(content, -1) {
		line := match[1]
		// Try quoted strings first.
		quoted := quotedStringRe.FindAllStringSubmatch(line, -1)
		if len(quoted) > 0 {
			for _, qm := range quoted {
				add(qm[1])
			}
		} else {
			// Fall back to colon-prefixed unquoted paths.
			for _, cm := range groovyColonRe.FindAllString(line, -1) {
				add(cm)
			}
		}
	}

	return result
}

// projectDirRe matches: project(":paging").projectDir = file("paging/lib")
var projectDirRe = regexp.MustCompile(`project\(["']([^"']+)["']\)\.projectDir\s*=\s*file\(["']([^"']+)["']\)`)

// parseProjectDirOverrides returns a map of module path -> relative directory.
func parseProjectDirOverrides(content string) map[string]string {
	overrides := make(map[string]string)
	for _, match := range projectDirRe.FindAllStringSubmatch(content, -1) {
		modPath := match[1]
		if !strings.HasPrefix(modPath, ":") {
			modPath = ":" + modPath
		}
		overrides[modPath] = match[2]
	}
	return overrides
}

// modulePathToDir converts a Gradle module path to its filesystem directory.
// Uses dirOverrides if present, otherwise converts `:core:util` to `<root>/core/util`.
func modulePathToDir(rootDir, modPath string, dirOverrides map[string]string) string {
	if override, ok := dirOverrides[modPath]; ok {
		return filepath.Join(rootDir, filepath.FromSlash(override))
	}
	// Strip leading colon and convert colons to path separators.
	rel := strings.TrimPrefix(modPath, ":")
	rel = strings.ReplaceAll(rel, ":", string(filepath.Separator))
	return filepath.Join(rootDir, rel)
}

// findSourceRoots scans a module directory for source roots like
// src/main/kotlin, src/main/java, etc.
func findSourceRoots(dir string) []string {
	var roots []string
	srcDir := filepath.Join(dir, "src")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		for _, lang := range []string{"kotlin", "java"} {
			candidate := filepath.Join(srcDir, entry.Name(), lang)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				roots = append(roots, candidate)
			}
		}
	}
	return roots
}
