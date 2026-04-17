package module

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// buildFiles lists build file names in priority order.
var buildFiles = []string{"build.gradle.kts", "build.gradle"}

// ParseDependencies parses the build.gradle.kts (or build.gradle) file for a
// module and populates its Dependencies and IsPublished fields. It also
// updates the graph's Consumers reverse-index.
func ParseDependencies(graph *ModuleGraph, module *Module) error {
	content, err := readBuildFile(module.Dir)
	if err != nil {
		return err
	}
	if content == "" {
		return nil
	}

	module.IsPublished = detectMavenPublish(content)
	module.Dependencies = parseDeps(content, graph)

	// Update the consumers reverse-index.
	for _, dep := range module.Dependencies {
		graph.Consumers[dep.ModulePath] = append(graph.Consumers[dep.ModulePath], module.Path)
	}

	return nil
}

// ParseAllDependencies parses dependencies for every module in the graph.
func ParseAllDependencies(graph *ModuleGraph) error {
	for _, m := range graph.Modules {
		if err := ParseDependencies(graph, m); err != nil {
			return err
		}
	}
	return nil
}

// readBuildFile reads the build file from the given directory.
func readBuildFile(dir string) (string, error) {
	for _, name := range buildFiles {
		path := filepath.Join(dir, name)
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

// projectDepRe matches standard project dependencies:
//   implementation(project(":core-util"))
//   api(project(":lib"))
//   testImplementation(project(":test-utils"))
var projectDepRe = regexp.MustCompile(`(\w+)\s*\(\s*(?:testFixtures\s*\(\s*)?project\(["']([^"']+)["']\)\s*\)?\s*\)`)

// flavorDepRe matches flavor-qualified dependencies:
//   "playImplementation"(project(":billing"))
var flavorDepRe = regexp.MustCompile(`"(\w+)"\s*\(\s*project\(["']([^"']+)["']\)\s*\)`)

// typesafeDepRe matches typesafe project accessor dependencies:
//   api(projects.circuitRuntime)
//   implementation(projects.sentryAndroidCore)
// It handles dotted paths like projects.samples.star.benchmark
var typesafeDepRe = regexp.MustCompile(`(\w+)\s*\(\s*projects\.([a-zA-Z][\w.]*\w)\s*\)`)

// mavenPublishRe detects the maven-publish plugin.
var mavenPublishRe = regexp.MustCompile(`(?:id\s*\(\s*["']maven-publish["']\s*\)|alias\s*\(\s*libs\.plugins\.mavenPublish\s*\)|apply\s+plugin:\s*["']maven-publish["'])`)

// parseDeps extracts all project dependencies from build file content.
func parseDeps(content string, graph *ModuleGraph) []Dependency {
	seen := make(map[string]bool)
	var deps []Dependency

	add := func(config, modPath string) {
		modPath = strings.TrimSpace(modPath)
		if !strings.HasPrefix(modPath, ":") {
			modPath = ":" + modPath
		}
		key := config + "|" + modPath
		if seen[key] {
			return
		}
		seen[key] = true
		deps = append(deps, Dependency{
			ModulePath:    modPath,
			Configuration: config,
		})
	}

	// Standard project() deps.
	for _, match := range projectDepRe.FindAllStringSubmatch(content, -1) {
		add(match[1], match[2])
	}

	// Flavor-qualified deps.
	for _, match := range flavorDepRe.FindAllStringSubmatch(content, -1) {
		add(match[1], match[2])
	}

	// Typesafe accessor deps.
	for _, match := range typesafeDepRe.FindAllStringSubmatch(content, -1) {
		config := match[1]
		accessor := match[2]
		modPath := typesafeAccessorToPath(accessor, graph)
		add(config, modPath)
	}

	return deps
}

// typesafeAccessorToPath converts a dotted typesafe accessor to a Gradle path.
// It first tries to match against known modules in the graph, then falls back
// to camelCase splitting.
func typesafeAccessorToPath(accessor string, graph *ModuleGraph) string {
	// Split on dots for nested accessors like "samples.star.benchmark".
	parts := strings.Split(accessor, ".")
	var segments []string
	for _, part := range parts {
		// Convert camelCase to kebab-case.
		converted := AccessorToPath(part)
		// Strip the leading ":"
		segments = append(segments, strings.TrimPrefix(converted, ":"))
	}
	candidate := ":" + strings.Join(segments, ":")
	if graph != nil {
		if _, ok := graph.Modules[candidate]; ok {
			return candidate
		}
	}
	return candidate
}

// detectMavenPublish returns true if the build file contains a maven-publish plugin.
func detectMavenPublish(content string) bool {
	return mavenPublishRe.MatchString(content)
}
