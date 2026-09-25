package scanner

// tree-sitter-kotlin can parse a top-level @Name(oneExpression) as an
// expression statement when another declaration follows it. Normalize that
// shape once, before indexing or rule dispatch, so declaration readers see
// the same modifiers tree as for correctly parsed annotations.
type annotationTreeNode struct {
	typ      string
	start    uint32
	end      uint32
	row      uint16
	col      uint16
	flags    uint8
	children []*annotationTreeNode
}

func normalizeTopLevelAnnotations(t *FlatTree) *FlatTree {
	if t == nil || len(t.Types) == 0 || nodeTypeName(t.Types[0]) != "source_file" {
		return t
	}
	potential := false
	for c := t.FirstChildren[0]; c != 0; c = t.NextSibs[c] {
		if nodeTypeName(t.Types[c]) == "prefix_expression" {
			potential = true
			break
		}
	}
	if !potential {
		return t
	}
	root := copyAnnotationTree(t, 0)
	changed := false
	for i := 0; i < len(root.children); i++ {
		prefix := root.children[i]
		if prefix.typ != "prefix_expression" {
			continue
		}
		annotations, decl, splitParen := chainedTopLevelAnnotations(prefix)
		decl, end := topLevelAnnotationTarget(root.children, i, decl)
		if decl == nil {
			continue
		}
		if len(annotations) == 0 {
			continue
		}
		if splitParen != nil {
			last := annotations[len(annotations)-1]
			last.children = append(last.children, splitParen)
		}
		for _, ann := range annotations {
			repairSplitAnnotation(ann)
		}
		var comments []*annotationTreeNode
		if end > i+1 {
			comments = root.children[i+1 : end-1]
		}
		attachTopLevelAnnotations(decl, annotations, comments)
		root.children = append(root.children[:i], append([]*annotationTreeNode{decl}, root.children[end:]...)...)
		changed = true
	}
	if !changed {
		return t
	}
	out := &FlatTree{}
	appendAnnotationTree(out, root, 0, false)
	out.buildNodesByType()
	return out
}

func topLevelAnnotationTarget(children []*annotationTreeNode, prefixIdx int, nested *annotationTreeNode) (*annotationTreeNode, int) {
	end := prefixIdx + 1
	if nested != nil {
		if nested.typ == "anonymous_function" && repairTopLevelAnonymousFunction(nested) {
			return nested, end
		}
		return nil, end
	}
	for end < len(children) && (children[end].typ == "multiline_comment" || children[end].typ == "line_comment") {
		end++
	}
	if end == len(children) || !isTopLevelAnnotationDeclaration(children[end].typ) {
		return nil, end
	}
	return children[end], end + 1
}

func copyAnnotationTree(t *FlatTree, idx uint32) *annotationTreeNode {
	n := &annotationTreeNode{typ: nodeTypeName(t.Types[idx]), start: t.StartBytes[idx], end: t.EndBytes[idx], row: t.StartRows[idx], col: t.StartCols[idx], flags: t.Flags[idx]}
	for c := t.FirstChildren[idx]; c != 0; c = t.NextSibs[c] {
		n.children = append(n.children, copyAnnotationTree(t, c))
	}
	return n
}

func chainedTopLevelAnnotations(prefix *annotationTreeNode) ([]*annotationTreeNode, *annotationTreeNode, *annotationTreeNode) {
	var annotations []*annotationTreeNode
	for prefix != nil && prefix.typ == "prefix_expression" && len(prefix.children) == 2 && prefix.children[0].typ == "annotation" {
		ann := prefix.children[0]
		operand := prefix.children[1]
		if operand.typ != "parenthesized_expression" && operand.typ != "prefix_expression" && operand.typ != "anonymous_function" {
			return nil, nil, nil
		}
		if operand.typ == "parenthesized_expression" {
			if len(ann.children) == 0 || ann.children[len(ann.children)-1].typ != "user_type" || len(operand.children) != 3 || operand.children[0].typ != "(" || operand.children[2].typ != ")" {
				return nil, nil, nil
			}
		}
		annotations = append(annotations, ann)
		switch operand.typ {
		case "prefix_expression":
			prefix = operand
		case "anonymous_function":
			return annotations, operand, nil
		default:
			return annotations, nil, operand
		}
	}
	return nil, nil, nil
}

func isTopLevelAnnotationDeclaration(typ string) bool {
	switch typ {
	case "function_declaration", "class_declaration", "object_declaration", "property_declaration", "type_alias":
		return true
	}
	return false
}

// repairTopLevelAnonymousFunction handles only plain fun name( recovery.
// Receiver forms like `fun Foo.name(` and type-parameter forms like
// `fun <T> name(` are not repaired when nested; they have no target match.
func repairTopLevelAnonymousFunction(decl *annotationTreeNode) bool {
	if len(decl.children) < 2 || decl.children[0].typ != "fun" || decl.children[1].typ != "ERROR" || len(decl.children[1].children) != 1 || decl.children[1].children[0].typ != "simple_identifier" {
		return false
	}
	decl.typ = "function_declaration"
	decl.children[1] = decl.children[1].children[0]
	return true
}

func repairSplitAnnotation(ann *annotationTreeNode) {
	if len(ann.children) < 3 || ann.children[len(ann.children)-1].typ != "parenthesized_expression" {
		return
	}
	paren := ann.children[len(ann.children)-1]
	userType := ann.children[len(ann.children)-2]
	if userType.typ != "user_type" || len(paren.children) < 3 {
		return
	}
	value := paren.children[1]
	arg := &annotationTreeNode{typ: "value_argument", start: value.start, end: value.end, row: value.row, col: value.col, flags: flatNodeFlagNamed, children: []*annotationTreeNode{value}}
	paren.typ = "value_arguments"
	paren.children[1] = arg
	call := &annotationTreeNode{typ: "constructor_invocation", start: userType.start, end: paren.end, row: userType.row, col: userType.col, flags: flatNodeFlagNamed, children: []*annotationTreeNode{userType, paren}}
	ann.children = append(ann.children[:len(ann.children)-2], call)
	ann.end = call.end
}

func attachTopLevelAnnotations(decl *annotationTreeNode, annotations, comments []*annotationTreeNode) {
	first := annotations[0]
	children := append(append([]*annotationTreeNode{}, annotations...), comments...)
	if len(decl.children) > 0 && decl.children[0].typ == "modifiers" {
		mods := decl.children[0]
		mods.children = append(children, mods.children...)
		mods.start, mods.row, mods.col = first.start, first.row, first.col
	} else {
		last := children[len(children)-1]
		mods := &annotationTreeNode{typ: "modifiers", start: first.start, end: last.end, row: first.row, col: first.col, flags: flatNodeFlagNamed, children: children}
		decl.children = append([]*annotationTreeNode{mods}, decl.children...)
	}
	decl.start, decl.row, decl.col = first.start, first.row, first.col
}

func appendAnnotationTree(t *FlatTree, n *annotationTreeNode, parent uint32, hasParent bool) uint32 {
	idx := uint32(len(t.Types))
	p := uint32(0)
	if hasParent {
		p = parent
	}
	t.Types = append(t.Types, internNodeType(n.typ))
	t.Parents = append(t.Parents, p)
	t.FirstChildren = append(t.FirstChildren, 0)
	t.NextSibs = append(t.NextSibs, 0)
	t.PrevSibs = append(t.PrevSibs, 0)
	t.StartBytes = append(t.StartBytes, n.start)
	t.EndBytes = append(t.EndBytes, n.end)
	t.StartRows = append(t.StartRows, n.row)
	t.StartCols = append(t.StartCols, n.col)
	t.ChildCounts = append(t.ChildCounts, saturateUint16(uint32(len(n.children))))
	named := uint32(0)
	flags := n.flags &^ flatNodeFlagError
	if n.flags&flatNodeFlagIsError != 0 {
		flags |= flatNodeFlagError
	}
	for _, child := range n.children {
		if child.flags&flatNodeFlagNamed != 0 {
			named++
		}
	}
	t.NamedCounts = append(t.NamedCounts, saturateUint16(named))
	t.Flags = append(t.Flags, flags)
	var prev uint32
	for _, child := range n.children {
		c := appendAnnotationTree(t, child, idx, true)
		if t.FirstChildren[idx] == 0 {
			t.FirstChildren[idx] = c
		}
		if prev != 0 {
			t.NextSibs[prev] = c
			t.PrevSibs[c] = prev
		}
		prev = c
		if t.Flags[c]&flatNodeFlagError != 0 {
			t.Flags[idx] |= flatNodeFlagError
		}
	}
	return idx
}
