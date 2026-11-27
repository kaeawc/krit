package scanner

import (
	"context"
	"slices"
	"sort"

	sitter "github.com/smacker/go-tree-sitter"
)

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
//
// The cause is the grammar's external scanner: it never emits an automatic
// semicolon before a '}' on the same line, and a class member needs one. When
// that body is nested in another body,
//
//	class H { companion object { val X = 1 } }
//
// recovery does not stay local. The inner ERROR takes the inner '}', the outer
// '}' closes the inner body, and the outer class plus every declaration after
// it in the file end up in one ERROR node. Every finding in the file is then
// dropped by the parse-error-region filter (#1839). When a ';' and more
// members follow the inner body,
//
//	class H { companion object { val X = 1 }; val y = 2 }
//
// the inner '}' sits alone in an ERROR node and the member it closes is the
// node before that ERROR (#1875). Unwrapping cannot fix either tree, so
// closeSameLineBodies reparses with a ';' before such a brace: in the space
// before it, or, when code ends right at the brace as in `{ val X = 1}}`,
// inserted into the reparsed buffer and mapped back out of the tree (#1866).
// The scan is not limited to class bodies: any '}' token in an ERROR node
// qualifies, and the error-extent check decides whether the reparse is kept.

// maxSameLineBodyReparses bounds the reparses of one file. One round usually
// fixes every brace; later rounds run when recovery hid a brace the first
// tree did not expose.
const maxSameLineBodyReparses = 3

// closeSameLineBodies reparses src while the tree has a '}' token inside an
// ERROR node right after a named node or another '}' on the same line, with a
// ';' before that brace (see sameLineBodyCloseSemis). A ';' replaces the space
// before the brace when there is one and is inserted otherwise. Each round
// builds on the last one, because an intermediate tree can be no better than
// the first. The best tree is kept only when it shrinks the error region, so
// real syntax errors and clean trees keep the first parse.
//
// The returned tree has the offsets, rows and columns of content: inserted
// bytes are mapped out of it (dropInsertedBytes). It also returns the source
// the tree matches, which has the replaced spaces but no inserted bytes, and
// the offsets of the replaced spaces for trimInsertedSemicolons.
func closeSameLineBodies(ctx context.Context, parser *sitter.Parser, ft *FlatTree, src, content []byte) (*FlatTree, []byte, []uint32) {
	best, bestSrc := ft, src
	var bestSemis, semis, inserts []uint32
	bestInserts := 0
	for round := 0; round < maxSameLineBodyReparses; round++ {
		spaces, braces := sameLineBodyCloseSemis(ft, src)
		braces = newInserts(inserts, braces)
		if len(spaces) == 0 && len(braces) == 0 {
			break
		}
		next := make([]byte, len(src))
		copy(next, src)
		for _, p := range spaces {
			next[p] = ';'
		}
		inserts = mergeInserts(inserts, braces)
		tree, err := parser.ParseCtx(ctx, nil, withInsertedSemicolons(next, inserts))
		if err != nil {
			break
		}
		flat := flattenTree(tree.RootNode())
		tree.Close()
		dropInsertedBytes(flat, inserts)
		ft = normalizeExplicitBackingFields(flat, content)
		src = next
		semis = append(semis, spaces...)
		if errorExtentLess(ft, best) {
			best, bestSrc, bestSemis, bestInserts = ft, src, append([]uint32(nil), semis...), len(inserts)
			if covered, nodes := errorExtent(best); covered == 0 && nodes == 0 {
				break
			}
		}
	}
	if bestInserts > 0 {
		// Rows are unchanged, since no inserted byte is a line break, but
		// columns after an inserted byte on its line are one too far.
		restorePositions(best, content)
	}
	return best, bestSrc, bestSemis
}

// sameLineBodyCloseSemis finds each '}' token that an ERROR node, in any
// context, holds right after a named node or another '}' on the same line.
// The node before the ERROR counts for its first child, because a '}' can sit
// alone in an ERROR whose member is its previous sibling (#1875). It returns
// the offsets of the spaces before such braces and, for braces that code
// precedes directly, the offsets of the braces themselves (#1866).
func sameLineBodyCloseSemis(ft *FlatTree, src []byte) (spaces, braces []uint32) {
	if ft == nil {
		return nil, nil
	}
	for idx, flags := range ft.Flags {
		if flags&flatNodeFlagIsError == 0 || ft.FirstChildren[idx] == 0 {
			continue
		}
		afterMember := isNamedNonError(ft, ft.PrevSibs[idx])
		for c := ft.FirstChildren[idx]; c != 0; c = ft.NextSibs[c] {
			isBrace := nodeTypeName(ft.Types[c]) == "}"
			if afterMember && isBrace {
				b := ft.StartBytes[c]
				if p, ok := spaceBeforeSameLineBrace(src, b); ok {
					spaces = append(spaces, p)
				} else if codeRightBeforeBrace(src, b) {
					braces = append(braces, b)
				}
			}
			afterMember = isBrace || isNamedNonError(ft, c)
		}
	}
	return spaces, braces
}

// isNamedNonError reports whether idx is a named node other than ERROR or
// MISSING. Index 0 is the root, which is never a sibling, so it means none.
func isNamedNonError(ft *FlatTree, idx uint32) bool {
	return idx != 0 && ft.Flags[idx]&flatNodeFlagNamed != 0 && ft.Flags[idx]&flatNodeFlagIsError == 0
}

// spaceBeforeSameLineBrace returns the offset of the space or tab right
// before the '}' at brace when code, not a line break, '{' or ';', precedes
// it on the same line.
func spaceBeforeSameLineBrace(src []byte, brace uint32) (uint32, bool) {
	if brace == 0 || int(brace) > len(src) || (src[brace-1] != ' ' && src[brace-1] != '\t') {
		return 0, false
	}
	i := int(brace) - 1
	for i >= 0 && (src[i] == ' ' || src[i] == '\t') {
		i--
	}
	if i < 0 {
		return 0, false
	}
	switch src[i] {
	case '\n', '\r', '{', ';':
		return 0, false
	}
	return brace - 1, true
}

// codeRightBeforeBrace reports whether the byte right before the '}' at brace
// ends code: anything but whitespace, '{' or ';'.
func codeRightBeforeBrace(src []byte, brace uint32) bool {
	if brace == 0 || int(brace) > len(src) {
		return false
	}
	switch src[brace-1] {
	case ' ', '\t', '\n', '\r', '\f', '{', ';':
		return false
	}
	return true
}

// newInserts returns the offsets in found that are not in have, sorted and
// without duplicates. have is sorted.
func newInserts(have, found []uint32) []uint32 {
	var out []uint32
	for _, b := range found {
		i := sort.Search(len(have), func(k int) bool { return have[k] >= b })
		if i < len(have) && have[i] == b {
			continue
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return slices.Compact(out)
}

// mergeInserts returns the sorted union of two sorted offset lists that share
// no offset.
func mergeInserts(a, b []uint32) []uint32 {
	if len(b) == 0 {
		return a
	}
	out := make([]uint32, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// withInsertedSemicolons returns src with a ';' put before each offset in
// inserts, which is sorted. It returns src itself when inserts is empty.
func withInsertedSemicolons(src []byte, inserts []uint32) []byte {
	if len(inserts) == 0 {
		return src
	}
	out := make([]byte, 0, len(src)+len(inserts))
	prev := uint32(0)
	for _, p := range inserts {
		out = append(out, src[prev:p]...)
		out = append(out, ';')
		prev = p
	}
	return append(out, src[prev:]...)
}

// dropInsertedBytes maps the byte offsets of a tree parsed from
// withInsertedSemicolons(src, inserts) back to offsets in src. An inserted
// ';' token becomes a zero-width node at the brace it was put before, and a
// node ending after that ';' ends before it. Rows and columns are not
// updated (see restorePositions).
func dropInsertedBytes(ft *FlatTree, inserts []uint32) {
	if len(inserts) == 0 {
		return
	}
	// The k-th inserted byte sits at inserts[k]+k in the parsed buffer, so an
	// offset n there follows the inserted bytes at positions below n.
	back := func(n uint32) uint32 {
		return n - uint32(sort.Search(len(inserts), func(k int) bool { return inserts[k]+uint32(k) >= n }))
	}
	for i := range ft.StartBytes {
		ft.StartBytes[i] = back(ft.StartBytes[i])
		ft.EndBytes[i] = back(ft.EndBytes[i])
	}
}

// errorExtentLess reports whether a has fewer bytes under ERROR or MISSING
// nodes than b, or as many bytes in fewer such nodes.
func errorExtentLess(a, b *FlatTree) bool {
	ab, an := errorExtent(a)
	bb, bn := errorExtent(b)
	return ab < bb || ab == bb && an < bn
}

// errorExtent returns the number of bytes covered by ERROR or MISSING nodes
// and the number of such nodes. Nodes come in preorder, so a nested error
// span starts inside an earlier one and only its uncovered tail counts.
func errorExtent(ft *FlatTree) (covered uint32, nodes int) {
	var end uint32
	for idx, flags := range ft.Flags {
		if flags&flatNodeFlagIsError == 0 {
			continue
		}
		nodes++
		s, e := ft.StartBytes[idx], ft.EndBytes[idx]
		if s < end {
			s = end
		}
		if e > s {
			covered += e - s
			end = e
		}
	}
	return covered, nodes
}

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
