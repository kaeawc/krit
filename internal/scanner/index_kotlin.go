package scanner

import (
	"strconv"
	"strings"
)

func indexFile(file *File) ([]Symbol, []Reference) {
	var symbols []Symbol
	var references []Reference

	if file == nil || file.FlatTree == nil || len(file.FlatTree.Nodes) == 0 {
		return symbols, references
	}

	collectDeclarationsFlat(file, &symbols)
	collectReferencesFlat(file, &references)

	return symbols, references
}

func collectDeclarationsFlat(file *File, symbols *[]Symbol) {
	pkg := packageNameForFile(file)
	file.FlatWalkAllNodes(0, func(idx uint32) {
		nodeType := file.FlatType(idx)
		switch nodeType {
		case "function_declaration":
			name := file.FlatChildTextOrEmpty(idx, "simple_identifier")
			if name == "" {
				return
			}
			name = internString(name)
			owner := symbolOwner(file, idx, pkg)
			fqn := symbolFQN(pkg, owner, name)
			sym := Symbol{
				Name:       name,
				Kind:       "function",
				Visibility: flatVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				Language:   file.Language,
				Package:    pkg,
				FQN:        fqn,
				Owner:      owner,
				Signature:  symbolSignature(owner, name, kotlinFunctionArity(file, idx)),
				Arity:      kotlinFunctionArity(file, idx),
				IsOverride: file.FlatHasModifier(idx, "override"),
				IsMain:     name == "main",
			}
			sym.IsTest = strings.Contains(file.FlatNodeText(idx), "@Test")
			*symbols = append(*symbols, sym)
		case "class_declaration":
			name := file.FlatChildTextOrEmpty(idx, "type_identifier")
			if name == "" {
				name = file.FlatChildTextOrEmpty(idx, "simple_identifier")
			}
			if name == "" {
				return
			}
			name = internString(name)
			owner := symbolOwner(file, idx, pkg)
			kind := "class"
			text := file.FlatNodeText(idx)
			if strings.Contains(text, "interface ") {
				kind = "interface"
			}
			fqn := symbolFQN(pkg, owner, name)
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       kind,
				Visibility: flatVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				Language:   file.Language,
				Package:    pkg,
				FQN:        fqn,
				Owner:      owner,
				Signature:  fqn,
			})
		case "object_declaration":
			name := file.FlatChildTextOrEmpty(idx, "type_identifier")
			if name == "" {
				name = file.FlatChildTextOrEmpty(idx, "simple_identifier")
			}
			if name == "" {
				return
			}
			name = internString(name)
			owner := symbolOwner(file, idx, pkg)
			fqn := symbolFQN(pkg, owner, name)
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       "object",
				Visibility: flatVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				Language:   file.Language,
				Package:    pkg,
				FQN:        fqn,
				Owner:      owner,
				Signature:  fqn,
			})
		case "property_declaration":
			parent, ok := file.FlatParent(idx)
			if !ok {
				return
			}
			parentType := file.FlatType(parent)
			if parentType != "source_file" && parentType != "class_body" &&
				(parentType != "statements" || !hasFlatAncestorType(file, parent, "class_body")) {
				return
			}
			name := file.FlatChildTextOrEmpty(idx, "simple_identifier")
			if name == "" {
				if varDecl, ok := file.FlatFindChild(idx, "variable_declaration"); ok {
					name = file.FlatChildTextOrEmpty(varDecl, "simple_identifier")
				}
			}
			if name == "" || name == "_" {
				return
			}
			name = internString(name)
			owner := symbolOwner(file, idx, pkg)
			fqn := symbolFQN(pkg, owner, name)
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       "property",
				Visibility: flatVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				Language:   file.Language,
				Package:    pkg,
				FQN:        fqn,
				Owner:      owner,
				Signature:  symbolSignature(owner, name, 0),
				Arity:      0,
				IsOverride: file.FlatHasModifier(idx, "override"),
			})
		}
	})
}

func collectReferencesFlat(file *File, refs *[]Reference) {
	simpleID, hasSimpleID := lookupNodeType("simple_identifier")
	typeID, hasTypeID := lookupNodeType("type_identifier")
	if !hasSimpleID && !hasTypeID {
		return
	}
	commentTypes := make([]uint16, 0, 2)
	if lineCommentID, ok := lookupNodeType("line_comment"); ok {
		commentTypes = append(commentTypes, lineCommentID)
	}
	if multilineCommentID, ok := lookupNodeType("multiline_comment"); ok {
		commentTypes = append(commentTypes, multilineCommentID)
	}

	file.FlatWalkAllNodes(0, func(idx uint32) {
		nodeType := file.FlatTree.Nodes[idx].Type
		if (!hasSimpleID || nodeType != simpleID) && (!hasTypeID || nodeType != typeID) {
			return
		}
		name := file.FlatNodeText(idx)
		if name == "" {
			return
		}
		*refs = append(*refs, Reference{
			Name:      name,
			File:      file.Path,
			Line:      file.FlatRow(idx) + 1,
			InComment: file.FlatHasAnyAncestorOfType(idx, commentTypes...),
			StartByte: int(file.FlatStartByte(idx)),
			EndByte:   int(file.FlatEndByte(idx)),
			Language:  file.Language,
		})
	})
}

func flatVisibility(file *File, idx uint32) string {
	switch {
	case file.FlatHasModifier(idx, "private"):
		return "private"
	case file.FlatHasModifier(idx, "internal"):
		return "internal"
	case file.FlatHasModifier(idx, "protected"):
		return "protected"
	default:
		return "public"
	}
}

func packageNameForFile(file *File) string {
	if file == nil {
		return ""
	}
	if file.Language == LangJava {
		return javaPackageName(file)
	}
	return kotlinPackageName(file)
}

// parsePackageHeaderText extracts a clean dotted package name from a
// `package_header` node's text. Tree-sitter Kotlin sometimes attaches
// trailing comments and whitespace to the package_header node, so we
// take only the first non-empty, non-comment line and strip the leading
// `package` keyword.
func parsePackageHeaderText(raw string) string {
	// Single-pass scan: walk the line-delimited string in place so we
	// don't allocate a slice covering every line for the typical
	// one-or-two-line header. Mirrors firstSourceLine in rename;
	// same shape — see #46.
	for s := raw; ; {
		var line string
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			line = strings.TrimSpace(s)
		} else {
			line = strings.TrimSpace(s[:i])
		}
		if line != "" && !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "/*") {
			line = strings.TrimPrefix(line, "package")
			line = strings.TrimSpace(line)
			line = strings.TrimSuffix(line, ";")
			return strings.TrimSpace(line)
		}
		if i < 0 {
			return ""
		}
		s = s[i+1:]
	}
}

func kotlinPackageName(file *File) string {
	if file == nil || file.FlatTree == nil {
		return ""
	}
	var pkg string
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if pkg != "" || file.FlatType(idx) != "package_header" {
			return
		}
		pkg = parsePackageHeaderText(file.FlatNodeText(idx))
	})
	return internString(pkg)
}

func symbolOwner(file *File, idx uint32, pkg string) string {
	var names []string
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "annotation_type_declaration":
			if name := file.FlatChildTextOrEmpty(parent, "identifier"); name != "" {
				names = append(names, name)
			} else if name := file.FlatChildTextOrEmpty(parent, "type_identifier"); name != "" {
				names = append(names, name)
			} else if name := file.FlatChildTextOrEmpty(parent, "simple_identifier"); name != "" {
				names = append(names, name)
			}
		case "object_declaration":
			if name := file.FlatChildTextOrEmpty(parent, "type_identifier"); name != "" {
				names = append(names, name)
			} else if name := file.FlatChildTextOrEmpty(parent, "simple_identifier"); name != "" {
				names = append(names, name)
			}
		}
	}
	if len(names) == 0 {
		return ""
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	owner := strings.Join(names, ".")
	if pkg != "" {
		owner = pkg + "." + owner
	}
	return internString(owner)
}

func symbolFQN(pkg, owner, name string) string {
	if name == "" {
		return ""
	}
	if owner != "" {
		return internString(owner + "." + name)
	}
	if pkg != "" {
		return internString(pkg + "." + name)
	}
	return name
}

func symbolSignature(owner, name string, arity int) string {
	prefix := owner
	if prefix == "" {
		prefix = "<package>"
	}
	return internString(prefix + "#" + name + "/" + strconv.Itoa(arity))
}

func kotlinFunctionArity(file *File, idx uint32) int {
	params, ok := file.FlatFindChild(idx, "function_value_parameters")
	if !ok {
		return 0
	}
	return countNamedChildrenOfType(file, params, "parameter")
}

func countNamedChildrenOfType(file *File, parent uint32, nodeTypes ...string) int {
	want := make(map[string]bool, len(nodeTypes))
	for _, typ := range nodeTypes {
		want[typ] = true
	}
	count := 0
	for i := 0; i < file.FlatNamedChildCount(parent); i++ {
		child := file.FlatNamedChild(parent, i)
		if want[file.FlatType(child)] {
			count++
		}
	}
	return count
}

func hasFlatAncestorType(file *File, idx uint32, want string) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		if file.FlatType(current) == want {
			return true
		}
	}
	return false
}
