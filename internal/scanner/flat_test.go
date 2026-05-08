package scanner

import (
	"os"
	"path/filepath"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

type preorderNode struct {
	node   *sitter.Node
	parent int
}

func collectPreorder(root *sitter.Node) []preorderNode {
	var out []preorderNode
	var walk func(node *sitter.Node, parent int)
	walk = func(node *sitter.Node, parent int) {
		if node == nil {
			return
		}
		idx := len(out)
		out = append(out, preorderNode{node: node, parent: parent})
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i), idx)
		}
	}
	walk(root, -1)
	return out
}

func TestFlattenTree_PreorderAndLinks(t *testing.T) {
	root, _ := parseKotlin(t, `
abstract class Example(private val name: String) {
    protected fun greet() {
        println(name)
    }
}
`)
	flat := flattenTree(root)
	preorder := collectPreorder(root)

	if flat == nil {
		t.Fatal("expected non-nil FlatTree")
	}
	if len(flat.Nodes) != len(preorder) {
		t.Fatalf("expected %d flat nodes, got %d", len(preorder), len(flat.Nodes))
	}

	childrenByParent := make(map[int][]int)
	for idx, info := range preorder {
		childrenByParent[info.parent] = append(childrenByParent[info.parent], idx)
	}

	for idx, info := range preorder {
		got := flat.Nodes[idx]
		if got.TypeName() != info.node.Type() {
			t.Errorf("node %d: expected type %q, got %q", idx, info.node.Type(), got.TypeName())
		}
		if got.StartByte != info.node.StartByte() || got.EndByte != info.node.EndByte() {
			t.Errorf("node %d: expected byte span [%d,%d), got [%d,%d)",
				idx, info.node.StartByte(), info.node.EndByte(), got.StartByte, got.EndByte)
		}
		start := info.node.StartPoint()
		if got.StartRow != saturateUint16(start.Row) || got.StartCol != saturateUint16(start.Column) {
			t.Errorf("node %d: expected start (%d,%d), got (%d,%d)",
				idx, start.Row, start.Column, got.StartRow, got.StartCol)
		}
		if got.ChildCount != saturateUint16(info.node.ChildCount()) {
			t.Errorf("node %d: expected child count %d, got %d", idx, info.node.ChildCount(), got.ChildCount)
		}
		if got.NamedCount != saturateUint16(info.node.NamedChildCount()) {
			t.Errorf("node %d: expected named child count %d, got %d", idx, info.node.NamedChildCount(), got.NamedCount)
		}
		if got.IsNamed() != info.node.IsNamed() {
			t.Errorf("node %d: expected named=%v, got %v", idx, info.node.IsNamed(), got.IsNamed())
		}
		if got.HasError() != (info.node.IsError() || info.node.HasError()) {
			t.Errorf("node %d: expected error=%v, got %v", idx, info.node.IsError() || info.node.HasError(), got.HasError())
		}
		if idx > 0 && got.Parent != uint32(info.parent) {
			t.Errorf("node %d: expected parent %d, got %d", idx, info.parent, got.Parent)
		}

		expectedFirstChild := uint32(0)
		if children := childrenByParent[idx]; len(children) > 0 {
			expectedFirstChild = uint32(children[0])
		}
		if got.FirstChild != expectedFirstChild {
			t.Errorf("node %d: expected first child %d, got %d", idx, expectedFirstChild, got.FirstChild)
		}

		expectedNextSib := uint32(0)
		if siblings := childrenByParent[info.parent]; len(siblings) > 0 {
			for sibIdx, sibling := range siblings {
				if sibling == idx && sibIdx+1 < len(siblings) {
					expectedNextSib = uint32(siblings[sibIdx+1])
					break
				}
			}
		}
		if got.NextSib != expectedNextSib {
			t.Errorf("node %d: expected next sibling %d, got %d", idx, expectedNextSib, got.NextSib)
		}
	}
}

func TestFlatHelpers_MatchTreeHelpers(t *testing.T) {
	root, content := parseKotlin(t, `
abstract class Example {
    protected fun greet() {}
}
`)
	flat := flattenTree(root)

	var classNode *sitter.Node
	WalkNodes(root, "class_declaration", func(node *sitter.Node) {
		if classNode == nil {
			classNode = node
		}
	})
	if classNode == nil {
		t.Fatal("expected class_declaration node")
	}

	var classIdx uint32
	var classCount int
	FlatWalkNodes(flat, "class_declaration", func(idx uint32) {
		classCount++
		if classCount == 1 {
			classIdx = idx
		}
	})
	if classCount != CountNodes(root, "class_declaration") {
		t.Fatalf("expected %d flat class nodes, got %d", CountNodes(root, "class_declaration"), classCount)
	}

	modsNode := FindChild(classNode, "modifiers")
	modsIdx, modsOk := FlatFindChild(flat, classIdx, "modifiers")
	if modsNode == nil || !modsOk {
		t.Fatalf("expected modifiers child in both trees, got node=%v ok=%v", modsNode != nil, modsOk)
	}
	if FlatNodeText(flat, modsIdx, content) != NodeText(modsNode, content) {
		t.Errorf("expected modifier text %q, got %q", NodeText(modsNode, content), FlatNodeText(flat, modsIdx, content))
	}
	if FlatHasModifier(flat, classIdx, content, "abstract") != HasModifier(classNode, content, "abstract") {
		t.Error("expected FlatHasModifier to match HasModifier for abstract")
	}
	if FlatHasModifier(flat, classIdx, content, "override") != HasModifier(classNode, content, "override") {
		t.Error("expected FlatHasModifier to match HasModifier for override")
	}
}

func TestFlatNodeBytes_ReturnsLiveView(t *testing.T) {
	root, content := parseKotlin(t, `
abstract class Example {
    protected fun greet() {}
}
`)
	flat := flattenTree(root)

	var classIdx uint32
	FlatWalkNodes(flat, "class_declaration", func(idx uint32) {
		if classIdx == 0 {
			classIdx = idx
		}
	})
	if classIdx == 0 {
		t.Fatal("expected class_declaration index")
	}

	modsIdx, ok := FlatFindChild(flat, classIdx, "modifiers")
	if !ok {
		t.Fatal("expected modifiers child")
	}

	view := FlatNodeBytes(flat, modsIdx, content)
	if string(view) != "abstract" {
		t.Fatalf("expected %q, got %q", "abstract", string(view))
	}

	content[flat.Nodes[modsIdx].StartByte] = 'A'
	if string(view) != "Abstract" {
		t.Fatalf("expected live view to reflect updated content, got %q", string(view))
	}
}

func TestFlatNodeString_ReturnsStableInternedCopy(t *testing.T) {
	root, content := parseKotlin(t, `
abstract class Example {
    protected fun greet() {}
}
`)
	flat := flattenTree(root)

	var classIdx uint32
	FlatWalkNodes(flat, "class_declaration", func(idx uint32) {
		if classIdx == 0 {
			classIdx = idx
		}
	})
	if classIdx == 0 {
		t.Fatal("expected class_declaration index")
	}

	modsIdx, ok := FlatFindChild(flat, classIdx, "modifiers")
	if !ok {
		t.Fatal("expected modifiers child")
	}

	pool := NewStringPool()
	got := FlatNodeString(flat, modsIdx, content, pool)
	content[flat.Nodes[modsIdx].StartByte] = 'A'
	if got != "abstract" {
		t.Fatalf("expected interned string to remain stable after content mutation, got %q", got)
	}
}

func TestParseFile_PopulatesFlatTree(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "FlatTree.kt")
	src := "fun greet() = println(\"hi\")\n"
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	file, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if file.FlatTree == nil {
		t.Fatal("expected ParseFile to populate FlatTree")
	}
	if len(file.FlatTree.Nodes) == 0 {
		t.Fatal("expected FlatTree to contain nodes")
	}
	if file.FlatTree.Nodes[0].TypeName() != "source_file" {
		t.Fatalf("expected flat root type source_file, got %q", file.FlatTree.Nodes[0].TypeName())
	}
}
