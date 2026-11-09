package scanner

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

// flattenTreeReference is the original per-node cgo walker, kept as the oracle
// for the single-pass C flattener in tsflat.go.
func flattenTreeReference(root *sitter.Node) *FlatTree {
	if root == nil {
		return &FlatTree{}
	}

	capHint := estimateFlatCapacity(root)
	t := &FlatTree{
		Types:         make([]uint16, 0, capHint),
		Parents:       make([]uint32, 0, capHint),
		FirstChildren: make([]uint32, 0, capHint),
		NextSibs:      make([]uint32, 0, capHint),
		PrevSibs:      make([]uint32, 0, capHint),
		StartBytes:    make([]uint32, 0, capHint),
		EndBytes:      make([]uint32, 0, capHint),
		StartRows:     make([]uint16, 0, capHint),
		StartCols:     make([]uint16, 0, capHint),
		ChildCounts:   make([]uint16, 0, capHint),
		NamedCounts:   make([]uint16, 0, capHint),
		Flags:         make([]uint8, 0, capHint),
	}

	var walk func(node *sitter.Node, parent uint32, hasParent bool) uint32
	walk = func(node *sitter.Node, parent uint32, hasParent bool) uint32 {
		idx := uint32(len(t.Types))
		start := node.StartPoint()
		typeID := internNodeType(node.Type())
		var flags uint8
		if node.IsNamed() {
			flags |= flatNodeFlagNamed
		}
		if node.IsError() || node.HasError() {
			flags |= flatNodeFlagError
		}
		if node.IsError() || node.IsMissing() {
			flags |= flatNodeFlagIsError
		}
		var parentField uint32
		if hasParent {
			parentField = parent
		}

		t.Types = append(t.Types, typeID)
		t.Parents = append(t.Parents, parentField)
		t.FirstChildren = append(t.FirstChildren, 0)
		t.NextSibs = append(t.NextSibs, 0)
		t.PrevSibs = append(t.PrevSibs, 0)
		t.StartBytes = append(t.StartBytes, node.StartByte())
		t.EndBytes = append(t.EndBytes, node.EndByte())
		t.StartRows = append(t.StartRows, saturateUint16(start.Row))
		t.StartCols = append(t.StartCols, saturateUint16(start.Column))
		t.ChildCounts = append(t.ChildCounts, saturateUint16(node.ChildCount()))
		t.NamedCounts = append(t.NamedCounts, saturateUint16(node.NamedChildCount()))
		t.Flags = append(t.Flags, flags)

		var prevChild uint32
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			childIdx := walk(child, idx, true)
			if t.FirstChildren[idx] == 0 {
				t.FirstChildren[idx] = childIdx
			}
			if prevChild != 0 {
				t.NextSibs[prevChild] = childIdx
				t.PrevSibs[childIdx] = prevChild
			}
			prevChild = childIdx
		}
		return idx
	}

	walk(root, 0, false)
	t.buildNodesByType()
	return normalizeBareReturnOperands(normalizeTopLevelAnnotations(t))
}

func estimateFlatCapacity(root *sitter.Node) int {
	if root == nil {
		return 0
	}
	capHint := int(root.ChildCount()) * 4
	if capHint < 1 {
		return 1
	}
	return capHint
}

func TestFlattenTreeMatchesReference(t *testing.T) {
	var files []string
	for _, root := range []string{"../../tests", "../../internal"} {
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && (strings.HasSuffix(p, ".kt") || strings.HasSuffix(p, ".kts") || strings.HasSuffix(p, ".gradle")) {
				files = append(files, p)
			}
			return nil
		})
	}
	if len(files) == 0 {
		t.Skip("no corpus files")
	}
	kotlin := GetKotlinParser()
	defer PutKotlinParser(kotlin)
	groovy := GetGroovyParser()
	defer PutGroovyParser(groovy)
	broken := [][]byte{
		[]byte("class A { fun ( { val = }"),
		[]byte("fun f() { if ( }"),
		[]byte(""),
		[]byte("package x\n\nimport a.b\n@Foo @Bar val x = 1"),
	}
	check := func(name string, parser *sitter.Parser, src []byte) {
		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err != nil || tree == nil {
			return
		}
		defer tree.Close()
		got := flattenTree(tree.RootNode())
		want := flattenTreeReference(tree.RootNode())
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: flattenTree differs from reference (len %d vs %d)", name, got.Len(), want.Len())
		}
	}
	for i, src := range broken {
		check("broken"+strconv.Itoa(i), kotlin, src)
	}
	t.Logf("comparing %d files", len(files))
	for _, p := range files {
		src, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		parser := kotlin
		if strings.HasSuffix(p, ".gradle") {
			parser = groovy
		}
		check(p, parser, src)
	}
}
