// Package modshard produces the type oracle by analyzing Gradle modules independently, in parallel
// worker JVMs, instead of compiling the whole repository in one JVM.
//
// A module is compiled against the Gradle-built outputs of its dependencies, so modules do not
// depend on each other during analysis and can run in any order. That needs a prior Gradle build;
// when the build outputs are missing, Run reports that it did not apply and the caller falls back
// to the whole-repository compile.
package modshard

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/gradlemodel"
)

// Unit is one module (Gradle source set) to analyze.
type Unit struct {
	Spec firchecks.ModuleSpec
	// Files is the number of Kotlin and Java sources, used to balance the shards.
	Files int
	// ContextFiles is the number of dependency sources compiled with the module as context because
	// those dependencies have no built output; they cost compile time like Files (#1661).
	ContextFiles int
	// Sources lists the module's own source files (sorted), found once per run; nil means not listed yet.
	Sources []string
	// ContextDropped is set when a dependency without built output was left out of the compile because
	// its sources exceed maxContextFiles; the module's facts are then incomplete.
	ContextDropped bool
}

// builtJar returns the Gradle-built library jar of the module at projectPath, or "".
func builtJar(repoRoot, projectPath string) string {
	project := strings.Split(strings.Trim(projectPath, ":"), ":")
	if len(project) == 0 || project[0] == "" {
		return ""
	}
	dir := filepath.Join(append([]string{repoRoot}, project...)...)
	for _, rel := range []string{
		"build/intermediates/compile_library_classes_jar/release/bundleLibCompileToJarRelease/classes.jar",
		"build/libs/" + project[len(project)-1] + ".jar",
	} {
		path := filepath.Join(dir, rel)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

// projectOf is the Gradle project path of a module ID ("<project>:<sourceSet>").
func projectOf(moduleID string) string {
	if i := strings.LastIndex(moduleID, ":"); i > 0 {
		return moduleID[:i]
	}
	return moduleID
}

// maxContextFiles bounds the dependency sources one module compiles because those dependencies were
// not built; beyond it the dependency stays unresolved (and the module's files non-authoritative).
const maxContextFiles = 3000

// projectJars returns the built outputs of the project at projectPath: the jars the Gradle plugin
// passed for it (--gradle-project-outputs, which come from this build's own compile tasks), else the
// conventional build/libs or release classes.jar. An outputs entry wins even over a jar at a
// conventional path, which may be left from an older build. A stale project (sources changed since
// the build, #1660) gets no conventional jar, which predates the change; its passed outputs were
// compiled from the current sources and still count. Only files that exist count.
func projectJars(repoRoot, projectPath string, outputs map[string][]string, stale bool) []string {
	var jars []string
	for _, jar := range outputs[projectPath] {
		if info, err := os.Stat(jar); err == nil && !info.IsDir() {
			jars = append(jars, jar)
		}
	}
	if len(jars) > 0 || stale {
		return jars
	}
	if jar := builtJar(repoRoot, projectPath); jar != "" {
		return []string{jar}
	}
	return nil
}

// Plan turns the model's modules into independent units. Project dependencies become built jars on
// the classpath (transitively), a test module also sees its project's main jar as a friend, and
// module edges are dropped. Only modules with a source root under one of scanPaths are returned.
// outputs are the projects' built outputs the Gradle plugin passed (--gradle-project-outputs), used
// before the conventional jar paths (see projectJars). missing counts the dependency projects with no
// built jar.
func Plan(model *gradlemodel.Model, repoRoot string, scanPaths []string, outputs map[string][]string) (units []Unit, missing int) {
	return planWith(model, repoRoot, scanPaths, outputs, nil)
}

// planWith is Plan with the projects whose built jar is stale: their sources changed since the
// build (PR mode's changed files, #1660), so a module depending on them compiles their sources as
// context instead of a jar that predates the change, unless outputs has a jar for them.
func planWith(model *gradlemodel.Model, repoRoot string, scanPaths []string, outputs map[string][]string, stale map[string]bool) (units []Unit, missing int) {
	specs := firchecks.ModuleSpecsFromModel(model)
	byID := make(map[string]*firchecks.ModuleSpec, len(specs))
	for i := range specs {
		byID[specs[i].ID] = &specs[i]
	}
	jars := map[string][]string{}
	jarsOf := func(project string) []string {
		if j, ok := jars[project]; ok {
			return j
		}
		j := projectJars(repoRoot, project, outputs, stale[project])
		jars[project] = j
		return j
	}
	closures := dependencyClosures(byID)
	missingProjects := map[string]bool{}
	for i := range specs {
		spec := specs[i]
		if !underAny(spec.SourceRoots, scanPaths) {
			continue
		}
		have := map[string]bool{}
		for _, entry := range spec.Classpath {
			have[entry] = true
		}
		add := func(jar string) {
			if jar != "" && !have[jar] {
				have[jar] = true
				spec.Classpath = append(spec.Classpath, jar)
			}
		}
		contextBudget := maxContextFiles
		contextDropped := false
		for _, dep := range closures(spec.ID) {
			depJars := jarsOf(projectOf(dep))
			if len(depJars) == 0 {
				missingProjects[projectOf(dep)] = true
				// No build output: compile the dependency's sources with this module instead.
				if d := byID[dep]; d != nil {
					roots := append(append([]string{}, d.SourceRoots...), d.GeneratedSourceRoots...)
					if n := len(sourceFiles(roots)); n <= contextBudget {
						contextBudget -= n
						spec.ContextSourceRoots = append(spec.ContextSourceRoots, roots...)
					} else {
						contextDropped = true
					}
				}
			}
			for _, jar := range depJars {
				add(jar)
			}
		}
		spec.Friends = []string{}
		if spec.Kind != "main" {
			if own := jarsOf(projectOf(spec.ID)); len(own) > 0 {
				for _, jar := range own {
					add(jar)
				}
				spec.Friends = append([]string(nil), own...)
			}
		}
		spec.DependsOn = []string{}
		sources := sourceFiles(append(append([]string{}, spec.SourceRoots...), spec.GeneratedSourceRoots...))
		units = append(units, Unit{Spec: spec, Files: len(sources), ContextFiles: maxContextFiles - contextBudget, Sources: sources, ContextDropped: contextDropped})
	}
	sort.Slice(units, func(i, j int) bool { return units[i].Spec.ID < units[j].Spec.ID })
	return units, len(missingProjects)
}

// dependencyClosures returns, for a module ID, its transitive main-module dependencies (a cycle is
// cut where it closes).
func dependencyClosures(byID map[string]*firchecks.ModuleSpec) func(id string) []string {
	closure := map[string][]string{}
	var deps func(id string, seen map[string]bool) []string
	deps = func(id string, seen map[string]bool) []string {
		if done, ok := closure[id]; ok {
			return done
		}
		spec := byID[id]
		if spec == nil || seen[id] {
			return nil
		}
		seen[id] = true
		var out []string
		have := map[string]bool{}
		for _, dep := range spec.DependsOn {
			if byID[dep] == nil || byID[dep].Kind != "main" {
				continue
			}
			for _, d := range append([]string{dep}, deps(dep, seen)...) {
				if !have[d] {
					have[d] = true
					out = append(out, d)
				}
			}
		}
		delete(seen, id)
		closure[id] = out
		return out
	}
	return func(id string) []string { return deps(id, map[string]bool{}) }
}

func underAny(roots, scanPaths []string) bool {
	for _, root := range roots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		for _, scan := range scanPaths {
			absScan, err := filepath.Abs(scan)
			if err != nil {
				continue
			}
			if rel, err := filepath.Rel(absScan, absRoot); err == nil && !strings.HasPrefix(rel, "..") {
				return true
			}
			if rel, err := filepath.Rel(absRoot, absScan); err == nil && !strings.HasPrefix(rel, "..") {
				return true
			}
		}
	}
	return false
}

// Shards splits units into n groups with balanced estimated work (longest first, each to the least
// loaded group). A unit's work is the files it compiles, its own and its dependency context, plus a
// small constant for fixed per-module cost.
func Shards(units []Unit, n int) [][]Unit {
	weights := make([]int, max(1, n))
	for i := range weights {
		weights[i] = 1
	}
	return weightedShards(units, weights)
}

// weightedShards is Shards over groups that work through their units at different rates, such as
// worker JVMs with different thread counts: group i gets a share of the work in proportion to
// weights[i].
func weightedShards(units []Unit, weights []int) [][]Unit {
	order := append([]Unit(nil), units...)
	sort.SliceStable(order, func(i, j int) bool { return order[i].work() > order[j].work() })
	groups := make([][]Unit, len(weights))
	load := make([]int, len(weights))
	for _, u := range order {
		best := 0
		for i := range load {
			// load[i]/weights[i] < load[best]/weights[best], without rounding.
			if load[i]*weights[best] < load[best]*weights[i] {
				best = i
			}
		}
		groups[best] = append(groups[best], u)
		load[best] += u.work()
	}
	return groups
}

// work estimates a unit's compile cost for Shards.
func (u Unit) work() int { return u.Files + u.ContextFiles + 3 }

// ResidualUnits covers files that no exported source set owns (variant-specific or non-standard
// source directories that a directory scan still finds). Files are grouped by their source root
// under the nearest enclosing project, and each group is analyzed like that project's main module,
// with its sources replaced by the group's root. Files outside every project stay uncovered.
func ResidualUnits(model *gradlemodel.Model, units []Unit, files []string) []Unit {
	type project struct {
		path, dir string
	}
	var projects []project
	for _, p := range model.Projects {
		if p.Dir != "" {
			projects = append(projects, project{p.Path, filepath.Clean(p.Dir)})
		}
	}
	sort.Slice(projects, func(i, j int) bool { return len(projects[i].dir) > len(projects[j].dir) })
	templateOf := map[string]*Unit{}
	for i := range units {
		u := &units[i]
		project := projectOf(u.Spec.ID)
		if cur, ok := templateOf[project]; !ok || (u.Spec.Kind == "main" && cur.Spec.Kind != "main") {
			templateOf[project] = u
		}
	}
	groups := map[string][]string{} // project path + "\x00" + root -> files
	for _, f := range files {
		for _, p := range projects {
			if f == p.dir || strings.HasPrefix(f, p.dir+string(filepath.Separator)) {
				root := sourceRootOf(f)
				groups[p.path+"\x00"+root] = append(groups[p.path+"\x00"+root], f)
				break
			}
		}
	}
	var out []Unit
	for key, fs := range groups {
		projectPath, root, _ := strings.Cut(key, "\x00")
		template := templateOf[projectPath]
		if template == nil {
			continue
		}
		spec := template.Spec
		spec.ID = projectPath + ":residual:" + filepath.ToSlash(root)
		spec.SourceRoots = []string{root}
		spec.GeneratedSourceRoots = []string{}
		spec.Fragments = nil
		out = append(out, Unit{Spec: spec, Files: len(fs), ContextFiles: template.ContextFiles})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Spec.ID < out[j].Spec.ID })
	return out
}

// sourceRootOf is the "src/<set>/kotlin" or "src/<set>/java" directory holding file, or its own
// directory when it is not under one.
func sourceRootOf(file string) string {
	parts := strings.Split(filepath.ToSlash(file), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "src" && (parts[i+2] == "kotlin" || parts[i+2] == "java") {
			return filepath.FromSlash(strings.Join(parts[:i+3], "/"))
		}
	}
	return filepath.Dir(file)
}

// NeedsIsolation reports whether some project has several of its source sets among the units. Those
// are the units the whole-repository compile merges, so a declaration present in two of them is
// reported as a redeclaration there and not here.
func NeedsIsolation(units []Unit) bool {
	seen := map[string]bool{}
	for _, u := range units {
		project := projectOf(u.Spec.ID)
		if seen[project] {
			return true
		}
		seen[project] = true
	}
	return false
}

// incompleteContext returns the ID of the first unit that left a dependency out, or "".
func incompleteContext(units []Unit) string {
	for _, u := range units {
		if u.ContextDropped {
			return u.Spec.ID
		}
	}
	return ""
}
