package fileignore

import (
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

// DefaultPrunedDir reports Krit's built-in source-discovery skips.
func DefaultPrunedDir(base string) bool {
	switch base {
	case ".git", "build", "node_modules", ".idea", ".gradle", "out", ".kotlin",
		"target", "third-party", "third_party", "vendor", "external":
		return true
	}
	return false
}

// Matcher applies .gitignore files from the Git root down to a candidate path.
type Matcher struct {
	root     string
	files    map[string][]string
	combined map[string]*ignore.GitIgnore
}

func MatcherForPath(path string, info os.FileInfo, matchers map[string]*Matcher) *Matcher {
	root := path
	if !info.IsDir() {
		root = filepath.Dir(path)
	}
	root = FindGitRoot(root)
	if matchers != nil {
		if m := matchers[root]; m != nil {
			return m
		}
	}
	m := &Matcher{
		root:     root,
		files:    make(map[string][]string),
		combined: make(map[string]*ignore.GitIgnore),
	}
	if matchers != nil {
		matchers[root] = m
	}
	return m
}

func FindGitRoot(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	dir := abs
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs
		}
		dir = parent
	}
}

func (m *Matcher) Ignored(path string, isDir bool) bool {
	if m == nil {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	rel, err := filepath.Rel(m.root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	gi := m.ignoreForDir(candidateDir(abs, isDir))
	if gi == nil {
		return false
	}
	return gi.MatchesPath(rel) || (isDir && gi.MatchesPath(rel+string(filepath.Separator)))
}

func candidateDir(path string, isDir bool) string {
	if isDir {
		return path
	}
	return filepath.Dir(path)
}

func (m *Matcher) ignoreDirsFor(path string, isDir bool) []string {
	dir := candidateDir(path, isDir)
	var rev []string
	for {
		rel, err := filepath.Rel(m.root, dir)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			rev = append(rev, dir)
		}
		if dir == m.root {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	dirs := make([]string, 0, len(rev))
	for i := len(rev) - 1; i >= 0; i-- {
		dirs = append(dirs, rev[i])
	}
	return dirs
}

func (m *Matcher) ignoreForDir(dir string) *ignore.GitIgnore {
	if gi, ok := m.combined[dir]; ok {
		return gi
	}
	var lines []string
	for _, ignoreDir := range m.ignoreDirsFor(dir, true) {
		relDir, err := filepath.Rel(m.root, ignoreDir)
		if err != nil {
			continue
		}
		for _, line := range m.linesForDir(ignoreDir) {
			lines = append(lines, qualifyPattern(line, relDir))
		}
	}
	if len(lines) == 0 {
		m.combined[dir] = nil
		return nil
	}
	gi := ignore.CompileIgnoreLines(lines...)
	m.combined[dir] = gi
	return gi
}

func (m *Matcher) linesForDir(dir string) []string {
	if lines, ok := m.files[dir]; ok {
		return lines
	}
	content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		m.files[dir] = nil
		return nil
	}
	lines := strings.Split(string(content), "\n")
	m.files[dir] = lines
	return lines
}

func qualifyPattern(line, relDir string) string {
	trimmed := strings.TrimSpace(line)
	if relDir == "." || relDir == "" || trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return line
	}

	prefix := ""
	pattern := line
	if strings.HasPrefix(pattern, "!") {
		prefix = "!"
		pattern = strings.TrimPrefix(pattern, "!")
	}
	if strings.HasPrefix(pattern, "\\!") || strings.HasPrefix(pattern, "\\#") {
		return relDir + "/" + pattern
	}
	if strings.HasPrefix(pattern, "/") {
		return prefix + relDir + "/" + strings.TrimPrefix(pattern, "/")
	}
	if strings.Contains(pattern, "/") {
		return prefix + relDir + "/" + pattern
	}
	return prefix + relDir + "/**/" + pattern
}
