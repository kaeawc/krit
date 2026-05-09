package scanner

import (
	"sort"
	"strings"
)

// DependentsIndex is a reverse-dependency map keyed by imported FQN.
// Given a changed file plus the FQNs that file declares (or removes),
// callers can compute the tight set of source files whose rule output
// might change. Used by watch-mode and LSP didChange to narrow
// incremental rerun scope from "every file" to "files reachable from
// the diff".
//
// The index records explicit imports only (`import a.b.C`). Wildcard
// imports (`import a.b.*`) are recorded as the package name. Aliases
// resolve to their underlying FQN.
type DependentsIndex struct {
	// importsByFile lists the imported FQNs for each source file. The
	// entries are sorted and deduped so callers can perform set diffs
	// without re-sorting.
	importsByFile map[string][]string

	// dependentsByFQN inverts importsByFile: given an FQN, list the
	// files that imported it. Sorted for stable output.
	dependentsByFQN map[string][]string

	// dependentsByPackage covers wildcard imports. A file that imports
	// "a.b.*" appears under "a.b". Callers that change a declaration in
	// package "a.b.X" should consult both dependentsByFQN["a.b.X"] and
	// dependentsByPackage["a.b"].
	dependentsByPackage map[string][]string
}

// BuildDependentsIndex walks each file's import_header nodes and
// constructs the per-file FQN list and its inverse. Pass parsed
// Kotlin files; non-Kotlin files contribute nothing. Files with no
// imports are still recorded with an empty slice so ImportsOfFile
// distinguishes "indexed but importless" from "not indexed".
func BuildDependentsIndex(files []*File) *DependentsIndex {
	idx := &DependentsIndex{
		importsByFile:       make(map[string][]string, len(files)),
		dependentsByFQN:     map[string][]string{},
		dependentsByPackage: map[string][]string{},
	}
	for _, f := range files {
		if f == nil || f.Language != LangKotlin {
			continue
		}
		fqns, packages := parseFileImports(f)
		idx.importsByFile[f.Path] = fqns
		for _, fqn := range fqns {
			idx.dependentsByFQN[fqn] = append(idx.dependentsByFQN[fqn], f.Path)
		}
		for _, pkg := range packages {
			idx.dependentsByPackage[pkg] = append(idx.dependentsByPackage[pkg], f.Path)
		}
	}
	for fqn, files := range idx.dependentsByFQN {
		sort.Strings(files)
		idx.dependentsByFQN[fqn] = dedupeStrings(files)
	}
	for pkg, files := range idx.dependentsByPackage {
		sort.Strings(files)
		idx.dependentsByPackage[pkg] = dedupeStrings(files)
	}
	return idx
}

// ImportsOfFile returns the explicit-import FQNs declared in the given
// file path. The returned slice is sorted and deduped. Returns nil for
// files the index doesn't know about.
func (d *DependentsIndex) ImportsOfFile(path string) []string {
	if d == nil {
		return nil
	}
	return d.importsByFile[path]
}

// FilesImporting returns the file paths that import the given FQN
// explicitly. Sorted ascending. Wildcard importers are not included
// here — use FilesImportingPackage for those.
func (d *DependentsIndex) FilesImporting(fqn string) []string {
	if d == nil {
		return nil
	}
	return d.dependentsByFQN[fqn]
}

// FilesImportingPackage returns the file paths that import the given
// package via a wildcard (`import pkg.*`). Sorted ascending.
func (d *DependentsIndex) FilesImportingPackage(pkg string) []string {
	if d == nil {
		return nil
	}
	return d.dependentsByPackage[pkg]
}

// FilesAffectedBy returns the union of:
//   - the changed files themselves
//   - every file that explicitly imports any FQN in changedFQNs
//   - every file with a wildcard import covering any FQN in changedFQNs
//
// The result is sorted and deduped. Used by watch-mode reruns to compute
// "given that file F changed and declares FQNs X, which files do I need
// to re-run rules on?" without scanning the whole project.
//
// changedFQNs may be nil, in which case only changedFiles are returned —
// useful when a file's edits affected nothing externally observable.
func (d *DependentsIndex) FilesAffectedBy(changedFiles, changedFQNs []string) []string {
	out := map[string]bool{}
	for _, f := range changedFiles {
		out[f] = true
	}
	if d != nil {
		for _, fqn := range changedFQNs {
			for _, f := range d.dependentsByFQN[fqn] {
				out[f] = true
			}
			if pkg := packageOfFQN(fqn); pkg != "" {
				for _, f := range d.dependentsByPackage[pkg] {
					out[f] = true
				}
			}
		}
	}
	result := make([]string, 0, len(out))
	for f := range out {
		result = append(result, f)
	}
	sort.Strings(result)
	return result
}

// parseFileImports walks the file's import_header nodes and returns the
// explicit-import FQNs and wildcard packages. Aliases are recorded as
// the underlying FQN; the alias name is irrelevant for reverse-dep
// queries.
func parseFileImports(file *File) (fqns []string, packages []string) {
	if file == nil || file.FlatTree == nil {
		return nil, nil
	}
	root := uint32(0)
	walkImportHeaders(file, root, &fqns, &packages)
	if len(fqns) > 1 {
		sort.Strings(fqns)
		fqns = dedupeStrings(fqns)
	}
	if len(packages) > 1 {
		sort.Strings(packages)
		packages = dedupeStrings(packages)
	}
	return fqns, packages
}

func walkImportHeaders(file *File, idx uint32, fqns *[]string, packages *[]string) {
	if file.FlatType(idx) == "import_header" {
		text := strings.TrimSpace(file.FlatNodeText(idx))
		text = strings.TrimPrefix(text, "import ")
		text = strings.TrimSpace(text)
		if i := strings.Index(text, " as "); i >= 0 {
			fqn := strings.TrimSpace(text[:i])
			if fqn != "" {
				*fqns = append(*fqns, fqn)
			}
			return
		}
		if strings.HasSuffix(text, ".*") {
			pkg := strings.TrimSuffix(text, ".*")
			if pkg != "" {
				*packages = append(*packages, pkg)
			}
			return
		}
		if text != "" {
			*fqns = append(*fqns, text)
		}
		return
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		walkImportHeaders(file, child, fqns, packages)
	}
}

func packageOfFQN(fqn string) string {
	last := strings.LastIndex(fqn, ".")
	if last < 0 {
		return ""
	}
	return fqn[:last]
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := in[:1]
	for _, s := range in[1:] {
		if s != out[len(out)-1] {
			out = append(out, s)
		}
	}
	return out
}
