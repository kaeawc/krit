package typeinfer

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

type fileHeaders struct {
	pkg string
	it  *ImportTable
}

func scanFileHeadersFlat(rootIdx uint32, file *scanner.File) fileHeaders {
	headers := fileHeaders{
		it: &ImportTable{
			Explicit: make(map[string]string),
			Aliases:  make(map[string]string),
		},
	}
	if file == nil || file.FlatTree == nil || int(rootIdx) >= len(file.FlatTree.Nodes) {
		return headers
	}
	visitChild := func(child uint32) {
		switch file.FlatType(child) {
		case "package_header":
			text := strings.TrimSpace(file.FlatNodeText(child))
			text = strings.TrimPrefix(text, "package ")
			headers.pkg = strings.TrimSpace(text)
		case "import_header", "import_list":
			extractImportsFlat(child, file, headers.it)
		}
	}
	if rootIdx == 0 {
		for i := 0; i < file.FlatNamedChildCount(0); i++ {
			child := file.FlatNamedChild(0, i)
			if child != 0 {
				visitChild(child)
			}
		}
		return headers
	}
	forEachFlatNamedChild(file, rootIdx, visitChild)
	return headers
}

func buildImportTableFlat(rootIdx uint32, file *scanner.File) *ImportTable {
	return scanFileHeadersFlat(rootIdx, file).it
}

func extractImportsFlat(nodeIdx uint32, file *scanner.File, it *ImportTable) {
	if file == nil || file.FlatTree == nil || nodeIdx == 0 {
		return
	}
	if file.FlatType(nodeIdx) == "import_header" {
		text := strings.TrimSpace(file.FlatNodeText(nodeIdx))
		text = strings.TrimPrefix(text, "import ")
		text = strings.TrimSpace(text)

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

func extractPackageFlat(rootIdx uint32, file *scanner.File) string {
	return scanFileHeadersFlat(rootIdx, file).pkg
}
