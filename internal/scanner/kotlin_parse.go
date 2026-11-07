package scanner

import (
	"context"

	sitter "github.com/smacker/go-tree-sitter"
)

// ParseKotlinFlat parses Kotlin source into a FlatTree, repairing grammar
// misparses that would otherwise hide top-level declarations:
//
//   - a class header followed by a constructor on a later line (see
//     ctorGapBreaks);
//   - two or more top-level annotations before a modifier-led function with
//     an empty parameter list, which the grammar reads as an expression.
//
// Byte offsets, rows and columns in the result match content.
func ParseKotlinFlat(ctx context.Context, parser *sitter.Parser, content []byte) (*FlatTree, error) {
	breaks := ctorGapBreaks(content)
	src := content
	if len(breaks) > 0 {
		src = blankBreaks(content, breaks)
	}
	src, rw := rewriteKotlin(src)
	tree, err := parser.ParseCtx(ctx, nil, src)
	if err != nil {
		return nil, err
	}
	ft := normalizeExplicitBackingFields(flattenTree(tree.RootNode()), content)
	if chains := findMisparsedAnnotationChains(ft, src); len(chains) > 0 {
		var spans [][2]uint32
		for _, ch := range chains {
			spans = append(spans, ch.annotationSpans...)
		}
		tree2, err := parser.ParseCtx(ctx, nil, blankSpans(src, spans))
		if err == nil {
			ft2 := normalizeExplicitBackingFields(flattenTree(tree2.RootNode()), content)
			ft = graftAnnotatedDeclarations(ctx, parser, src, ft, ft2, chains)
		}
	}
	ft = unwrapClassBodyErrors(ft)
	if rw != nil {
		ft = rw.repairTree(ctx, parser, ft)
	}
	if len(breaks) > 0 || rw != nil {
		restorePositions(ft, content)
	}
	return ft, nil
}

// ParseKotlinFile parses content like ParseKotlinFlat and wraps it in a File.
func ParseKotlinFile(ctx context.Context, parser *sitter.Parser, path string, content []byte) (*File, error) {
	ft, err := ParseKotlinFlat(ctx, parser, content)
	if err != nil {
		return nil, err
	}
	return newKotlinFileFromFlatTree(path, content, ft), nil
}

func blankSpans(src []byte, spans [][2]uint32) []byte {
	out := make([]byte, len(src))
	copy(out, src)
	for _, sp := range spans {
		for i := sp[0]; i < sp[1] && int(i) < len(out); i++ {
			if out[i] != '\n' && out[i] != '\r' {
				out[i] = ' '
			}
		}
	}
	return out
}

// misparsedChain is a run of top-level annotations followed by a
// prefix_expression the grammar produced instead of a declaration.
type misparsedChain struct {
	// first and last are the top-level child positions (inclusive) replaced.
	first, last     int
	annotations     []uint32
	annotationSpans [][2]uint32
	annEnd, end     uint32
}

func findMisparsedAnnotationChains(t *FlatTree, src []byte) []misparsedChain {
	if t == nil || len(t.Types) == 0 || nodeTypeName(t.Types[0]) != "source_file" {
		return nil
	}
	var chains []misparsedChain
	lex := &kotlinRewriter{orig: src}
	pos := 0
	var pendingAnn []uint32
	pendingFirst := -1
	for c := t.FirstChildren[0]; c != 0; c = t.NextSibs[c] {
		switch nodeTypeName(t.Types[c]) {
		case "annotation":
			if len(pendingAnn) == 0 {
				pendingFirst = pos
			}
			pendingAnn = append(pendingAnn, c)
		case "prefix_expression":
			ch := misparsedChain{first: pos, last: pos, end: t.EndBytes[c]}
			if len(pendingAnn) > 0 {
				ch.first = pendingFirst
				ch.annotations = append(ch.annotations, pendingAnn...)
			}
			pendingAnn = nil
			for n := c; n != 0 && nodeTypeName(t.Types[n]) == "prefix_expression"; {
				ann := t.FirstChildren[n]
				if ann == 0 || nodeTypeName(t.Types[ann]) != "annotation" {
					break
				}
				ch.annotations = append(ch.annotations, ann)
				n = t.NextSibs[ann]
			}
			if len(ch.annotations) >= 1 {
				for _, a := range ch.annotations {
					end := t.EndBytes[a]
					// The grammar can leave the arguments out of the annotation node.
					if e := uint32(lex.annotationEnd(int(t.StartBytes[a]))); e > end && e <= ch.end {
						end = e
					}
					ch.annotationSpans = append(ch.annotationSpans, [2]uint32{t.StartBytes[a], end})
					ch.annEnd = end
				}
				chains = append(chains, ch)
			}
		default:
			pendingAnn = nil
		}
		pos++
	}
	return chains
}

// graftAnnotatedDeclarations replaces each chain in t1 with the declaration
// t2 parsed once the chain's annotations were blanked, then attaches the
// annotation nodes from t1. Chains without a matching declaration stay as
// t1 parsed them.
func graftAnnotatedDeclarations(ctx context.Context, parser *sitter.Parser, src []byte, t1, t2 *FlatTree, chains []misparsedChain) *FlatTree {
	root1 := copyAnnotationTree(t1, 0)
	root2 := copyAnnotationTree(t2, 0)
	changed := false
	for k := len(chains) - 1; k >= 0; k-- {
		ch := chains[k]
		var repl []*annotationTreeNode
		for _, n := range root2.children {
			if n.start >= ch.annEnd && n.end <= ch.end {
				repl = append(repl, n)
			}
		}
		if len(repl) == 0 || !isTopLevelAnnotationDeclaration(repl[0].typ) || repl[len(repl)-1].end != ch.end {
			continue
		}
		var anns []*annotationTreeNode
		for i, a := range ch.annotations {
			ann := copyAnnotationTree(t1, a)
			if sp := ch.annotationSpans[i]; sp[1] > ann.end {
				if built := parseStandaloneAnnotation(ctx, parser, src, sp[0], sp[1]); built != nil {
					ann = built
				}
			}
			anns = append(anns, ann)
		}
		for _, a := range anns {
			repairSplitAnnotation(a)
		}
		attachTopLevelAnnotations(repl[0], anns, nil)
		root1.children = append(root1.children[:ch.first], append(append([]*annotationTreeNode{}, repl...), root1.children[ch.last+1:]...)...)
		changed = true
	}
	if !changed {
		return t1
	}
	out := &FlatTree{}
	appendAnnotationTree(out, root1, 0, false)
	out.buildNodesByType()
	return out
}
