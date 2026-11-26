package module

import (
	"path/filepath"
	"strings"
)

// Module represents a single Gradle module in a multi-module project.
type Module struct {
	Path         string       // Gradle path, e.g. ":app", ":core:util"
	Dir          string       // absolute filesystem directory
	Dependencies []Dependency // module-to-module deps
	IsPublished  bool         // has maven-publish plugin
	SourceRoots  []string     // source directories
	// BuildPath is the path prefix of the Gradle build that owns the
	// module: empty for the root build, ":samples:app" style for a
	// separate build nested under a root without a settings file.
	BuildPath string
}

// Dependency represents a module-to-module dependency.
type Dependency struct {
	ModulePath    string // Gradle path of depended-on module
	Configuration string // "api", "implementation", "compileOnly", etc.
	// Test is set for a test configuration (testImplementation,
	// androidTestImplementation, testFixturesApi) or a dependency declared
	// in a test source set's block (commonTest.dependencies { ... }).
	Test bool
}

// BuildSystemBazel marks a Graph built from Bazel BUILD files.
const BuildSystemBazel = "bazel"

// Graph holds all discovered modules and their relationships.
type Graph struct {
	Modules   map[string]*Module  // path -> module
	Consumers map[string][]string // module path -> list of modules that depend on it
	RootDir   string
	// BuildSystem is "" for Gradle and BuildSystemBazel for a graph of
	// Bazel packages, whose module paths are labels ("//app/src/main").
	BuildSystem string
}

// NewModuleGraph creates an empty Graph rooted at the given directory.
func NewModuleGraph(rootDir string) *Graph {
	return &Graph{
		Modules:   make(map[string]*Module),
		Consumers: make(map[string][]string),
		RootDir:   rootDir,
	}
}

// resolveModulePath maps a project path referenced from a build file to a
// module in the graph. It tries the path inside the referencing build
// first (buildPath prefix), then the path as written, then the one module
// whose last segment matches a single-segment path — filesystem-discovered
// modules carry their directory path while a custom settings DSL may have
// registered them under the bare name. Unresolved paths are returned as
// written.
func (g *Graph) resolveModulePath(path, buildPath string) string {
	if g == nil {
		return path
	}
	if buildPath != "" {
		if _, ok := g.Modules[buildPath+path]; ok {
			return buildPath + path
		}
	}
	if _, ok := g.Modules[path]; ok {
		return path
	}
	if strings.Count(path, ":") != 1 {
		return path
	}
	match := ""
	for p, m := range g.Modules {
		if m.BuildPath == buildPath && strings.HasSuffix(p, path) {
			if match != "" {
				return path
			}
			match = p
		}
	}
	if match == "" {
		return path
	}
	return match
}

// resolveAccessorPath maps a type-safe project accessor ("featureNotes",
// "core.dataStore") to the one module of the build whose path segments
// spell it with dashes, underscores or camel case (":feature_notes",
// ":core:dataStore"). It returns "" when path, the accessor's dashed form,
// already names a module, or when no module or more than one matches.
func (g *Graph) resolveAccessorPath(accessor, path, buildPath string) string {
	if g == nil {
		return ""
	}
	if _, ok := g.Modules[buildPath+path]; ok {
		return ""
	}
	if _, ok := g.Modules[path]; ok {
		return ""
	}
	want := strings.Split(strings.ToLower(accessor), ".")
	match := ""
	for p, m := range g.Modules {
		if m.BuildPath != buildPath || !accessorSegmentsMatch(strings.TrimPrefix(p, buildPath), want) {
			continue
		}
		if match != "" {
			return ""
		}
		match = p
	}
	return match
}

// accessorSegmentsMatch reports whether a module path's segments, with
// dashes and underscores dropped and case folded, equal the accessor's.
func accessorSegmentsMatch(path string, want []string) bool {
	segments := strings.Split(strings.TrimPrefix(path, ":"), ":")
	if len(segments) != len(want) {
		return false
	}
	for i, segment := range segments {
		if strings.ToLower(accessorSpellingReplacer.Replace(segment)) != want[i] {
			return false
		}
	}
	return true
}

var accessorSpellingReplacer = strings.NewReplacer("-", "", "_", "")

// FileToModule maps an absolute file path to the Gradle path of the module
// that contains it. Returns an empty string if no module matches.
func (g *Graph) FileToModule(filePath string) string {
	clean := filepath.Clean(filePath)
	bestMatch := ""
	bestLen := 0
	for _, m := range g.Modules {
		dir := filepath.Clean(m.Dir)
		// Check that the file is under this module's directory.
		if strings.HasPrefix(clean, dir+string(filepath.Separator)) || clean == dir {
			if len(dir) > bestLen {
				bestLen = len(dir)
				bestMatch = m.Path
			}
		}
	}
	return bestMatch
}
