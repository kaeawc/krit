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
	if t == nil || len(t.Types) == 0 || nodeTypeName(t.Types[0]) != "source_file" || !hasTopLevelAnnotationCandidate(t) {
		return t
	}
	root := copyAnnotationTree(t, 0)
	changed := repairTopLevelAnnotatedAccessors(root)
	// Walk backward so a run of annotation expressions attaches from the
	// declaration outward.
	for i := len(root.children) - 1; i >= 0; i-- {
		switch root.children[i].typ {
		case "annotation":
			// The grammar can leave the first annotation of a run outside the
			// declaration it belongs to.
			if attachBareTopLevelAnnotation(root, i) {
				changed = true
			}
		case "prefix_expression":
			if attachChainedTopLevelAnnotations(root, i) {
				changed = true
			}
		}
	}
	if !changed {
		return t
	}
	out := &FlatTree{}
	appendAnnotationTree(out, root, 0, false)
	out.buildNodesByType()
	return out
}

// repairTopLevelAnnotatedAccessors rewrites top-level accessors that
// tree-sitter-kotlin misparses. A top-level property's accessor on its own
// line with a single annotation and a block body, such as
//
//	val Prop: Int
//	    @Composable get() { ... }
//
// parses as a call to `get` with a trailing lambda, either wrapped with the
// annotation in a prefix_expression or after a bare top-level annotation, so
// the accessor and its annotation are lost (#1613). The same happens to
// `@Ann set(value) { ... }`. Only those exact shapes directly after a
// property or another accessor are rewritten into the getter/setter tree the
// grammar produces for a class member.
func repairTopLevelAnnotatedAccessors(root *annotationTreeNode) bool {
	changed := false
	prev := ""
	for i := 0; i < len(root.children); i++ {
		c := root.children[i]
		switch c.typ {
		case "line_comment", "multiline_comment":
			continue
		}
		if prev == "property_declaration" || prev == "getter" || prev == "setter" {
			var accessor *annotationTreeNode
			switch {
			case c.typ == "prefix_expression" && len(c.children) == 2 && c.children[0].typ == "annotation":
				accessor = recoverAnnotatedAccessor(c.children[0], c.children[1])
				if accessor != nil {
					root.children[i] = accessor
				}
			case c.typ == "annotation" && i+1 < len(root.children):
				accessor = recoverAnnotatedAccessor(c, root.children[i+1])
				if accessor != nil {
					root.children = append(root.children[:i], append([]*annotationTreeNode{accessor}, root.children[i+2:]...)...)
				}
			}
			if accessor != nil {
				c = accessor
				changed = true
			}
		}
		prev = c.typ
	}
	return changed
}

// recoverAnnotatedAccessor returns the getter or setter for annotation ann
// followed by the misparsed call_expression(call_expression(get|set, (args)), { body }),
// or nil when call has any other shape.
func recoverAnnotatedAccessor(ann, call *annotationTreeNode) *annotationTreeNode {
	keyword, args, lambda := misparsedAccessorParts(call)
	if keyword == nil {
		return nil
	}
	params := accessorParams(keyword.typ, args)
	if params == nil {
		return nil
	}
	typ := "getter"
	if keyword.typ == "set" {
		typ = "setter"
	}
	mods := &annotationTreeNode{typ: "modifiers", start: ann.start, end: ann.end, row: ann.row, col: ann.col, flags: flatNodeFlagNamed, children: []*annotationTreeNode{ann}}
	body := &annotationTreeNode{typ: "function_body", start: lambda.start, end: lambda.end, row: lambda.row, col: lambda.col, flags: lambda.flags | flatNodeFlagNamed, children: lambda.children}
	return &annotationTreeNode{typ: typ, start: ann.start, end: call.end, row: ann.row, col: ann.col, flags: call.flags | flatNodeFlagNamed,
		children: append(append([]*annotationTreeNode{mods, keyword}, params...), body)}
}

// misparsedAccessorParts splits call_expression(call_expression(get|set, (args)), { body })
// into the get/set keyword, the value_arguments and the parameterless lambda;
// keyword is nil for any other shape.
func misparsedAccessorParts(call *annotationTreeNode) (keyword, args, lambda *annotationTreeNode) {
	if !treeShape(call, "call_expression", 2) {
		return nil, nil, nil
	}
	inner, trailing := call.children[0], call.children[1]
	if !treeShape(inner, "call_expression", 2) || !treeShape(trailing, "call_suffix", 1) {
		return nil, nil, nil
	}
	name, suffix := inner.children[0], inner.children[1]
	if !treeShape(name, "simple_identifier", 1) || !treeShape(suffix, "call_suffix", 1) || suffix.children[0].typ != "value_arguments" {
		return nil, nil, nil
	}
	if k := name.children[0].typ; k != "get" && k != "set" {
		return nil, nil, nil
	}
	lambdaArg := trailing.children[0]
	if !treeShape(lambdaArg, "annotated_lambda", 1) || lambdaArg.children[0].typ != "lambda_literal" {
		return nil, nil, nil
	}
	lambda = lambdaArg.children[0]
	for _, part := range lambda.children {
		if part.typ == "lambda_parameters" || part.typ == "->" {
			return nil, nil, nil
		}
	}
	return name.children[0], suffix.children[0], lambda
}

// accessorParams converts a misparsed call's value_arguments into accessor
// parameters: `()` for a getter, `(name)` for a setter. nil means the
// arguments cannot be accessor parameters.
func accessorParams(keyword string, args *annotationTreeNode) []*annotationTreeNode {
	n := len(args.children)
	if n < 2 || args.children[0].typ != "(" || args.children[n-1].typ != ")" {
		return nil
	}
	switch {
	case keyword == "get" && n == 2:
		return args.children
	case keyword == "set" && n == 3:
		arg := args.children[1]
		if !treeShape(arg, "value_argument", 1) || arg.children[0].typ != "simple_identifier" {
			return nil
		}
		param := &annotationTreeNode{typ: "parameter_with_optional_type", start: arg.start, end: arg.end, row: arg.row, col: arg.col, flags: arg.flags | flatNodeFlagNamed, children: arg.children}
		return []*annotationTreeNode{args.children[0], param, args.children[2]}
	}
	return nil
}

func treeShape(n *annotationTreeNode, typ string, children int) bool {
	return n != nil && n.typ == typ && len(n.children) == children
}

// hasTopLevelAnnotationCandidate reports whether the source file has a top-level node that may hold a detached annotation.
func hasTopLevelAnnotationCandidate(t *FlatTree) bool {
	for c := t.FirstChildren[0]; c != 0; c = t.NextSibs[c] {
		if typ := nodeTypeName(t.Types[c]); typ == "prefix_expression" || typ == "annotation" {
			return true
		}
	}
	return false
}

// attachBareTopLevelAnnotation moves the annotation at root.children[i] onto the declaration after it.
func attachBareTopLevelAnnotation(root *annotationTreeNode, i int) bool {
	decl, end := topLevelAnnotationTarget(root.children, i, nil)
	if decl == nil {
		return false
	}
	attachAndSpliceTopLevel(root, i, end, decl, []*annotationTreeNode{root.children[i]})
	return true
}

// attachChainedTopLevelAnnotations moves the annotations chained in the prefix_expression at
// root.children[i] onto the declaration they belong to.
func attachChainedTopLevelAnnotations(root *annotationTreeNode, i int) bool {
	annotations, decl, splitParen := chainedTopLevelAnnotations(root.children[i])
	decl, end := topLevelAnnotationTarget(root.children, i, decl)
	if decl == nil || len(annotations) == 0 {
		return false
	}
	if splitParen != nil {
		last := annotations[len(annotations)-1]
		last.children = append(last.children, splitParen)
	}
	for _, ann := range annotations {
		repairSplitAnnotation(ann)
	}
	attachAndSpliceTopLevel(root, i, end, decl, annotations)
	return true
}

// attachAndSpliceTopLevel attaches annotations and the comments between them and decl to decl,
// then replaces root.children[i:end] with decl.
func attachAndSpliceTopLevel(root *annotationTreeNode, i, end int, decl *annotationTreeNode, annotations []*annotationTreeNode) {
	var comments []*annotationTreeNode
	if end > i+1 {
		comments = root.children[i+1 : end-1]
	}
	attachTopLevelAnnotations(decl, annotations, comments)
	root.children = append(root.children[:i], append([]*annotationTreeNode{decl}, root.children[end:]...)...)
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
