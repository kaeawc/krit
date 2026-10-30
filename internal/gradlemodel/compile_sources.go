package gradlemodel

import (
	"path/filepath"
	"strings"
)

type compileSourceRef struct {
	projectIndex int
	setIndex     int
	fixtures     bool
}

type compileSourceSelection struct {
	model    *Model
	main     map[string][]compileSourceRef
	fixtures map[string][]compileSourceRef
	selected map[compileSourceRef]bool
	queue    []compileSourceRef
	resolved map[string]string
	resolver func(string) (string, error)
}

func (m *Model) CompileSourceDirs(scanPaths []string) []string {
	dirs, _ := m.CompileSourceContext(scanPaths)
	return dirs
}

func (m *Model) CompileSourceContext(scanPaths []string) (dirs []string, fallback bool) {
	if m == nil {
		return nil, false
	}
	selection := newCompileSourceSelection(m)
	selection.match(scanPaths)
	if len(selection.queue) == 0 {
		return m.SourceDirs(), true
	}
	selection.expand()
	return selection.sourceDirs(), false
}

func newCompileSourceSelection(model *Model) *compileSourceSelection {
	selection := &compileSourceSelection{
		model: model, main: map[string][]compileSourceRef{},
		fixtures: map[string][]compileSourceRef{}, selected: map[compileSourceRef]bool{},
		resolved: map[string]string{}, resolver: resolvedSourcePath,
	}
	for projectIndex, project := range model.Projects {
		for setIndex, set := range project.SourceSets {
			if set.Platform != "" && set.Platform != "jvm" && set.Platform != "android" {
				continue
			}
			ref := compileSourceRef{projectIndex: projectIndex, setIndex: setIndex}
			switch set.Kind {
			case "main":
				selection.main[project.Path] = append(selection.main[project.Path], ref)
			case "testFixtures":
				selection.fixtures[project.Path] = append(selection.fixtures[project.Path], ref)
			}
		}
	}
	return selection
}

func (selection *compileSourceSelection) enqueue(ref compileSourceRef) {
	if !selection.selected[ref] {
		selection.selected[ref] = true
		selection.queue = append(selection.queue, ref)
	}
}

func (selection *compileSourceSelection) match(paths []string) {
	resolvedPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		resolvedPaths = append(resolvedPaths, selection.resolvePath(path))
	}
	for projectIndex, project := range selection.model.Projects {
		for setIndex, set := range project.SourceSets {
			if selection.sourceSetOverlaps(set, resolvedPaths) {
				selection.enqueue(compileSourceRef{projectIndex, setIndex, set.Kind == "test" || set.Kind == "androidTest" || set.Kind == "testFixtures"})
			}
		}
	}
}

func (selection *compileSourceSelection) resolvePath(path string) string {
	if path == "" {
		return ""
	}
	if resolved, ok := selection.resolved[path]; ok {
		return resolved
	}
	resolved, err := selection.resolver(path)
	if err != nil {
		resolved = ""
	}
	selection.resolved[path] = resolved
	return resolved
}

func (selection *compileSourceSelection) sourceSetOverlaps(set SourceSet, paths []string) bool {
	roots := append(append([]string(nil), set.SourceDirs...), set.GeneratedSourceDirs...)
	for _, root := range roots {
		resolvedRoot := selection.resolvePath(root)
		for _, path := range paths {
			if sourcePathsOverlap(resolvedRoot, path) {
				return true
			}
		}
	}
	return false
}

func (selection *compileSourceSelection) includeProject(path string, fixtures bool) {
	for _, ref := range selection.main[path] {
		ref.fixtures = fixtures
		selection.enqueue(ref)
	}
	if fixtures {
		for _, ref := range selection.fixtures[path] {
			ref.fixtures = true
			selection.enqueue(ref)
		}
	}
}

func (selection *compileSourceSelection) expand() {
	for queueIndex := 0; queueIndex < len(selection.queue); queueIndex++ {
		ref := selection.queue[queueIndex]
		project := selection.model.Projects[ref.projectIndex]
		set := project.SourceSets[ref.setIndex]
		if set.Kind != "main" {
			selection.includeProject(project.Path, ref.fixtures)
		}
		for _, dependency := range set.ProjectDeps {
			selection.includeProject(dependency, ref.fixtures)
		}
	}
}

func (selection *compileSourceSelection) sourceDirs() []string {
	context := &Model{}
	for projectIndex, project := range selection.model.Projects {
		contextProject := project
		contextProject.SourceSets = nil
		for setIndex, set := range project.SourceSets {
			ref := compileSourceRef{projectIndex: projectIndex, setIndex: setIndex}
			fixtureRef := ref
			fixtureRef.fixtures = true
			if selection.selected[ref] || selection.selected[fixtureRef] {
				contextProject.SourceSets = append(contextProject.SourceSets, set)
			}
		}
		context.Projects = append(context.Projects, contextProject)
	}
	return context.SourceDirs()
}

func resolvedSourcePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return abs, nil
}

func sourcePathsOverlap(first, second string) bool {
	if first == "" || second == "" {
		return false
	}
	for _, pair := range [][2]string{{first, second}, {second, first}} {
		relative, err := filepath.Rel(pair[0], pair[1])
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
