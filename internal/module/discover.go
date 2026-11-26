package module

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kaeawc/krit/internal/bazel"
	"github.com/kaeawc/krit/internal/lazyre"
)

// settingsFiles lists the settings file names in priority order.
var settingsFiles = []string{"settings.gradle.kts", "settings.gradle"}

// DiscoverModules discovers the Gradle modules of the project rooted at
// rootDir. Modules declared in the settings file come first; directories
// with a build script that the settings parser could not see (custom
// settings DSLs, helper functions, ValueSources) are added from the
// filesystem. A root without a settings file that holds several
// independent builds gets each build's modules under its directory path.
// Returns nil when rootDir has no settings file and no build scripts.
// A rootDir that names a file (a scan path such as src/A.kt) is
// discovered from BuildRoot of the file's directory.
func DiscoverModules(ctx context.Context, rootDir string) (*Graph, error) {
	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(rootDir); err == nil && !info.IsDir() {
		rootDir = BuildRoot(filepath.Dir(rootDir))
	}
	graph := NewModuleGraph(rootDir)
	found, err := discoverBuild(ctx, graph, rootDir, "")
	if err != nil {
		return nil, err
	}
	if !found {
		return discoverBazelModules(rootDir), nil
	}
	return graph, nil
}

// discoverBazelModules builds a graph with one module per Bazel package
// that holds JVM or Android targets ("//app/src/main"), with edges from the
// packages its targets depend on. Returns nil without such targets.
func discoverBazelModules(rootDir string) *Graph {
	ws := bazel.Discover(rootDir)
	if len(ws.Targets) == 0 {
		return nil
	}
	graph := NewModuleGraph(rootDir)
	graph.BuildSystem = BuildSystemBazel
	seen := map[string]bool{}
	for _, t := range ws.Targets {
		path := "//" + t.Package
		m := graph.Modules[path]
		if m == nil {
			dir := filepath.Join(rootDir, filepath.FromSlash(t.Package))
			m = &Module{Path: path, Dir: dir, SourceRoots: []string{dir}}
			graph.Modules[path] = m
		}
		for _, dep := range t.Deps {
			depPath := "//" + dep
			key := fmt.Sprintf("%s|%s|%t", path, depPath, t.Test)
			if seen[key] {
				continue
			}
			seen[key] = true
			m.Dependencies = append(m.Dependencies, Dependency{ModulePath: depPath, Configuration: t.Kind, Test: t.Test})
			graph.Consumers[depPath] = append(graph.Consumers[depPath], path)
		}
	}
	return graph
}

// discoverBuild adds the modules of the Gradle build at buildDir to graph,
// prefixing their paths with prefix. It reports whether buildDir holds a
// settings file or any build script.
func discoverBuild(ctx context.Context, graph *Graph, buildDir, prefix string) (bool, error) {
	settingsPath, content, err := readSettingsFile(buildDir)
	if err != nil {
		return false, err
	}

	var modulePaths []string
	var dirOverrides map[string]string
	switch {
	case content == "":
	case strings.HasSuffix(settingsPath, ".kts"):
		// Kotlin DSL — tree-sitter parse handles both static include()
		// calls and dynamic dir.listFiles().forEach { include(...) }
		// idioms. See discover_kts.go.
		parsed := parseSettingsKts(ctx, buildDir, content)
		modulePaths = parsed.paths
		dirOverrides = parsed.overrides
	default:
		// Groovy DSL — regex-based parser. We don't attempt to expand
		// dynamic includes for Groovy.
		modulePaths = parseIncludes(content)
		dirOverrides = parseProjectDirOverrides(content)
	}

	declared := make([]*Module, 0, len(modulePaths))
	for _, modPath := range modulePaths {
		m := &Module{
			Path:      prefix + modPath,
			Dir:       modulePathToDir(buildDir, modPath, dirOverrides),
			BuildPath: prefix,
		}
		declared = append(declared, m)
		graph.Modules[m.Path] = m
	}

	scan := scanBuildTree(buildDir)
	claimed := make(map[string]bool, len(declared))
	for _, m := range declared {
		claimed[m.Dir] = true
	}
	relocateDeclaredModules(declared, scan.buildDirs, claimed)
	for _, dir := range scan.buildDirs {
		if claimed[dir] {
			continue
		}
		rel, err := filepath.Rel(buildDir, dir)
		if err != nil {
			continue
		}
		path := prefix + ":" + strings.ReplaceAll(filepath.ToSlash(rel), "/", ":")
		if _, exists := graph.Modules[path]; exists {
			continue
		}
		graph.Modules[path] = &Module{Path: path, Dir: dir, BuildPath: prefix}
	}
	for _, m := range graph.Modules {
		if m.BuildPath == prefix && m.SourceRoots == nil {
			m.SourceRoots = findSourceRoots(m.Dir)
		}
	}

	// Included builds (build-logic, a nested settings file) are
	// independent graphs of the build that includes them. Only a root
	// without settings — a repository of separate samples — gets the
	// nested builds' modules.
	found := settingsPath != "" || len(scan.buildDirs) > 0
	if settingsPath == "" {
		for _, nested := range scan.nestedBuilds {
			rel, err := filepath.Rel(buildDir, nested)
			if err != nil {
				continue
			}
			nestedPrefix := prefix + ":" + strings.ReplaceAll(filepath.ToSlash(rel), "/", ":")
			ok, err := discoverBuild(ctx, graph, nested, nestedPrefix)
			if err != nil {
				return false, err
			}
			found = found || ok
		}
	}
	return found, nil
}

// relocateDeclaredModules points a declared module whose directory has no
// build script at the one unclaimed build directory with the same name.
// Settings helpers such as `fun module(path) { include(name);
// project(":$name").projectDir = file(path) }` declare the name but hide
// the directory.
func relocateDeclaredModules(declared []*Module, buildDirs []string, claimed map[string]bool) {
	byName := make(map[string][]string)
	for _, dir := range buildDirs {
		if !claimed[dir] {
			byName[filepath.Base(dir)] = append(byName[filepath.Base(dir)], dir)
		}
	}
	for _, m := range declared {
		if hasBuildScript(m.Dir) {
			continue
		}
		name := m.Path[strings.LastIndex(m.Path, ":")+1:]
		if candidates := byName[name]; len(candidates) == 1 && !claimed[candidates[0]] {
			delete(claimed, m.Dir)
			m.Dir = candidates[0]
			claimed[m.Dir] = true
		}
	}
}

// BuildRoot returns the nearest directory at or above dir that holds a
// Gradle settings file or a Bazel workspace file (MODULE.bazel, WORKSPACE,
// ...), without going above a repository root (a directory with .git).
// Without one it returns dir itself, so a file outside any build is
// discovered from its own directory rather than from a large ancestor such
// as $HOME, whose whole tree discovery would walk.
func BuildRoot(dir string) string {
	for d := dir; ; {
		if isBuildRoot(d) {
			return d
		}
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(d)
		if parent == d {
			return dir
		}
		d = parent
	}
}

func isBuildRoot(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && (bazel.IsWorkspaceFile(e.Name()) || slices.Contains(settingsFiles, e.Name())) {
			return true
		}
	}
	return false
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
var includeKtsRe = lazyre.New(`(?s)include\s*\(([^)]+)\)`)

// includeGroovyRe matches Groovy include calls: include ':app', ':lib'
var includeGroovyRe = lazyre.New(`(?m)^[[:blank:]]*include[[:blank:]]+((?:['":][^\n]+))`)

// quotedStringRe extracts quoted strings.
var quotedStringRe = lazyre.New(`["']([^"']+)["']`)

// groovyColonRe extracts Groovy-style unquoted colon-prefixed module paths.
var groovyColonRe = lazyre.New(`:[\w\-:]+`)

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
var projectDirRe = lazyre.New(`project\(["']([^"']+)["']\)\.projectDir\s*=\s*file\(["']([^"']+)["']\)`)

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
// src/main/kotlin, src/main/java, etc. Without any, it falls back to the
// flat multiplatform layout kotlinx.coroutines and ktor use, where each
// target directory holds src/ and test/ directly (common/src, jvm/test).
func findSourceRoots(dir string) []string {
	var roots []string
	srcDir := filepath.Join(dir, "src")
	entries, err := os.ReadDir(srcDir)
	if err == nil {
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
	}
	if len(roots) > 0 {
		return roots
	}
	return findFlatTargetSourceRoots(dir)
}

func findFlatTargetSourceRoots(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var roots []string
	for _, entry := range entries {
		if !entry.IsDir() || skipBuildScanDir(entry.Name()) {
			continue
		}
		target := filepath.Join(dir, entry.Name())
		if hasBuildScript(target) {
			continue
		}
		for _, kind := range []string{"src", "test"} {
			candidate := filepath.Join(target, kind)
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
