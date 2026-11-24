package modshard

// variant.go — recheck gated files in variant source directories the Gradle model did not export
// (#1023). The model exports one Android variant per project, but a directory scan still compiles
// sibling variant directories (src/release next to an exported debug, product-flavor directories)
// in the merged compile, where a class each build type declares on its own is a redeclaration.

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

// maxVariantUnits bounds the variant source sets one recheck compiles. Each is a compile of its own;
// the ones beyond it, in sorted order, keep their merged verdicts.
const maxVariantUnits = 16

// variantUnits returns one unit per unexported variant source set holding a gated file in files,
// compiled the way that variant's build would see it: the set's own kotlin and java directories, with
// the base set of its kind (src/main, src/test or src/androidTest), the sets its name combines
// (src/free and src/release for src/freeRelease), and the template unit's generated sources and
// dependency context compiled alongside but not checked. The exported variant's own directories
// (src/debug for a debug model) are left out, since the real build never compiles them with this set.
// The template is the project's exported unit of the same kind, whose classpath, compiler settings and
// friends the unit takes. Projects in skipped, and sets without a template or whose template lost
// dependency context, keep their merged verdicts. beyond lists the sets over maxVariantUnits.
func variantUnits(model *gradlemodel.Model, units []Unit, skipped, files []string) (out []Unit, beyond []string) {
	if model == nil || len(files) == 0 {
		return nil, nil
	}
	variantOf := map[string]string{}
	for _, p := range model.Projects {
		for _, s := range p.SourceSets {
			variantOf[p.Path+":"+s.Name] = s.Variant
		}
	}
	template := map[string]*Unit{} // project path + "\x00" + kind
	for i := range units {
		key := projectOf(units[i].Spec.ID) + "\x00" + units[i].Spec.Kind
		if _, ok := template[key]; !ok {
			template[key] = &units[i]
		}
	}
	for _, g := range variantGroups(model, files) {
		kind, _ := variantKind(g.set)
		t := template[g.project+"\x00"+kind]
		if kind == "" || t == nil || t.ContextDropped || slices.Contains(skipped, g.project) {
			continue
		}
		if len(out) == maxVariantUnits {
			beyond = append(beyond, fmt.Sprintf("%s (%d gated files)", filepath.Join(g.srcDir, g.set), g.files))
			continue
		}
		if u, ok := variantUnit(g, t, variantOf[t.Spec.ID]); ok {
			out = append(out, u)
		}
	}
	return out, beyond
}

// variantGroup is one unexported source set of a project and the number of gated files in it.
type variantGroup struct {
	project, srcDir, set string
	files                int
}

// variantGroups groups files under conventional source roots by their project (the nearest enclosing
// model project) and source set, sorted. Files outside every project are left out.
func variantGroups(model *gradlemodel.Model, files []string) []*variantGroup {
	type project struct{ path, dir string }
	var projects []project
	for _, p := range model.Projects {
		if p.Dir != "" {
			projects = append(projects, project{p.Path, filepath.Clean(p.Dir)})
		}
	}
	sort.Slice(projects, func(i, j int) bool { return len(projects[i].dir) > len(projects[j].dir) })
	groups := map[string]*variantGroup{}
	for _, f := range files {
		srcDir, set, ok := sourceSetOf(f)
		if !ok {
			continue
		}
		for _, p := range projects {
			if f == p.dir || strings.HasPrefix(f, p.dir+string(filepath.Separator)) {
				key := p.path + "\x00" + filepath.Join(srcDir, set)
				if groups[key] == nil {
					groups[key] = &variantGroup{project: p.path, srcDir: srcDir, set: set}
				}
				groups[key].files++
				break
			}
		}
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]*variantGroup, len(keys))
	for i, k := range keys {
		out[i] = groups[k]
	}
	return out
}

// variantUnit builds the unit of g from template t, the project's exported set of g's kind, whose
// variant is exportedVariant. It reports false when the set has no kotlin or java directory.
func variantUnit(g *variantGroup, t *Unit, exportedVariant string) (Unit, bool) {
	own := setDirs(g.srcDir, g.set)
	if len(own) == 0 {
		return Unit{}, false
	}
	kind, base := variantKind(g.set)
	spec := t.Spec
	spec.ID = g.project + ":variant-" + g.set
	spec.SourceRoots = own
	spec.GeneratedSourceRoots = []string{}
	spec.Fragments = nil
	exported := map[string]bool{}
	for _, c := range nameComponents(exportedVariant, true) {
		exported[prefixed(base, kind, c)] = true
	}
	var context []string
	for _, root := range t.Spec.SourceRoots {
		if _, set, ok := sourceSetOf(filepath.Join(root, "x")); ok && exported[set] && set != base {
			continue
		}
		context = append(context, root)
	}
	combined := g.set
	if kind != "main" {
		combined = strings.TrimPrefix(g.set, base)
	}
	for _, c := range nameComponents(combined, false) {
		context = append(context, setDirs(g.srcDir, prefixed(base, kind, c))...)
	}
	context = append(context, t.Spec.GeneratedSourceRoots...)
	context = append(context, t.Spec.ContextSourceRoots...)
	spec.ContextSourceRoots = dedupe(context, own)
	sources := sourceFiles(own)
	return Unit{Spec: spec, Files: len(sources), Sources: sources}, true
}

// sourceSetOf returns the src directory and source set name of a file under a conventional
// src/<set>/kotlin or src/<set>/java root.
func sourceSetOf(file string) (srcDir, set string, ok bool) {
	parts := strings.Split(filepath.ToSlash(file), "/")
	for i := 0; i+3 < len(parts); i++ {
		if parts[i] == "src" && (parts[i+2] == "kotlin" || parts[i+2] == "java") && parts[i+1] != "" {
			return filepath.FromSlash(strings.Join(parts[:i+1], "/")), parts[i+1], true
		}
	}
	return "", "", false
}

// variantKind returns the unit kind a source set name compiles as, and that kind's base set name:
// testRelease is a test set, androidTestFree an androidTest set, any other a main set. Test fixtures
// are not exported as a kind of their own, so they have none.
func variantKind(set string) (kind, base string) {
	switch {
	case hasWordPrefix(set, "testFixtures"):
		return "", ""
	case hasWordPrefix(set, "androidTest"):
		return "androidTest", "androidTest"
	case hasWordPrefix(set, "test"):
		return "test", "test"
	}
	return "main", "main"
}

// hasWordPrefix reports whether set is prefix or prefix followed by a capitalized word.
func hasWordPrefix(set, prefix string) bool {
	rest, ok := strings.CutPrefix(set, prefix)
	return ok && (rest == "" || unicode.IsUpper(rune(rest[0])))
}

// prefixed spells a variant name component as a set of kind: "release" is release for main and
// testRelease for test. An empty component is the base set.
func prefixed(base, kind, component string) string {
	if component == "" {
		return base
	}
	if kind == "main" {
		return component
	}
	return base + strings.ToUpper(component[:1]) + component[1:]
}

// nameComponents returns the source set names a variant or flavor-combination name is built from:
// every run of consecutive camelCase words, lowercased first (freeStagingRelease gives free, staging,
// release, freeStaging and stagingRelease). whole also includes the name itself.
func nameComponents(name string, whole bool) []string {
	var words []string
	start := 0
	for i, r := range name {
		if i > 0 && unicode.IsUpper(r) {
			words = append(words, name[start:i])
			start = i
		}
	}
	if start < len(name) {
		words = append(words, name[start:])
	}
	var out []string
	for i := range words {
		for j := i + 1; j <= len(words); j++ {
			if !whole && i == 0 && j == len(words) {
				continue
			}
			c := strings.Join(words[i:j], "")
			out = append(out, strings.ToLower(c[:1])+c[1:])
		}
	}
	return out
}

// setDirs returns the existing kotlin and java directories of a source set.
func setDirs(srcDir, set string) []string {
	var out []string
	for _, lang := range []string{"kotlin", "java"} {
		dir := filepath.Join(srcDir, set, lang)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			out = append(out, dir)
		}
	}
	return out
}

// dedupe returns roots in order without repeats or any root in exclude.
func dedupe(roots, exclude []string) []string {
	seen := map[string]bool{}
	for _, r := range exclude {
		seen[filepath.Clean(r)] = true
	}
	out := []string{}
	for _, r := range roots {
		if c := filepath.Clean(r); !seen[c] {
			seen[c] = true
			out = append(out, r)
		}
	}
	return out
}
