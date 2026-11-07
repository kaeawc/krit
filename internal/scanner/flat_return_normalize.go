package scanner

// tree-sitter-kotlin parses `return e...` (an operand whose first letter is a
// lowercase "e") as a bare `return` jump_expression followed by the operand as
// its next sibling. Reattach the operand so every rule sees one jump_expression
// that owns its value, as it does for any other first letter.
//
// Nodes keep their pre-order positions, so only links and counts change.
func normalizeBareReturnOperands(t *FlatTree) *FlatTree {
	if t == nil || len(t.Types) == 0 {
		return t
	}
	jump := internNodeType("jump_expression")
	for _, j := range t.NodesOfType(jump) {
		ret := t.FirstChildren[j]
		if ret == 0 || t.NextSibs[ret] != 0 || t.EndBytes[j]-t.StartBytes[j] != 6 {
			continue
		}
		operand := t.NextSibs[j]
		if operand == 0 || !t.NodeIsNamed(operand) || t.Parents[operand] != t.Parents[j] {
			continue
		}
		switch nodeTypeName(t.Types[operand]) {
		case "line_comment", "multiline_comment":
			continue
		}
		// A value on the next line belongs to the next statement.
		if t.StartRows[operand] != t.StartRows[j] || t.StartBytes[operand] < t.EndBytes[j] {
			continue
		}
		next := t.NextSibs[operand]
		parent := t.Parents[j]
		t.NextSibs[ret] = operand
		t.PrevSibs[operand] = ret
		t.Parents[operand] = j
		t.NextSibs[j] = next
		if next != 0 {
			t.PrevSibs[next] = j
		}
		t.NextSibs[operand] = 0
		t.EndBytes[j] = t.EndBytes[operand]
		t.ChildCounts[j] = 2
		t.NamedCounts[j] = 1
		if t.ChildCounts[parent] > 0 {
			t.ChildCounts[parent]--
		}
		if t.NamedCounts[parent] > 0 {
			t.NamedCounts[parent]--
		}
	}
	return t
}
