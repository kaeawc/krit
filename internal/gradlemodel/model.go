// Package gradlemodel reads the classpath model exported by krit-gradle-plugin.
package gradlemodel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/hashutil"
)

// Model contains valid projects from all readable model files.
type Model struct{ Projects []Project }

// Project is one Gradle project and its exported source sets.
type Project struct {
	Path       string      `json:"path"`
	Dir        string      `json:"dir"`
	SourceSets []SourceSet `json:"sourceSets"`
}

// SourceSet contains compiler paths exported for one source set.
type SourceSet struct {
	Name                string   `json:"name"`
	Kind                string   `json:"kind"`
	Platform            string   `json:"platform"`
	Variant             string   `json:"variant"`
	SourceDirs          []string `json:"sourceDirs"`
	GeneratedSourceDirs []string `json:"generatedSourceDirs"`
	ClasspathEntries    []string `json:"classpath"`
	GeneratedClasspath  []string `json:"generatedClasspath"`
	BootClasspath       []string `json:"bootClasspath"`
	ProjectDeps         []string `json:"projectDeps"`
	JvmTarget           string   `json:"jvmTarget"`
}

// Load reads JSON files directly in dir. Invalid files and projects with
// missing directories are skipped with warnings; failure to read dir is fatal.
func Load(dir string) (*Model, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("read gradle model %s: %w", dir, err)
	}
	model := &Model{}
	var warnings []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		var file struct {
			Schema   int       `json:"schema"`
			Projects []Project `json:"projects"`
		}
		if err := json.Unmarshal(data, &file); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: malformed JSON: %v", path, err))
			continue
		}
		if file.Schema != 1 {
			warnings = append(warnings, fmt.Sprintf("%s: unsupported schema %d", path, file.Schema))
			continue
		}
		for _, project := range file.Projects {
			info, err := os.Stat(project.Dir)
			if err != nil || !info.IsDir() {
				warnings = append(warnings, fmt.Sprintf("%s: project %s directory missing: %s", path, project.Path, project.Dir))
				continue
			}
			model.Projects = append(model.Projects, project)
		}
	}
	sort.SliceStable(model.Projects, func(i, j int) bool { return model.Projects[i].Path < model.Projects[j].Path })
	return model, warnings, nil
}

// Classpath returns existing boot entries followed by existing ordinary entries,
// deduplicated in project-path and source-set order. The second result counts
// missing entries before deduplication.
func (m *Model) Classpath() ([]string, int) {
	if m == nil {
		return nil, 0
	}
	seen := map[string]bool{}
	var out []string
	missing := 0
	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		if _, err := os.Stat(p); err != nil {
			missing++
			return
		}
		out = append(out, p)
	}
	for _, p := range m.Projects {
		for _, s := range p.SourceSets {
			for _, entry := range s.BootClasspath {
				add(entry)
			}
		}
	}
	for _, p := range m.Projects {
		for _, s := range p.SourceSets {
			for _, entry := range s.ClasspathEntries {
				add(entry)
			}
		}
	}
	for _, p := range m.Projects {
		for _, s := range p.SourceSets {
			for _, entry := range s.GeneratedClasspath {
				add(entry)
			}
		}
	}
	return out, missing
}

// SourceDirs returns existing source and generated source roots in model order.
func (m *Model) SourceDirs() []string {
	if m == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range m.Projects {
		for _, s := range p.SourceSets {
			for _, dir := range append(append([]string(nil), s.SourceDirs...), s.GeneratedSourceDirs...) {
				if dir == "" || seen[dir] {
					continue
				}
				seen[dir] = true
				if info, err := os.Stat(dir); err == nil && info.IsDir() {
					out = append(out, dir)
				}
			}
		}
	}
	return out
}

// MaxJvmTarget returns the highest numeric JVM target declared by the model.
// Whole-repo FIR compilation uses this one target; per-module targets require
// a separate compilation design.
func (m *Model) MaxJvmTarget() string {
	if m == nil {
		return ""
	}
	best, version := "", -1
	for _, p := range m.Projects {
		for _, s := range p.SourceSets {
			n, ok := parseJvmTarget(s.JvmTarget)
			if ok && n > version {
				best, version = s.JvmTarget, n
			}
		}
	}
	return best
}

func parseJvmTarget(target string) (int, bool) {
	target = strings.TrimPrefix(target, "1.")
	n, err := strconv.Atoi(target)
	return n, err == nil && n > 0
}

// BootClasspath returns existing boot entries, deduplicated in project-path
// and source-set order. Missing entries are dropped.
func (m *Model) BootClasspath() []string {
	if m == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range m.Projects {
		for _, s := range p.SourceSets {
			for _, entry := range s.BootClasspath {
				if entry == "" || seen[entry] {
					continue
				}
				seen[entry] = true
				if _, err := os.Stat(entry); err == nil {
					out = append(out, entry)
				}
			}
		}
	}
	return out
}

// CompileClasspath returns existing ordinary entries, deduplicated in
// project-path and source-set order. Missing entries are dropped.
func (m *Model) CompileClasspath() []string {
	if m == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range m.Projects {
		for _, s := range p.SourceSets {
			for _, entry := range s.ClasspathEntries {
				if entry == "" || seen[entry] {
					continue
				}
				seen[entry] = true
				if _, err := os.Stat(entry); err == nil {
					out = append(out, entry)
				}
			}
		}
	}
	return out
}

// GeneratedClasspath returns existing generated class jars in model order.
func (m *Model) GeneratedClasspath() []string {
	if m == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range m.Projects {
		for _, s := range p.SourceSets {
			for _, entry := range s.GeneratedClasspath {
				if entry == "" || seen[entry] {
					continue
				}
				seen[entry] = true
				if _, err := os.Stat(entry); err == nil {
					out = append(out, entry)
				}
			}
		}
	}
	return out
}

// SettingsAncestors returns settings roots from nearest to outermost, stopping
// at the nearest .git directory or worktree pointer file (inclusive).
func SettingsAncestors(scanRoot string) []string {
	dir, err := filepath.Abs(scanRoot)
	if err != nil {
		return nil
	}
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	var roots []string
	for {
		for _, settings := range []string{"settings.gradle", "settings.gradle.kts"} {
			if info, err := os.Stat(filepath.Join(dir, settings)); err == nil && !info.IsDir() {
				roots = append(roots, dir)
				break
			}
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return roots
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return roots
		}
		dir = parent
	}
}

// Discover returns the nearest .krit/gradle-model directory at or above scanRoot,
// searching up to and including the nearest settings.gradle(.kts) root and
// stopping at a .git directory or worktree pointer file.
func Discover(scanRoot string) string {
	dir, err := filepath.Abs(scanRoot)
	if err != nil {
		return ""
	}
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		model := filepath.Join(dir, ".krit", "gradle-model")
		if info, err := os.Stat(model); err == nil && info.IsDir() {
			return model
		}
		for _, settings := range []string{"settings.gradle", "settings.gradle.kts"} {
			if info, err := os.Stat(filepath.Join(dir, settings)); err == nil && !info.IsDir() {
				return ""
			}
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// Fingerprint hashes resolved classpath paths, file sizes, and modification
// times. It changes when a referenced jar is replaced at the same path.
func (m *Model) Fingerprint() string {
	entries, _ := m.Classpath()
	return ClasspathFingerprint(entries)
}

// ClasspathFingerprint hashes paths, sizes, and modification times in order.
// Missing paths retain a marker so removing an existing entry changes the key.
func ClasspathFingerprint(entries []string) string {
	var b strings.Builder
	for _, entry := range entries {
		b.WriteString(entry)
		b.WriteByte(0)
		if info, err := os.Stat(entry); err == nil {
			b.WriteString(strconv.FormatInt(info.Size(), 10))
			b.WriteByte(0)
			b.WriteString(strconv.FormatInt(info.ModTime().UnixNano(), 10))
		} else {
			b.WriteString("missing")
		}
		b.WriteByte(0)
	}
	return hashutil.HashHex([]byte(b.String()))
}
