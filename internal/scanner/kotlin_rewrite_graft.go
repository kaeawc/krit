package scanner

import (
	"bytes"
	"context"

	sitter "github.com/smacker/go-tree-sitter"
)

// repairTree restores what the rewrites hid: widened declarations and the
// `by` clauses of delegated supertypes.
func (r *kotlinRewriter) repairTree(ctx context.Context, parser *sitter.Parser, ft *FlatTree) *FlatTree {
	hasContext := false
	for _, p := range r.prefixes {
		hasContext = hasContext || p.context
	}
	if len(r.delegations) > 0 || len(r.parenFixes) > 0 || hasContext {
		root := copyAnnotationTree(ft, 0)
		changed := graftDelegations(ctx, parser, root, r.orig, r.delegations)
		if unwrapAssignmentTargets(root, r.parenFixes) {
			changed = true
		}
		if graftContextParameters(ctx, parser, root, r.orig, r.prefixes) {
			changed = true
		}
		if changed {
			out := &FlatTree{}
			appendAnnotationTree(out, root, 0, false)
			out.buildNodesByType()
			ft = out
		}
	}
	extendDeclPrefixes(ft, r.prefixes)
	trimInsertedSemicolons(ft, r.semis)
	return ft
}

func isPrefixTarget(typ string) bool {
	switch typ {
	case "class_declaration", "object_declaration", "function_declaration", "property_declaration", "type_alias", "secondary_constructor":
		return true
	}
	return false
}

// extendDeclPrefixes moves the start of each declaration back over the
// blanked prefix that belongs to it.
func extendDeclPrefixes(ft *FlatTree, prefixes []declPrefix) {
	for _, p := range prefixes {
		if p.context && p.grafted {
			continue
		}
		for i := 1; i < len(ft.Types); i++ {
			if ft.StartBytes[i] != p.declStart || !isPrefixTarget(nodeTypeName(ft.Types[i])) {
				continue
			}
			for n := uint32(i); ft.StartBytes[n] == p.declStart; n = ft.Parents[n] {
				ft.StartBytes[n] = p.start
				if n == 0 {
					break
				}
			}
			break
		}
	}
}

// graftDelegations rebuilds explicit_delegation nodes for blanked `by` clauses.
func graftDelegations(ctx context.Context, parser *sitter.Parser, root *annotationTreeNode, orig []byte, reps []delegationRepair) bool {
	changed := false
	for _, d := range reps {
		x, path := findDelegatedType(root, nil, d.typeEnd)
		if x == nil {
			continue
		}
		expr := parseDelegateExpression(ctx, parser, orig, d)
		if expr == nil {
			continue
		}
		parent := path[len(path)-1]
		by := &annotationTreeNode{typ: "by", start: d.byStart, end: d.byStart + 2}
		e := &annotationTreeNode{typ: "explicit_delegation", start: x.start, end: d.exprEnd, row: x.row, col: x.col, flags: flatNodeFlagNamed, children: []*annotationTreeNode{x, by, expr}}
		for i, c := range parent.children {
			if c == x {
				parent.children[i] = e
			}
		}
		for _, a := range path {
			if a.end < d.exprEnd {
				a.end = d.exprEnd
			}
		}
		changed = true
	}
	return changed
}

// unwrapAssignmentTargets removes the parentheses rewriteCallChainAssignment
// added, so the target's tree is the one an unparenthesized target would have.
func unwrapAssignmentTargets(root *annotationTreeNode, fixes []parenFix) bool {
	changed := false
	for _, f := range fixes {
		var path []*annotationTreeNode
		if f.kind == parenFixExpression {
			if pe := findParenthesizedAt(root, f.open, f.close+1, &path); pe != nil && unwrapParenthesized(pe, path, f) {
				changed = true
			}
			continue
		}
		if a := findAssignmentAt(root, f.open, &path); a != nil && unwrapAssignment(a, path, f) {
			changed = true
		}
	}
	return changed
}

func findParenthesizedAt(n *annotationTreeNode, open, end uint32, path *[]*annotationTreeNode) *annotationTreeNode {
	if n.start > open || n.end < end {
		return nil
	}
	if n.typ == "parenthesized_expression" && n.start == open && n.end == end {
		return n
	}
	*path = append(*path, n)
	for _, c := range n.children {
		if x := findParenthesizedAt(c, open, end, path); x != nil {
			return x
		}
	}
	*path = (*path)[:len(*path)-1]
	return nil
}

// unwrapParenthesized replaces a parenthesized_expression added by a rewrite
// with its inner expression.
func unwrapParenthesized(pe *annotationTreeNode, path []*annotationTreeNode, f parenFix) bool {
	if len(pe.children) != 3 || pe.children[0].typ != "(" || pe.children[2].typ != ")" || len(path) == 0 {
		return false
	}
	inner := pe.children[1]
	parent := path[len(path)-1]
	for i, c := range parent.children {
		if c == pe {
			parent.children[i] = inner
		}
	}
	for _, anc := range path {
		if anc.start == f.open {
			anc.start = f.start
		}
		if anc.end == f.close+1 {
			anc.end = inner.end
		}
	}
	return true
}

func findAssignmentAt(n *annotationTreeNode, open uint32, path *[]*annotationTreeNode) *annotationTreeNode {
	if n.start > open || n.end <= open {
		return nil
	}
	if n.typ == "assignment" && n.start == open {
		return n
	}
	*path = append(*path, n)
	for _, c := range n.children {
		if a := findAssignmentAt(c, open, path); a != nil {
			return a
		}
	}
	*path = (*path)[:len(*path)-1]
	return nil
}

func unwrapAssignment(a *annotationTreeNode, ancestors []*annotationTreeNode, f parenFix) bool {
	if len(a.children) == 0 || a.children[0].typ != "directly_assignable_expression" {
		return false
	}
	dae := a.children[0]
	if len(dae.children) != 1 || dae.children[0].typ != "parenthesized_expression" {
		return false
	}
	pe := dae.children[0]
	if len(pe.children) != 3 || pe.children[0].typ != "(" || pe.children[2].typ != ")" || pe.end != f.close+1 {
		return false
	}
	inner := pe.children[1]
	dae.children = []*annotationTreeNode{inner}
	dae.start, dae.end = inner.start, inner.end
	a.start = f.start
	for _, anc := range ancestors {
		if anc.start == f.open {
			anc.start = f.start
		}
	}
	return true
}

// findDelegatedType finds the user_type that ends at typeEnd inside a
// delegation_specifier. path holds the ancestors, parent last.
func findDelegatedType(n *annotationTreeNode, path []*annotationTreeNode, typeEnd uint32) (*annotationTreeNode, []*annotationTreeNode) {
	if n.typ == "user_type" && n.end == typeEnd && len(path) > 0 && path[len(path)-1].typ == "delegation_specifier" {
		return n, path
	}
	if n.end < typeEnd {
		return nil, nil
	}
	path = append(path, n)
	for _, c := range n.children {
		if c.end < typeEnd || c.start > typeEnd {
			continue
		}
		if x, p := findDelegatedType(c, path, typeEnd); x != nil {
			return x, p
		}
	}
	return nil, nil
}

// parseDelegateExpression parses the delegate expression alone, at its
// original offsets, and returns its tree.
func parseDelegateExpression(ctx context.Context, parser *sitter.Parser, orig []byte, d delegationRepair) *annotationTreeNode {
	buf := make([]byte, d.exprEnd)
	for i := range buf {
		buf[i] = ' '
		if orig[i] == '\n' || orig[i] == '\r' {
			buf[i] = orig[i]
		}
	}
	copy(buf[d.exprStart:], orig[d.exprStart:d.exprEnd])
	tree, err := parser.ParseCtx(ctx, nil, buf)
	if err != nil || tree == nil {
		return nil
	}
	t := flattenTree(tree.RootNode())
	var only uint32
	for c := t.FirstChildren[0]; c != 0; c = t.NextSibs[c] {
		switch nodeTypeName(t.Types[c]) {
		case "line_comment", "multiline_comment":
			continue
		}
		if only != 0 || t.Flags[c]&flatNodeFlagError != 0 {
			return nil
		}
		only = c
	}
	if only == 0 {
		return nil
	}
	return copyAnnotationTree(t, only)
}

// parseStandaloneAnnotation parses the annotation at [start, end) next to a
// throwaway class and returns the annotation node at its original offsets.
func parseStandaloneAnnotation(ctx context.Context, parser *sitter.Parser, src []byte, start, end uint32) *annotationTreeNode {
	buf := make([]byte, 0, int(end)+len("\nclass Z"))
	buf = append(buf, src[:end]...)
	for i := uint32(0); i < start; i++ {
		if buf[i] != '\n' && buf[i] != '\r' {
			buf[i] = ' '
		}
	}
	buf = append(buf, "\nclass Z"...)
	tree, err := parser.ParseCtx(ctx, nil, buf)
	if err != nil || tree == nil {
		return nil
	}
	t := flattenTree(tree.RootNode())
	for c := t.FirstChildren[0]; c != 0; c = t.NextSibs[c] {
		if nodeTypeName(t.Types[c]) != "class_declaration" {
			continue
		}
		mods := firstChildOfType(t, c, "modifiers")
		if mods == 0 {
			return nil
		}
		if ann := firstChildOfType(t, mods, "annotation"); ann != 0 && t.StartBytes[ann] == start && t.EndBytes[ann] == end {
			return copyAnnotationTree(t, ann)
		}
	}
	return nil
}

func firstChildOfType(t *FlatTree, parent uint32, typ string) uint32 {
	for c := t.FirstChildren[parent]; c != 0; c = t.NextSibs[c] {
		if nodeTypeName(t.Types[c]) == typ {
			return c
		}
	}
	return 0
}

// trimInsertedSemicolons ends nodes before a ';' that a rewrite put in place
// of whitespace, so node text matches the original source.
func trimInsertedSemicolons(ft *FlatTree, semis []uint32) {
	if len(semis) == 0 {
		return
	}
	set := make(map[uint32]bool, len(semis))
	for _, p := range semis {
		set[p+1] = true
	}
	for i := range ft.EndBytes {
		if set[ft.EndBytes[i]] {
			ft.EndBytes[i]--
			if ft.StartBytes[i] > ft.EndBytes[i] {
				ft.StartBytes[i] = ft.EndBytes[i]
			}
		}
	}
}

// graftContextParameters rebuilds the named parameters of each blanked
// context clause as a context_parameters node in front of its declaration,
// and widens the declaration over the clause. Clauses it cannot rebuild are
// left to extendDeclPrefixes.
func graftContextParameters(ctx context.Context, parser *sitter.Parser, root *annotationTreeNode, orig []byte, prefixes []declPrefix) bool {
	changed := false
	for i := range prefixes {
		p := &prefixes[i]
		if !p.context {
			continue
		}
		decl, path := findDeclAt(root, p.declStart, nil)
		if decl == nil {
			continue
		}
		params := parseContextParameters(ctx, parser, orig, p)
		if params == nil {
			continue
		}
		decl.children = append([]*annotationTreeNode{params}, decl.children...)
		for _, n := range append(path, decl) {
			if n.start == p.declStart {
				n.start = p.start
			}
		}
		p.grafted = true
		changed = true
	}
	return changed
}

func findDeclAt(n *annotationTreeNode, start uint32, path []*annotationTreeNode) (*annotationTreeNode, []*annotationTreeNode) {
	if n.start > start || n.end < start {
		return nil, nil
	}
	if n.start == start && isPrefixTarget(n.typ) {
		return n, path
	}
	path = append(path, n)
	for _, c := range n.children {
		if d, p := findDeclAt(c, start, path); d != nil {
			return d, p
		}
	}
	return nil, nil
}

// parseContextParameters parses the text inside `context( ... )` as the
// parameter list of a throwaway function, at its original offsets.
func parseContextParameters(ctx context.Context, parser *sitter.Parser, orig []byte, p *declPrefix) *annotationTreeNode {
	open := bytes.IndexByte(orig[p.start:p.end], '(')
	if open < 0 || open < 5 {
		return nil
	}
	open += int(p.start)
	end := int(p.end)
	buf := make([]byte, 0, end+4)
	for i := 0; i < open+1; i++ {
		buf = append(buf, ' ')
	}
	copy(buf[open-5:open+1], "fun f(")
	buf = append(buf, orig[open+1:end-1]...)
	buf = append(buf, ") {}"...)
	tree, err := parser.ParseCtx(ctx, nil, buf)
	if err != nil || tree == nil {
		return nil
	}
	t := flattenTree(tree.RootNode())
	if t.Flags[0]&flatNodeFlagError != 0 {
		return nil
	}
	fn := firstChildOfType(t, 0, "function_declaration")
	if fn == 0 {
		return nil
	}
	fvp := firstChildOfType(t, fn, "function_value_parameters")
	if fvp == 0 {
		return nil
	}
	node := copyAnnotationTree(t, fvp)
	node.typ = ContextParametersNodeType
	node.start, node.end = uint32(open), uint32(end)
	if len(node.children) > 0 && node.children[0].typ == "(" {
		node.children[0].start, node.children[0].end = uint32(open), uint32(open+1)
	}
	if n := len(node.children); n > 0 && node.children[n-1].typ == ")" {
		node.children[n-1].start, node.children[n-1].end = uint32(end-1), uint32(end)
	}
	return node
}
