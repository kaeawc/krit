package scanner

import "strings"

// groovyNodeType returns the tree-sitter type at idx, or an empty type for an invalid index.
func groovyNodeType(tree *FlatTree, idx uint32) string {
	if tree == nil || int(idx) >= len(tree.Types) {
		return ""
	}
	return nodeTypeName(tree.Types[idx])
}

// groovyChildren returns a node's direct children in source order.
func groovyChildren(tree *FlatTree, idx uint32) []uint32 {
	if tree == nil || int(idx) >= len(tree.Types) {
		return nil
	}
	var out []uint32
	for child := tree.FirstChildren[idx]; child != 0; child = tree.NextSibs[child] {
		out = append(out, child)
	}
	return out
}

// groovyFindDescendant finds the first descendant with the requested tree-sitter type.
func groovyFindDescendant(tree *FlatTree, idx uint32, typ string) (uint32, bool) {
	for _, child := range groovyChildren(tree, idx) {
		if groovyNodeType(tree, child) == typ {
			return child, true
		}
		if found, ok := groovyFindDescendant(tree, child, typ); ok {
			return found, true
		}
	}
	return 0, false
}

// GroovyCallName extracts the callee from either parenthesized calls or Groovy command expressions.
func GroovyCallName(tree *FlatTree, idx uint32, content []byte) string {
	t := groovyNodeType(tree, idx)
	if t != "function_call" && t != "juxt_function_call" {
		return ""
	}
	for _, child := range groovyChildren(tree, idx) {
		ct := groovyNodeType(tree, child)
		if ct == "identifier" || ct == "dotted_identifier" {
			return FlatNodeText(tree, child, content)
		}
	}
	return ""
}

// GroovyCallArgs finds the argument list shared by parenthesized and command-form calls.
func GroovyCallArgs(tree *FlatTree, idx uint32) (uint32, bool) {
	if groovyNodeType(tree, idx) != "function_call" && groovyNodeType(tree, idx) != "juxt_function_call" {
		return 0, false
	}
	return FlatFindChild(tree, idx, "argument_list")
}

// GroovyCallClosure finds closures nested in arguments or adjacent to a call, as Groovy permits both forms.
func GroovyCallClosure(tree *FlatTree, idx uint32, content []byte) (uint32, bool) {
	if tree == nil || (groovyNodeType(tree, idx) != "function_call" && groovyNodeType(tree, idx) != "juxt_function_call") || int(idx) >= len(tree.NextSibs) {
		return 0, false
	}
	if args, ok := GroovyCallArgs(tree, idx); ok {
		if closure, found := FlatFindChild(tree, args, "closure"); found {
			return closure, true
		}
	}
	sib := tree.NextSibs[idx]
	if sib != 0 && groovyNodeType(tree, sib) == "closure" && groovyAdjacent(tree, idx, sib, content) {
		return sib, true
	}
	return 0, false
}

// GroovyCommandChain groups whitespace-separated command calls while stopping at Groovy statement boundaries.
func GroovyCommandChain(tree *FlatTree, idx uint32, content []byte) []uint32 {
	if groovyNodeType(tree, idx) != "juxt_function_call" || int(idx) >= len(tree.NextSibs) {
		return []uint32{idx}
	}
	chain := []uint32{idx}
	prev := idx
	for sib := tree.NextSibs[prev]; sib != 0 && groovyNodeType(tree, sib) == "juxt_function_call"; sib = tree.NextSibs[sib] {
		if !groovyAdjacent(tree, prev, sib, content) {
			break
		}
		chain = append(chain, sib)
		prev = sib
	}
	return chain
}

func groovyAdjacent(tree *FlatTree, prev, sib uint32, content []byte) bool {
	if tree == nil || int(prev) >= len(tree.EndBytes) || int(sib) >= len(tree.StartBytes) {
		return false
	}
	start, end := tree.EndBytes[prev], tree.StartBytes[sib]
	if start > end || uint64(end) > uint64(len(content)) {
		return false
	}
	for _, b := range content[start:end] {
		if b != ' ' && b != '\t' {
			return false
		}
	}
	return true
}

// GroovyStringLiteral extracts non-interpolated strings without interpreting Groovy escape sequences.
func GroovyStringLiteral(tree *FlatTree, idx uint32, content []byte) (string, bool) {
	if groovyNodeType(tree, idx) != "string" {
		return "", false
	}
	raw := FlatNodeText(tree, idx, content)
	if len(raw) < 2 {
		return "", false
	}
	quote := raw[0]
	if quote != '\'' && quote != '"' {
		return "", false
	}
	width := 1
	if len(raw) >= 6 && raw[:3] == strings.Repeat(string(quote), 3) {
		width = 3
	}
	if len(raw) < width*2 || raw[len(raw)-width:] != strings.Repeat(string(quote), width) {
		return "", false
	}
	value := raw[width : len(raw)-width]
	if strings.ContainsAny(value, "\\") || (quote == '"' && strings.Contains(value, "$")) {
		return "", false
	}
	return value, true
}
