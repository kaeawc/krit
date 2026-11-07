package scanner

// The grammar wraps the last member of a class, object or interface body in an
// ERROR node when the closing brace follows it on the same line:
//
//	class A { val x = 1 }
//	object S { fun f() {} }
//	interface X { fun f() }
//
// The member itself parses as a complete declaration. unwrapClassBodyErrors
// replaces such an ERROR node by its declarations, so rules see the same body
// as for a member followed by a line break. Offsets, rows and columns are
// unchanged. An ERROR node that holds anything else is left alone.

// classBodyErrorMembers are the node types an unwrapped ERROR node may hold.
var classBodyErrorMembers = map[string]bool{
	"property_declaration": true, "function_declaration": true, "class_declaration": true,
	"object_declaration": true, "secondary_constructor": true, "companion_object": true,
	"anonymous_initializer": true, "type_alias": true, "enum_entry": true,
	"line_comment": true, "multiline_comment": true,
}

func unwrapClassBodyErrors(ft *FlatTree) *FlatTree {
	if ft == nil || len(ft.Types) == 0 {
		return ft
	}
	found := false
	for i, flags := range ft.Flags {
		if flags&flatNodeFlagIsError != 0 && isClassBodyError(ft, uint32(i)) {
			found = true
			break
		}
	}
	if !found {
		return ft
	}
	root := copyAnnotationTree(ft, 0)
	if !spliceClassBodyErrors(root) {
		return ft
	}
	out := &FlatTree{}
	appendAnnotationTree(out, root, 0, false)
	out.buildNodesByType()
	return out
}

func isClassBodyError(ft *FlatTree, idx uint32) bool {
	if ft.FirstChildren[idx] == 0 || idx == 0 {
		return false
	}
	switch nodeTypeName(ft.Types[ft.Parents[idx]]) {
	case "class_body", "enum_class_body":
	default:
		return false
	}
	declarations := 0
	for c := ft.FirstChildren[idx]; c != 0; c = ft.NextSibs[c] {
		typ := nodeTypeName(ft.Types[c])
		if !classBodyErrorMembers[typ] || ft.Flags[c]&(flatNodeFlagError|flatNodeFlagIsError) != 0 {
			return false
		}
		if typ != "line_comment" && typ != "multiline_comment" {
			declarations++
		}
	}
	return declarations > 0
}

func spliceClassBodyErrors(n *annotationTreeNode) bool {
	changed := false
	var children []*annotationTreeNode
	for _, c := range n.children {
		if spliceClassBodyErrors(c) {
			changed = true
		}
		if (n.typ == "class_body" || n.typ == "enum_class_body") && c.typ == "ERROR" && c.flags&flatNodeFlagIsError != 0 && onlyClassMembers(c) {
			children = append(children, c.children...)
			changed = true
			continue
		}
		children = append(children, c)
	}
	n.children = children
	return changed
}

func onlyClassMembers(n *annotationTreeNode) bool {
	declarations := 0
	for _, c := range n.children {
		if !classBodyErrorMembers[c.typ] || c.flags&flatNodeFlagIsError != 0 {
			return false
		}
		if c.typ != "line_comment" && c.typ != "multiline_comment" {
			declarations++
		}
	}
	return declarations > 0
}
