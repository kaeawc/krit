package scanner

import (
	"regexp"
	"sort"
	"strings"
)

var sourcePackageRe = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z_][A-Za-z0-9_.]*)`)

// TopLevelDeclaredNames returns the package of a parsed Kotlin file and the
// distinct names of its top-level declarations that other files can see:
// functions (extensions included), properties, classes, interfaces, and
// objects that are not private. These are the names whose arrival in a
// package can change how a call or type reference in another file resolves.
// The names come from the flat AST, so declarations inside comments, strings,
// and nested scopes are not reported.
func TopLevelDeclaredNames(file *File) (pkg string, names []string) {
	if file == nil || file.FlatTree == nil || file.FlatTree.Len() == 0 {
		return "", nil
	}
	symbols, _ := indexFile(file)
	seen := map[string]bool{}
	for _, sym := range symbols {
		if sym.Owner != "" || sym.Visibility == "private" || seen[sym.Name] {
			continue
		}
		seen[sym.Name] = true
		names = append(names, sym.Name)
	}
	sort.Strings(names)
	return packageNameForFile(file), names
}

// SourceHeaderFacts reads the package declaration and the imports from
// Kotlin source text. imports holds explicit import targets (the imported
// FQN, not the alias) and wildcards the packages of `.*` imports. It is a
// line-oriented scan of the header, not a parse.
func SourceHeaderFacts(content []byte) (pkg string, imports, wildcards []string) {
	if m := sourcePackageRe.FindSubmatch(content); m != nil {
		pkg = string(m[1])
	}
	for _, m := range sourceImportRe.FindAllSubmatch(content, -1) {
		target := string(m[1])
		if strings.HasSuffix(target, ".*") {
			wildcards = append(wildcards, strings.TrimSuffix(target, ".*"))
		} else if target != "" {
			imports = append(imports, target)
		}
	}
	return pkg, imports, wildcards
}
