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
func DiscoverModules(rootDir string) (*Graph, error) {
	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}

	settingsPath, content, err := readSettingsFile(rootDir)
	if err != nil {
		return nil, err
	}
	if content == "" {
		return nil, nil
	}

	graph := NewModuleGraph(rootDir)

	var modulePaths []string
	var dirOverrides map[string]string

	if strings.HasSuffix(settingsPath, ".kts") {
		// Kotlin DSL — tree-sitter parse handles both static include()
		// calls and dynamic dir.listFiles().forEach { include(...) }
		// idioms. See discover_kts.go.
		parsed := parseSettingsKts(rootDir, content)
		modulePaths = parsed.paths
		dirOverrides = parsed.overrides
	} else {
		// Groovy DSL — regex-based parser. We don't attempt to expand
		// dynamic includes for Groovy.
		modulePaths = parseIncludes(content)
		dirOverrides = parseProjectDirOverrides(content)
	}

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

// readSettingsFile finds and reads the settings file content. Returns the
// path that was read along with its content; empty path/content if no
// settings file exists.
func readSettingsFile(rootDir string) (string, string, error) {
	for _, name := range settingsFiles {
		path := filepath.Join(rootDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", "", err
		}
		return path, string(data), nil
	}
	return "", "", nil
}

// includeKtsRe matches Kotlin DSL include() calls: include(":app", ":lib")
// It captures the full argument list inside the parentheses. Used by the
// Groovy fallback path; Kotlin DSL goes through parseSettingsKts.
var includeKtsRe = regexp.MustCompile(`(?s)include\s*\(([^)]+)\)`)

// includeGroovyRe matches Groovy include calls: include ':app', ':lib'
var includeGroovyRe = regexp.MustCompile(`(?m)^[[:blank:]]*include[[:blank:]]+((?:['":][^\n]+))`)

// quotedStringRe extracts quoted strings.
var quotedStringRe = regexp.MustCompile(`["']([^"']+)["']`)

// groovyColonRe extracts Groovy-style unquoted colon-prefixed module paths.
var groovyColonRe = regexp.MustCompile(`:[\w\-:]+`)

// parseIncludes extracts all module paths from include() calls. This is
// the Groovy-DSL path; Kotlin DSL settings go through parseSettingsKts.
// Templated paths containing `${...}` are skipped — Groovy dynamic
// expansion is not implemented and emitting the literal would create a
// phantom module.
func parseIncludes(content string) []string {
	seen := make(map[string]bool)
	var result []string

	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if strings.Contains(path, "${") {
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

	// Kotlin DSL syntax shape (still useful as a fallback if anyone calls
	// parseIncludes on .kts content directly): include(":app", ":lib")
	for _, match := range includeKtsRe.FindAllStringSubmatch(content, -1) {
		args := match[1]
		for _, qm := range quotedStringRe.FindAllStringSubmatch(args, -1) {
			add(qm[1])
		}
	}

	// Groovy DSL: include ':app', ':lib'
	for _, match := range includeGroovyRe.FindAllStringSubmatch(content, -1) {
		line := match[1]
		quoted := quotedStringRe.FindAllStringSubmatch(line, -1)
		if len(quoted) > 0 {
			for _, qm := range quoted {
				add(qm[1])
			}
		} else {
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

// hasBuildScript reports whether dir contains a Gradle build script.
// Used by the .kts dynamic-include expansion to confirm a candidate
// subdirectory is actually a Gradle module.
func hasBuildScript(dir string) bool {
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}
