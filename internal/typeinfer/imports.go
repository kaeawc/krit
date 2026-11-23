package typeinfer

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/sourceheader"
)

type fileHeaders struct {
	pkg string
	it  *ImportTable
}

// scanFileHeadersFlat reads the package and import headers of a file.
func scanFileHeadersFlat(file *scanner.File) fileHeaders {
	headers := fileHeaders{
		it: &ImportTable{
			Explicit: make(map[string]string),
			Aliases:  make(map[string]string),
		},
	}
	if file == nil || file.FlatTree == nil || file.FlatTree.Len() == 0 {
		return headers
	}
	// Package and import headers precede every top-level declaration, so
	// stop at the first one. Callers resolve the package once per type
	// reference; scanning a file with thousands of top-level declarations
	// each time made indexing superlinear.
	t := file.FlatTree
	for child := t.FirstChildren[0]; child != 0; child = t.NextSibs[child] {
		if !t.Node(child).IsNamed() {
			continue
		}
		switch file.FlatType(child) {
		case "package_header":
			headers.pkg = sourceheader.FirstHeaderLine(file.FlatNodeText(child), "package")
		case "import_header", "import_list":
			extractImportsFlat(child, file, headers.it)
		default:
			if isTopLevelDeclarationType(file.FlatType(child)) {
				return headers
			}
		}
	}
	return headers
}

func isTopLevelDeclarationType(t string) bool {
	switch t {
	case "class_declaration", "object_declaration", "function_declaration",
		"property_declaration", "type_alias":
		return true
	}
	return false
}

func buildImportTableFlat(file *scanner.File) *ImportTable {
	return scanFileHeadersFlat(file).it
}

func extractImportsFlat(nodeIdx uint32, file *scanner.File, it *ImportTable) {
	if file == nil || file.FlatTree == nil || nodeIdx == 0 {
		return
	}
	if file.FlatType(nodeIdx) == "import_header" {
		text := sourceheader.FirstHeaderLine(file.FlatNodeText(nodeIdx), "import")

		if idx := strings.Index(text, " as "); idx >= 0 {
			fqn := strings.TrimSpace(text[:idx])
			alias := strings.TrimSpace(text[idx+4:])
			it.Aliases[alias] = fqn
			return
		}

		if strings.HasSuffix(text, ".*") {
			pkg := strings.TrimSuffix(text, ".*")
			it.Wildcard = append(it.Wildcard, pkg)
			return
		}

		parts := strings.Split(text, ".")
		if len(parts) > 0 {
			simpleName := parts[len(parts)-1]
			it.Explicit[simpleName] = text
		}
		return
	}

	forEachFlatNamedChild(file, nodeIdx, func(child uint32) {
		extractImportsFlat(child, file, it)
	})
}

func extractPackageFlat(file *scanner.File) string {
	return scanFileHeadersFlat(file).pkg
}
