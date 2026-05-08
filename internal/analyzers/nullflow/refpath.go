package nullflow

import "github.com/kaeawc/krit/internal/scanner"

// ReferencePath is a textual+positional decomposition of a navigation chain
// expression like `a.b.c` or `this.a.b`. Parts holds the dotted text segments;
// Nodes holds the flat-AST node indexes for each segment; Root is the first
// node in the chain (the leftmost identifier or this_expression).
type ReferencePath struct {
	Parts []string
	Nodes []uint32
	Root  uint32
}

// FlatReferencePathFromExpr decomposes a simple_identifier, this_expression,
// or navigation_expression into a ReferencePath. Returns ok=false for any
// other shape (call expressions, indexing expressions, etc.).
func FlatReferencePathFromExpr(file *scanner.File, idx uint32) (ReferencePath, bool) {
	idx = flatUnwrapParenExpr(file, idx)
	switch file.FlatType(idx) {
	case "simple_identifier", "this_expression":
		return ReferencePath{Parts: []string{file.FlatNodeText(idx)}, Nodes: []uint32{idx}, Root: idx}, true
	case "navigation_expression":
		var out ReferencePath
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			switch file.FlatType(child) {
			case "simple_identifier", "this_expression", "navigation_expression":
				childPath, ok := FlatReferencePathFromExpr(file, child)
				if !ok {
					return ReferencePath{}, false
				}
				if out.Root == 0 {
					out.Root = childPath.Root
				}
				out.Parts = append(out.Parts, childPath.Parts...)
				out.Nodes = append(out.Nodes, childPath.Nodes...)
			case "navigation_suffix":
				if flatCallSuffixValueArgs(file, child) != 0 {
					return ReferencePath{}, false
				}
				ident, ok := file.FlatFindChild(child, "simple_identifier")
				if !ok || ident == 0 {
					return ReferencePath{}, false
				}
				out.Parts = append(out.Parts, file.FlatNodeText(ident))
				out.Nodes = append(out.Nodes, ident)
			default:
				return ReferencePath{}, false
			}
		}
		return out, out.Root != 0 && len(out.Parts) > 0
	default:
		return ReferencePath{}, false
	}
}

// ResolveSimpleReferenceDeclaration returns the flat-AST index of the
// declaration that ref textually resolves to in the same file, preferring
// nearer local declarations over class members. Returns 0 when no candidate
// is visible. Cross-file resolution is out of scope; callers needing more
// precise resolution should use the typeinfer.Resolver.
func ResolveSimpleReferenceDeclaration(file *scanner.File, ref uint32) uint32 {
	if file == nil || ref == 0 {
		return 0
	}
	name := file.FlatNodeText(ref)
	if name == "" || name == "this" {
		return 0
	}
	var bestLocal uint32
	var bestMember uint32
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if candidate == 0 || candidate == ref {
			return
		}
		switch file.FlatType(candidate) {
		case "parameter", "class_parameter", "property_declaration":
			if extractIdentifier(file, candidate) != name || !declarationVisibleFromReference(file, candidate, ref) {
				return
			}
			if _, local := flatEnclosingAncestor(file, candidate, "function_declaration", "lambda_literal"); local {
				if bestLocal == 0 || file.FlatStartByte(candidate) >= file.FlatStartByte(bestLocal) {
					bestLocal = candidate
				}
				return
			}
			if bestMember == 0 || file.FlatStartByte(candidate) >= file.FlatStartByte(bestMember) {
				bestMember = candidate
			}
		}
	})
	if bestLocal != 0 {
		return bestLocal
	}
	return bestMember
}

func declarationVisibleFromReference(file *scanner.File, decl, ref uint32) bool {
	declLocalOwner, declLocal := flatEnclosingAncestor(file, decl, "function_declaration", "lambda_literal")
	refLocalOwner, refLocal := flatEnclosingAncestor(file, ref, "function_declaration", "lambda_literal")
	if declLocal {
		return refLocal && declLocalOwner == refLocalOwner && file.FlatStartByte(decl) <= file.FlatStartByte(ref)
	}

	declClassOwner, declClass := flatEnclosingAncestor(file, decl, "class_declaration", "object_declaration")
	refClassOwner, refClass := flatEnclosingAncestor(file, ref, "class_declaration", "object_declaration")
	if declClass {
		return refClass && declClassOwner == refClassOwner
	}

	return true
}

func trimLeadingThisPath(path ReferencePath) (ReferencePath, bool) {
	if len(path.Parts) < 2 || path.Parts[0] != "this" {
		return path, false
	}
	return ReferencePath{
		Parts: path.Parts[1:],
		Nodes: path.Nodes[1:],
		Root:  path.Nodes[1],
	}, true
}

func referencePathsMatchReceiver(file *scanner.File, candPath, recvPath ReferencePath, useIdx uint32) bool {
	if len(candPath.Parts) != len(recvPath.Parts) || len(candPath.Parts) == 0 {
		return false
	}
	for i := range candPath.Parts {
		if candPath.Parts[i] != recvPath.Parts[i] {
			return false
		}
	}
	if len(candPath.Parts) == 1 {
		return sameResolvableReferenceTarget(file, candPath.Nodes[0], recvPath.Nodes[0])
	}
	return sameQualifiedReceiverTarget(file, candPath.Nodes[0], recvPath.Nodes[0], useIdx)
}

func sameExplicitThisReferenceTarget(file *scanner.File, candPath, recvPath ReferencePath, useIdx uint32) bool {
	candTrimmed, candHadThis := trimLeadingThisPath(candPath)
	recvTrimmed, recvHadThis := trimLeadingThisPath(recvPath)
	if !candHadThis && !recvHadThis {
		return false
	}
	if len(candTrimmed.Parts) == 0 || len(recvTrimmed.Parts) == 0 {
		return false
	}
	if candHadThis && recvHadThis {
		candClass, candOK := flatEnclosingAncestor(file, candPath.Nodes[0], "class_declaration", "object_declaration")
		recvClass, recvOK := flatEnclosingAncestor(file, recvPath.Nodes[0], "class_declaration", "object_declaration")
		return candOK && recvOK && candClass == recvClass
	}
	if candHadThis {
		return explicitThisMemberMatchesReference(file, candPath.Nodes[0], candTrimmed.Nodes[0], recvTrimmed.Nodes[0], useIdx)
	}
	return explicitThisMemberMatchesReference(file, recvPath.Nodes[0], recvTrimmed.Nodes[0], candTrimmed.Nodes[0], useIdx)
}

func explicitThisMemberMatchesReference(file *scanner.File, thisNode, memberNameNode, otherRoot uint32, useIdx uint32) bool {
	classNode, ok := flatEnclosingAncestor(file, thisNode, "class_declaration", "object_declaration")
	if !ok {
		return false
	}
	useClass, ok := flatEnclosingAncestor(file, useIdx, "class_declaration", "object_declaration")
	if !ok || useClass != classNode {
		return false
	}
	memberDecl := classMemberDeclarationByName(file, classNode, file.FlatNodeText(memberNameNode))
	if memberDecl == 0 {
		return false
	}
	return ResolveSimpleReferenceDeclaration(file, otherRoot) == memberDecl
}

func classMemberDeclarationByName(file *scanner.File, classNode uint32, name string) uint32 {
	var found uint32
	file.FlatWalkAllNodes(classNode, func(candidate uint32) {
		if found != 0 || extractIdentifier(file, candidate) != name {
			return
		}
		switch file.FlatType(candidate) {
		case "property_declaration":
			owner, ok := flatEnclosingAncestor(file, candidate, "class_declaration", "object_declaration")
			if ok && owner == classNode {
				found = candidate
			}
		case "class_parameter":
			if parameterDeclaresProperty(file, candidate) {
				found = candidate
			}
		}
	})
	return found
}

func parameterDeclaresProperty(file *scanner.File, param uint32) bool {
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "val" || file.FlatType(child) == "var" ||
			file.FlatNodeTextEquals(child, "val") || file.FlatNodeTextEquals(child, "var") {
			return true
		}
	}
	return false
}

func sameResolvableReferenceTarget(file *scanner.File, a, b uint32) bool {
	if a == 0 || b == 0 || !file.FlatNodeTextEquals(a, file.FlatNodeText(b)) {
		return false
	}
	declA := ResolveSimpleReferenceDeclaration(file, a)
	declB := ResolveSimpleReferenceDeclaration(file, b)
	if declA == 0 || declB == 0 {
		return false
	}
	return declA == declB
}

func sameQualifiedReceiverTarget(file *scanner.File, a, b, useIdx uint32) bool {
	if a == 0 || b == 0 {
		return false
	}
	if file.FlatNodeTextEquals(a, "this") && file.FlatNodeTextEquals(b, "this") {
		classA, okA := flatEnclosingAncestor(file, a, "class_declaration", "object_declaration")
		classB, okB := flatEnclosingAncestor(file, b, "class_declaration", "object_declaration")
		return okA && okB && classA == classB
	}
	if sameResolvableReferenceTarget(file, a, b) {
		return true
	}
	ownerA, okA := flatEnclosingAncestor(file, a, "function_declaration", "lambda_literal", "property_declaration")
	ownerB, okB := flatEnclosingAncestor(file, b, "function_declaration", "lambda_literal", "property_declaration")
	ownerUse, okUse := flatEnclosingAncestor(file, useIdx, "function_declaration", "lambda_literal", "property_declaration")
	return okA && okB && okUse && ownerA == ownerB && ownerA == ownerUse && file.FlatNodeTextEquals(a, file.FlatNodeText(b))
}
