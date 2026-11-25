package gradlemodel

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// ProjectOutputsSchema is the version of the file LoadProjectOutputs reads.
const ProjectOutputsSchema = 1

// LoadProjectOutputs reads the built outputs of Gradle project dependencies, as the
// Gradle plugin writes them for --gradle-project-outputs:
//
//	{"schema": 1, "projects": {":lib": ["/repo/lib/build/libs/lib.jar"]}}
//
// Each project maps to its jars. The Gradle plugin passes jars, not class directories:
// krit's caches key a classpath entry by its size and modification time, which a
// directory keeps when the classes in it are recompiled. A project listed here goes on
// the classpath through these outputs instead of being compiled from its sources.
func LoadProjectOutputs(path string) (map[string][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read project outputs %s: %w", path, err)
	}
	var file struct {
		Schema   int                 `json:"schema"`
		Projects map[string][]string `json:"projects"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("project outputs %s: malformed JSON: %w", path, err)
	}
	if file.Schema != ProjectOutputsSchema {
		return nil, fmt.Errorf("project outputs %s: unsupported schema %d", path, file.Schema)
	}
	if file.Projects == nil {
		file.Projects = map[string][]string{}
	}
	return file.Projects, nil
}

// ProjectOutputEntries flattens outputs into classpath entries, ordered by project path.
func ProjectOutputEntries(outputs map[string][]string) []string {
	projects := make([]string, 0, len(outputs))
	for project := range outputs {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	var entries []string
	for _, project := range projects {
		entries = append(entries, outputs[project]...)
	}
	return entries
}
