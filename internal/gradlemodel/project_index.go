package gradlemodel

import (
	"path/filepath"
)

// ProjectIndex maps source files to the Gradle project that compiles them.
type ProjectIndex struct {
	// roots maps a clean directory to the project path owning it.
	roots map[string]string
}

// ProjectIndex indexes every project directory and source root of the model.
// A source root wins over the project directory that contains it, and a
// nested project wins over its parent, because the deepest root is matched.
func (m *Model) ProjectIndex() *ProjectIndex {
	idx := &ProjectIndex{roots: map[string]string{}}
	if m == nil {
		return idx
	}
	add := func(dir, project string) {
		if dir == "" {
			return
		}
		idx.roots[filepath.Clean(dir)] = project
		// The model and the scan may spell a symlinked checkout differently.
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			idx.roots[filepath.Clean(resolved)] = project
		}
	}
	for _, p := range m.Projects {
		add(p.Dir, p.Path)
	}
	for _, p := range m.Projects {
		for _, s := range p.SourceSets {
			for _, dir := range s.SourceDirs {
				add(dir, p.Path)
			}
			for _, dir := range s.GeneratedSourceDirs {
				add(dir, p.Path)
			}
		}
	}
	return idx
}

// Len returns the number of indexed directories.
func (idx *ProjectIndex) Len() int {
	if idx == nil {
		return 0
	}
	return len(idx.roots)
}

// ProjectOf returns the Gradle path of the project owning the absolute path,
// or "" when no project directory or source root contains it.
func (idx *ProjectIndex) ProjectOf(path string) string {
	if idx == nil || len(idx.roots) == 0 || path == "" {
		return ""
	}
	if project := idx.lookup(filepath.Clean(path)); project != "" {
		return project
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return idx.lookup(filepath.Clean(resolved))
	}
	return ""
}

func (idx *ProjectIndex) lookup(path string) string {
	for dir := path; ; {
		if project, ok := idx.roots[dir]; ok {
			return project
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
