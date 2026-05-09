package scanner

import (
	"strings"
)

func collectJavaDeclarationsFlat(file *File, symbols *[]Symbol) {
	if file == nil || file.FlatTree == nil {
		return
	}
	pkg := javaPackageName(file)
	file.FlatWalkAllNodes(0, func(idx uint32) {
		switch file.FlatType(idx) {
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "annotation_type_declaration":
			name := file.FlatChildTextOrEmpty(idx, "identifier")
			if name == "" {
				return
			}
			name = internString(name)
			owner := symbolOwner(file, idx, pkg)
			fqn := symbolFQN(pkg, owner, name)
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       javaClassKind(file.FlatType(idx)),
				Visibility: javaVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				Language:   LangJava,
				Package:    pkg,
				FQN:        fqn,
				Owner:      owner,
				Signature:  fqn,
				IsTest:     javaDeclarationHasTestAnnotation(file, idx),
				IsStatic:   file.FlatHasModifier(idx, "static"),
				IsFinal:    file.FlatHasModifier(idx, "final"),
			})
		case "method_declaration":
			name := file.FlatChildTextOrEmpty(idx, "identifier")
			if name == "" {
				return
			}
			name = internString(name)
			owner := symbolOwner(file, idx, pkg)
			arity := javaFormalParameterArity(file, idx)
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       "method",
				Visibility: javaVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				Language:   LangJava,
				Package:    pkg,
				FQN:        symbolFQN(pkg, owner, name),
				Owner:      owner,
				Signature:  symbolSignature(owner, name, arity),
				Arity:      arity,
				IsTest:     javaDeclarationHasTestAnnotation(file, idx),
				IsStatic:   file.FlatHasModifier(idx, "static"),
				IsFinal:    file.FlatHasModifier(idx, "final"),
			})
		case "constructor_declaration", "compact_constructor_declaration":
			name := file.FlatChildTextOrEmpty(idx, "identifier")
			if name == "" {
				return
			}
			name = internString(name)
			owner := symbolOwner(file, idx, pkg)
			arity := javaFormalParameterArity(file, idx)
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       "constructor",
				Visibility: javaVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				Language:   LangJava,
				Package:    pkg,
				FQN:        symbolFQN(pkg, owner, name),
				Owner:      owner,
				Signature:  symbolSignature(owner, name, arity),
				Arity:      arity,
				IsTest:     javaDeclarationHasTestAnnotation(file, idx),
			})
		case "field_declaration":
			collectJavaFieldDeclarations(file, idx, pkg, symbols)
		}
	})
}

func javaVisibility(file *File, idx uint32) string {
	switch {
	case file.FlatHasModifier(idx, "private"):
		return "private"
	case file.FlatHasModifier(idx, "protected"):
		return "protected"
	case file.FlatHasModifier(idx, "public"):
		return "public"
	default:
		return "package"
	}
}

func javaClassKind(nodeType string) string {
	switch nodeType {
	case "interface_declaration":
		return "interface"
	case "enum_declaration":
		return "enum"
	case "record_declaration":
		return "record"
	case "annotation_type_declaration":
		return "annotation"
	default:
		return "class"
	}
}

func javaDeclarationHasTestAnnotation(file *File, idx uint32) bool {
	return strings.Contains(file.FlatNodeText(idx), "@Test")
}

func javaPackageName(file *File) string {
	if file == nil || file.FlatTree == nil {
		return ""
	}
	var pkg string
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if pkg != "" || file.FlatType(idx) != "package_declaration" {
			return
		}
		pkg = parsePackageHeaderText(file.FlatNodeText(idx))
	})
	return internString(pkg)
}

func javaFormalParameterArity(file *File, idx uint32) int {
	params, ok := file.FlatFindChild(idx, "formal_parameters")
	if !ok {
		return 0
	}
	return countNamedChildrenOfType(file, params, "formal_parameter", "spread_parameter")
}

func collectJavaFieldDeclarations(file *File, idx uint32, pkg string, symbols *[]Symbol) {
	owner := symbolOwner(file, idx, pkg)
	file.FlatWalkAllNodes(idx, func(child uint32) {
		if file.FlatType(child) != "variable_declarator" {
			return
		}
		name := file.FlatChildTextOrEmpty(child, "identifier")
		if name == "" {
			return
		}
		name = internString(name)
		*symbols = append(*symbols, Symbol{
			Name:       name,
			Kind:       "field",
			Visibility: javaVisibility(file, idx),
			File:       file.Path,
			Line:       file.FlatRow(child) + 1,
			StartByte:  int(file.FlatStartByte(child)),
			EndByte:    int(file.FlatEndByte(child)),
			Language:   LangJava,
			Package:    pkg,
			FQN:        symbolFQN(pkg, owner, name),
			Owner:      owner,
			Signature:  symbolSignature(owner, name, 0),
			Arity:      0,
			IsStatic:   file.FlatHasModifier(idx, "static"),
			IsFinal:    file.FlatHasModifier(idx, "final"),
		})
	})
}

func collectJavaReferencesFlat(file *File, refs *[]Reference) {
	if file == nil {
		return
	}
	if file.ReferencesPrecomputed {
		*refs = append(*refs, file.PrecomputedReferences...)
		return
	}
	collectJavaReferencesFlatUncached(file, refs)
}

func collectJavaReferencesFlatUncached(file *File, refs *[]Reference) {
	if file == nil || file.FlatTree == nil {
		return
	}
	identifierID, hasIdentifier := lookupNodeType("identifier")
	typeIdentifierID, hasTypeIdentifier := lookupNodeType("type_identifier")
	scopedIDs := lookupExistingNodeTypes("scoped_identifier", "scoped_type_identifier")
	if !hasIdentifier && !hasTypeIdentifier && len(scopedIDs) == 0 {
		return
	}
	nodes := file.FlatTree.Nodes
	for i := range nodes {
		node := nodes[i]
		isSimple := (hasIdentifier && node.Type == identifierID) || (hasTypeIdentifier && node.Type == typeIdentifierID)
		isScoped := hasNodeType(scopedIDs, node.Type)
		if !isSimple && !isScoped {
			continue
		}
		idx := uint32(i)
		name := FlatNodeText(file.FlatTree, idx, file.Content)
		if name == "" {
			continue
		}
		if isScoped && !strings.Contains(name, ".") {
			continue
		}
		*refs = append(*refs, Reference{
			Name:      name,
			File:      file.Path,
			Line:      int(node.StartRow) + 1,
			StartByte: int(node.StartByte),
			EndByte:   int(node.EndByte),
			Language:  LangJava,
		})
		if isSimple {
			if prop := javaAccessorPropertyName(name); prop != "" {
				*refs = append(*refs, Reference{
					Name:      prop,
					File:      file.Path,
					Line:      int(node.StartRow) + 1,
					StartByte: int(node.StartByte),
					EndByte:   int(node.EndByte),
					Language:  LangJava,
				})
			}
		}
	}
}

func javaAccessorPropertyName(name string) string {
	switch {
	case strings.HasPrefix(name, "get") && len(name) > len("get") && isASCIIUpper(name[len("get")]):
		return lowerASCIIInitial(name[len("get"):])
	case strings.HasPrefix(name, "set") && len(name) > len("set") && isASCIIUpper(name[len("set")]):
		return lowerASCIIInitial(name[len("set"):])
	case strings.HasPrefix(name, "is") && len(name) > len("is") && isASCIIUpper(name[len("is")]):
		return "is" + name[len("is"):]
	default:
		return ""
	}
}

func lowerASCIIInitial(value string) string {
	if value == "" {
		return ""
	}
	b := []byte(value)
	if b[0] >= 'A' && b[0] <= 'Z' {
		b[0] = b[0] - 'A' + 'a'
	}
	return string(b)
}

func isASCIIUpper(ch byte) bool {
	return ch >= 'A' && ch <= 'Z'
}

func lookupExistingNodeTypes(types ...string) []uint16 {
	out := make([]uint16, 0, len(types))
	for _, typ := range types {
		if id, ok := lookupNodeType(typ); ok {
			out = append(out, id)
		}
	}
	return out
}

func hasNodeType(types []uint16, typ uint16) bool {
	for _, candidate := range types {
		if candidate == typ {
			return true
		}
	}
	return false
}
