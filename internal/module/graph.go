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
}

// Dependency represents a module-to-module dependency.
type Dependency struct {
	ModulePath    string // Gradle path of depended-on module
	Configuration string // "api", "implementation", "compileOnly", etc.
}

// ModuleGraph holds all discovered modules and their relationships.
type ModuleGraph struct {
	Modules   map[string]*Module  // path -> module
	Consumers map[string][]string // module path -> list of modules that depend on it
	RootDir   string
}

// NewModuleGraph creates an empty ModuleGraph rooted at the given directory.
func NewModuleGraph(rootDir string) *ModuleGraph {
	return &ModuleGraph{
		Modules:   make(map[string]*Module),
		Consumers: make(map[string][]string),
		RootDir:   rootDir,
	}
}

// FileToModule maps an absolute file path to the Gradle path of the module
// that contains it. Returns an empty string if no module matches.
func (g *ModuleGraph) FileToModule(filePath string) string {
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
