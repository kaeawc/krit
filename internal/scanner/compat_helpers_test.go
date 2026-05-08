package scanner

import sitter "github.com/smacker/go-tree-sitter"

// This file holds the node-era traversal helpers that exist only to keep
// the scanner-package parity tests (flat_test.go, scanner_test.go) running
// as behavioral oracles for the FlatTree equivalents. They were moved out of
// production scanner.go during Track C (2026-04-14) so the scanner package's
// production binary has zero exported `*sitter.Node` helpers — everything
// below is test-only.
//
// Do not move anything here back into a non-`_test.go` file without a
// matching conversion in the production callers.

func NodeBytes(node *sitter.Node, content []byte) []byte {
	return content[node.StartByte():node.EndByte()]
}

func NodeText(node *sitter.Node, content []byte) string {
	return string(NodeBytes(node, content))
}

func ForEachChild(node *sitter.Node, fn func(child *sitter.Node)) {
	if node == nil {
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		if child := node.Child(i); child != nil {
			fn(child)
		}
	}
}

func FindChild(node *sitter.Node, childType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		if node.Child(i).Type() == childType {
			return node.Child(i)
		}
	}
	return nil
}

func WalkNodes(node *sitter.Node, nodeType string, fn func(*sitter.Node)) {
	if node.Type() == nodeType {
		fn(node)
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		WalkNodes(node.Child(i), nodeType, fn)
	}
}

func WalkAllNodes(node *sitter.Node, fn func(*sitter.Node)) {
	fn(node)
	for i := 0; i < int(node.ChildCount()); i++ {
		WalkAllNodes(node.Child(i), fn)
	}
}

func HasModifier(node *sitter.Node, content []byte, modifier string) bool {
	mods := FindChild(node, "modifiers")
	if mods == nil {
		return false
	}
	for i := 0; i < int(mods.ChildCount()); i++ {
		child := mods.Child(i)
		if string(NodeBytes(child, content)) == modifier {
			return true
		}
		for j := 0; j < int(child.ChildCount()); j++ {
			if string(NodeBytes(child.Child(j), content)) == modifier {
				return true
			}
		}
	}
	return false
}

func HasAncestorOfType(node *sitter.Node, ancestorType string) bool {
	for p := node.Parent(); p != nil; p = p.Parent() {
		if p.Type() == ancestorType {
			return true
		}
	}
	return false
}

// NodeString returns a pool-interned copy of a node's text for parity
// tests that measure string-pool behavior against the FlatTree
// equivalent.
func NodeString(node *sitter.Node, content []byte, pool *StringPool) string {
	b := NodeBytes(node, content)
	if len(b) == 0 {
		return ""
	}
	if pool == nil {
		return internBytes(b)
	}
	return pool.Intern(bytesToStringView(b))
}

// CountNodes counts descendants of the given type and is used as the
// oracle for flat walker counts.
func CountNodes(node *sitter.Node, nodeType string) int {
	count := 0
	WalkNodes(node, nodeType, func(_ *sitter.Node) { count++ })
	return count
}

// FindModifierNode returns the node matching a given modifier token and is
// used as the oracle for flat modifier lookups.
func FindModifierNode(node *sitter.Node, content []byte, modifier string) *sitter.Node {
	mods := FindChild(node, "modifiers")
	if mods == nil {
		return nil
	}
	for i := 0; i < int(mods.ChildCount()); i++ {
		child := mods.Child(i)
		if NodeText(child, content) == modifier {
			return child
		}
		for j := 0; j < int(child.ChildCount()); j++ {
			gc := child.Child(j)
			if NodeText(gc, content) == modifier {
				return gc
			}
		}
	}
	return nil
}
