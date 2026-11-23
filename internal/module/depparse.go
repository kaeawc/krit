package module

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kaeawc/krit/internal/lazyre"
)

// buildFiles lists build file names in priority order.
var buildFiles = []string{"build.gradle.kts", "build.gradle"}

// ParseDependencies parses the build.gradle.kts (or build.gradle) file for a
// module and populates its Dependencies and IsPublished fields. It also
// updates the graph's Consumers reverse-index.
func ParseDependencies(graph *Graph, module *Module) error {
	content, err := readBuildFile(module.Dir)
	if err != nil {
		return err
	}
	if content == "" {
		return nil
	}

	module.IsPublished = detectMavenPublish(content)
	module.Dependencies = parseDeps(content, graph, module.BuildPath)

	// Update the consumers reverse-index.
	for _, dep := range module.Dependencies {
		graph.Consumers[dep.ModulePath] = append(graph.Consumers[dep.ModulePath], module.Path)
	}

	return nil
}

// ParseAllDependencies parses dependencies for every module in the graph, in
// module-path order so Consumers is deterministic. A module whose build file
// cannot be read is skipped and reported in the joined error; the rest still parse.
func ParseAllDependencies(graph *Graph) error {
	var errs []error
	for _, path := range slices.Sorted(maps.Keys(graph.Modules)) {
		if err := ParseDependencies(graph, graph.Modules[path]); err != nil {
			errs = append(errs, fmt.Errorf("module %s: %w", path, err))
		}
	}
	return errors.Join(errs...)
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
//
//	implementation(project(":core-util"))
//	api(project(":lib"))
//	testImplementation(project(":test-utils"))
var projectDepRe = lazyre.New(`(\w+)\s*\(\s*(?:testFixtures\s*\(\s*)?project\(["']([^"']+)["']\)\s*\)?\s*\)`)

// flavorDepRe matches flavor-qualified dependencies:
//
//	"playImplementation"(project(":billing"))
var flavorDepRe = lazyre.New(`"(\w+)"\s*\(\s*project\(["']([^"']+)["']\)\s*\)`)

// typesafeDepRe matches typesafe project accessor dependencies:
//
//	api(projects.circuitRuntime)
//	implementation(projects.sentryAndroidCore)
//
// It handles dotted paths like projects.samples.star.benchmark
var typesafeDepRe = lazyre.New(`(\w+)\s*\(\s*projects\.([a-zA-Z][\w.]*\w)\s*\)`)

// mavenPublishRe detects the maven-publish plugin.
var mavenPublishRe = lazyre.New(`(?:id\s*\(\s*["']maven-publish["']\s*\)|alias\s*\(\s*libs\.plugins\.mavenPublish\s*\)|apply\s+plugin:\s*["']maven-publish["'])`)

// parseDeps extracts all project dependencies from build file content.
// buildPath is the owning build's path prefix (see Module.BuildPath).
func parseDeps(content string, graph *Graph, buildPath string) []Dependency {
	seen := make(map[string]bool)
	var deps []Dependency
	testBlocks := testSourceSetBlocks(content)

	add := func(config, modPath string, offset int) {
		modPath = strings.TrimSpace(modPath)
		if !strings.HasPrefix(modPath, ":") {
			modPath = ":" + modPath
		}
		modPath = graph.resolveModulePath(modPath, buildPath)
		test := isTestConfiguration(config) || inBlocks(testBlocks, offset)
		key := fmt.Sprintf("%s|%s|%t", config, modPath, test)
		if seen[key] {
			return
		}
		seen[key] = true
		deps = append(deps, Dependency{
			ModulePath:    modPath,
			Configuration: config,
			Test:          test,
		})
	}

	// Standard project() deps.
	for _, m := range projectDepRe.FindAllStringSubmatchIndex(content, -1) {
		add(content[m[2]:m[3]], content[m[4]:m[5]], m[0])
	}

	// Flavor-qualified deps.
	for _, m := range flavorDepRe.FindAllStringSubmatchIndex(content, -1) {
		add(content[m[2]:m[3]], content[m[4]:m[5]], m[0])
	}

	// Typesafe accessor deps.
	for _, m := range typesafeDepRe.FindAllStringSubmatchIndex(content, -1) {
		add(content[m[2]:m[3]], typesafeAccessorToPath(content[m[4]:m[5]]), m[0])
	}

	return deps
}

// isTestConfiguration reports a Gradle configuration that only test code
// compiles or runs against: testImplementation, androidTestApi,
// testFixturesImplementation, kspTest, commonTestImplementation.
func isTestConfiguration(config string) bool {
	return strings.Contains(strings.ToLower(config), "test")
}

// testBlockHeaderRe matches a block header that names a test source set:
// `commonTest.dependencies {`, `val jvmTest by getting {`,
// `getByName("androidUnitTest") {`, `sourceSets.test {`.
var testBlockHeaderRe = lazyre.New(`(?:^|[^A-Za-z0-9_])(?:tests?|testFixtures|[a-z][A-Za-z0-9]*Tests?(?:Fixtures)?)(?:[^A-Za-z0-9_]|$)`)

type byteRange struct{ start, end int }

// testSourceSetBlocks returns the brace-delimited blocks of a build script
// whose header names a test source set. Braces inside strings and comments
// are not distinguished; a mismatched brace only drops the classification.
func testSourceSetBlocks(content string) []byteRange {
	var blocks []byteRange
	var stack []int
	headerStart := 0
	for i := 0; i < len(content); i++ {
		switch content[i] {
		case '\n', ';':
			headerStart = i + 1
		case '{':
			header := content[headerStart:i]
			if testBlockHeaderRe.MatchString(header) && !strings.Contains(header, "withType") {
				stack = append(stack, i)
			} else {
				stack = append(stack, -1)
			}
			headerStart = i + 1
		case '}':
			if n := len(stack); n > 0 {
				if open := stack[n-1]; open >= 0 {
					blocks = append(blocks, byteRange{open, i})
				}
				stack = stack[:n-1]
			}
			headerStart = i + 1
		}
	}
	return blocks
}

func inBlocks(blocks []byteRange, offset int) bool {
	for _, b := range blocks {
		if offset > b.start && offset < b.end {
			return true
		}
	}
	return false
}

// typesafeAccessorToPath converts a dotted typesafe accessor to a Gradle
// path by camelCase splitting; parseDeps resolves it against the graph.
func typesafeAccessorToPath(accessor string) string {
	// Split on dots for nested accessors like "samples.star.benchmark".
	parts := strings.Split(accessor, ".")
	var segments []string
	for _, part := range parts {
		// Convert camelCase to kebab-case.
		converted := AccessorToPath(part)
		// Strip the leading ":"
		segments = append(segments, strings.TrimPrefix(converted, ":"))
	}
	return ":" + strings.Join(segments, ":")
}

// detectMavenPublish returns true if the build file contains a maven-publish plugin.
func detectMavenPublish(content string) bool {
	return mavenPublishRe.MatchString(content)
}
