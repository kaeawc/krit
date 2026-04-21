package scanner

import (
	"math"
	"sync"
	"sync/atomic"

	sitter "github.com/smacker/go-tree-sitter"
)

const (
	flatNodeFlagNamed uint8 = 1 << iota
	flatNodeFlagError
)

var (
	nodeTypeMu    sync.RWMutex
	NodeTypeTable []string
	nodeTypeIndex = make(map[string]uint16)

	// nodeTypeSnapshot is an atomically-updated pointer to a snapshot of
	// NodeTypeTable. Reads during dispatch use this for lock-free access —
	// the table only grows during parsing, so the snapshot is stable by the
	// time any rule calls FlatType(). Writes hold nodeTypeMu.
	nodeTypeSnapshot atomic.Pointer[[]string]
)

// FlatNode stores a tree-sitter node in a compact, cgo-free form.
// Size: 40 bytes. PrevSib is stored explicitly so FlatPrevSibling can be
// O(1); without it, prev-sibling access required walking from FirstChild
// which gave O(sibling_index) per call and O(N²) across adjacent callers.
type FlatNode struct {
	Type       uint16
	Parent     uint32
	FirstChild uint32
	NextSib    uint32
	PrevSib    uint32
	StartByte  uint32
	EndByte    uint32
	StartRow   uint16
	StartCol   uint16
	ChildCount uint16
	NamedCount uint16
	Flags      uint8
	_pad       uint8
}

// IsNamed reports whether this node is a named tree-sitter node.
func (n FlatNode) IsNamed() bool {
	return n.Flags&flatNodeFlagNamed != 0
}

// HasError reports whether this node or its subtree contains a parse error.
func (n FlatNode) HasError() bool {
	return n.Flags&flatNodeFlagError != 0
}

// TypeName resolves the node's interned type back to its string name.
func (n FlatNode) TypeName() string {
	return nodeTypeName(n.Type)
}

// FlatTree holds a preorder-flattened syntax tree.
type FlatTree struct {
	Nodes []FlatNode
}

func flattenTree(root *sitter.Node) *FlatTree {
	if root == nil {
		return &FlatTree{}
	}

	nodes := make([]FlatNode, 0, estimateFlatCapacity(root))
	var walk func(node *sitter.Node, parent uint32, hasParent bool) uint32
	walk = func(node *sitter.Node, parent uint32, hasParent bool) uint32 {
		idx := uint32(len(nodes))
		start := node.StartPoint()
		flatNode := FlatNode{
			Type:       internNodeType(node.Type()),
			StartByte:  node.StartByte(),
			EndByte:    node.EndByte(),
			StartRow:   saturateUint16(start.Row),
			StartCol:   saturateUint16(start.Column),
			ChildCount: saturateUint16(node.ChildCount()),
			NamedCount: saturateUint16(node.NamedChildCount()),
		}
		if hasParent {
			flatNode.Parent = parent
		}
		if node.IsNamed() {
			flatNode.Flags |= flatNodeFlagNamed
		}
		if node.IsError() || node.HasError() {
			flatNode.Flags |= flatNodeFlagError
		}
		nodes = append(nodes, flatNode)

		var prevChild uint32
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			childIdx := walk(child, idx, true)
			if nodes[idx].FirstChild == 0 {
				nodes[idx].FirstChild = childIdx
			}
			if prevChild != 0 {
				nodes[prevChild].NextSib = childIdx
				nodes[childIdx].PrevSib = prevChild
			}
			prevChild = childIdx
		}
		return idx
	}

	walk(root, 0, false)
	return &FlatTree{Nodes: nodes}
}

// FlatNodeText returns the source text spanned by the flattened node.
func FlatNodeText(tree *FlatTree, idx uint32, content []byte) string {
	return string(FlatNodeBytes(tree, idx, content))
}

// FlatNodeBytes returns the source bytes spanned by the flattened node.
// The returned slice aliases content and is only valid while content is live.
func FlatNodeBytes(tree *FlatTree, idx uint32, content []byte) []byte {
	if tree == nil || int(idx) >= len(tree.Nodes) {
		return nil
	}
	node := tree.Nodes[idx]
	return content[node.StartByte:node.EndByte]
}

// FlatNodeTextEquals reports whether the node's source text equals s.
// Zero-alloc: the compiler optimises string(b)==s to a direct byte comparison
// regardless of whether s is a constant or a variable (confirmed via escape
// analysis — the temporary string is stack-allocated, never escapes to heap).
func FlatNodeTextEquals(tree *FlatTree, idx uint32, content []byte, s string) bool {
	return string(FlatNodeBytes(tree, idx, content)) == s
}

// FlatNodeString returns an interned string for a flattened node.
func FlatNodeString(tree *FlatTree, idx uint32, content []byte, pool *StringPool) string {
	b := FlatNodeBytes(tree, idx, content)
	if len(b) == 0 {
		return ""
	}
	if pool == nil {
		return internBytes(b)
	}
	return pool.Intern(bytesToStringView(b))
}

// FlatWalkNodes calls fn for every node of the given type.
func FlatWalkNodes(tree *FlatTree, nodeType string, fn func(uint32)) {
	if tree == nil {
		return
	}
	typeID, ok := lookupNodeType(nodeType)
	if !ok {
		return
	}
	for i := range tree.Nodes {
		if tree.Nodes[i].Type == typeID {
			fn(uint32(i))
		}
	}
}

// FlatFindChild finds the first direct child with the given type.
// It returns 0 when no matching child exists.
func FlatFindChild(tree *FlatTree, parent uint32, childType string) uint32 {
	if tree == nil || int(parent) >= len(tree.Nodes) {
		return 0
	}
	typeID, ok := lookupNodeType(childType)
	if !ok {
		return 0
	}
	for child := tree.Nodes[parent].FirstChild; child != 0; child = tree.Nodes[child].NextSib {
		if tree.Nodes[child].Type == typeID {
			return child
		}
	}
	return 0
}

// FlatHasModifier checks whether a flattened declaration has the given modifier.
func FlatHasModifier(tree *FlatTree, idx uint32, content []byte, modifier string) bool {
	mods := FlatFindChild(tree, idx, "modifiers")
	if mods == 0 {
		return false
	}
	for child := tree.Nodes[mods].FirstChild; child != 0; child = tree.Nodes[child].NextSib {
		if bytesEqualString(FlatNodeBytes(tree, child, content), modifier) {
			return true
		}
		for grandChild := tree.Nodes[child].FirstChild; grandChild != 0; grandChild = tree.Nodes[grandChild].NextSib {
			if bytesEqualString(FlatNodeBytes(tree, grandChild, content), modifier) {
				return true
			}
		}
	}
	return false
}

func (f *File) FlatNodeBytes(idx uint32) []byte {
	return FlatNodeBytes(f.FlatTree, idx, f.Content)
}

func (f *File) FlatNodeText(idx uint32) string {
	return FlatNodeText(f.FlatTree, idx, f.Content)
}

func (f *File) FlatNodeTextEquals(idx uint32, s string) bool {
	return FlatNodeTextEquals(f.FlatTree, idx, f.Content, s)
}

func (f *File) FlatNodeString(idx uint32, pool *StringPool) string {
	return FlatNodeString(f.FlatTree, idx, f.Content, pool)
}

func (f *File) FlatFindChild(parent uint32, childType string) uint32 {
	return FlatFindChild(f.FlatTree, parent, childType)
}

func (f *File) FlatHasModifier(idx uint32, modifier string) bool {
	return FlatHasModifier(f.FlatTree, idx, f.Content, modifier)
}

func (f *File) FlatType(idx uint32) string {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Nodes) {
		return ""
	}
	return f.FlatTree.Nodes[idx].TypeName()
}

func (f *File) FlatChildCount(idx uint32) int {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Nodes) {
		return 0
	}
	return int(f.FlatTree.Nodes[idx].ChildCount)
}

func (f *File) FlatChild(parent uint32, childIdx int) uint32 {
	if f == nil || f.FlatTree == nil || int(parent) >= len(f.FlatTree.Nodes) || childIdx < 0 {
		return 0
	}
	child := f.FlatTree.Nodes[parent].FirstChild
	for i := 0; child != 0; i++ {
		if i == childIdx {
			return child
		}
		child = f.FlatTree.Nodes[child].NextSib
	}
	return 0
}

func (f *File) FlatNamedChildCount(idx uint32) int {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Nodes) {
		return 0
	}
	return int(f.FlatTree.Nodes[idx].NamedCount)
}

func (f *File) FlatNamedChild(parent uint32, childIdx int) uint32 {
	if f == nil || f.FlatTree == nil || int(parent) >= len(f.FlatTree.Nodes) || childIdx < 0 {
		return 0
	}
	namedIdx := 0
	for child := f.FlatTree.Nodes[parent].FirstChild; child != 0; child = f.FlatTree.Nodes[child].NextSib {
		if !f.FlatTree.Nodes[child].IsNamed() {
			continue
		}
		if namedIdx == childIdx {
			return child
		}
		namedIdx++
	}
	return 0
}

func (f *File) FlatForEachChild(parent uint32, fn func(uint32)) {
	if f == nil || f.FlatTree == nil || int(parent) >= len(f.FlatTree.Nodes) || fn == nil {
		return
	}
	for child := f.FlatTree.Nodes[parent].FirstChild; child != 0; child = f.FlatTree.Nodes[child].NextSib {
		fn(child)
	}
}

func (f *File) FlatHasChildOfType(parent uint32, childType string) bool {
	return f.FlatFindChild(parent, childType) != 0
}

func (f *File) FlatParent(idx uint32) (uint32, bool) {
	if f == nil || f.FlatTree == nil || idx == 0 || int(idx) >= len(f.FlatTree.Nodes) {
		return 0, false
	}
	return f.FlatTree.Nodes[idx].Parent, true
}

func (f *File) FlatNextSibling(idx uint32) (uint32, bool) {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Nodes) {
		return 0, false
	}
	next := f.FlatTree.Nodes[idx].NextSib
	if next == 0 {
		return 0, false
	}
	return next, true
}

func (f *File) FlatPrevSibling(idx uint32) (uint32, bool) {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Nodes) {
		return 0, false
	}
	prev := f.FlatTree.Nodes[idx].PrevSib
	if prev == 0 {
		return 0, false
	}
	return prev, true
}

// FlatFirstChild returns the first child index of parent, or 0 if parent has
// no children. O(1). Intended for linked-list iteration over children:
//
//	for c := file.FlatFirstChild(p); c != 0; c = file.FlatNextSib(c) {
//	    // ... use c ...
//	}
//
// Prefer this over `for i := 0; i < FlatChildCount(p); i++ { FlatChild(p, i) }`
// which is O(k) per child access and O(N²) across the full iteration.
func (f *File) FlatFirstChild(parent uint32) uint32 {
	if f == nil || f.FlatTree == nil || int(parent) >= len(f.FlatTree.Nodes) {
		return 0
	}
	return f.FlatTree.Nodes[parent].FirstChild
}

// FlatNextSib returns the next sibling index, or 0 if idx is the last child.
// O(1). Simpler variant of FlatNextSibling (which returns (uint32, bool))
// for the linked-list iteration idiom.
func (f *File) FlatNextSib(idx uint32) uint32 {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Nodes) {
		return 0
	}
	return f.FlatTree.Nodes[idx].NextSib
}

// FlatIsNamed reports whether the node at idx is a named tree-sitter node
// (i.e., not an anonymous punctuation / keyword token). O(1). Use inside
// linked-list child iteration to replicate the semantics of FlatNamedChild:
//
//	for c := file.FlatFirstChild(p); c != 0; c = file.FlatNextSib(c) {
//	    if !file.FlatIsNamed(c) { continue }
//	    // ... use c as a named child ...
//	}
func (f *File) FlatIsNamed(idx uint32) bool {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Nodes) {
		return false
	}
	return f.FlatTree.Nodes[idx].IsNamed()
}

func (f *File) FlatHasAncestorOfType(idx uint32, ancestorType string) bool {
	if f == nil || f.FlatTree == nil {
		return false
	}
	typeID, ok := lookupNodeType(ancestorType)
	if !ok {
		return false
	}
	for current, ok := f.FlatParent(idx); ok; current, ok = f.FlatParent(current) {
		if f.FlatTree.Nodes[current].Type == typeID {
			return true
		}
	}
	return false
}

func (f *File) FlatWalkNodes(root uint32, nodeType string, fn func(uint32)) {
	if f == nil || f.FlatTree == nil || fn == nil || int(root) >= len(f.FlatTree.Nodes) {
		return
	}
	typeID, ok := lookupNodeType(nodeType)
	if !ok {
		return
	}
	var walk func(uint32)
	walk = func(idx uint32) {
		if f.FlatTree.Nodes[idx].Type == typeID {
			fn(idx)
		}
		for child := f.FlatTree.Nodes[idx].FirstChild; child != 0; child = f.FlatTree.Nodes[child].NextSib {
			walk(child)
		}
	}
	walk(root)
}

func (f *File) FlatWalkAllNodes(root uint32, fn func(uint32)) {
	if f == nil || f.FlatTree == nil || fn == nil || int(root) >= len(f.FlatTree.Nodes) {
		return
	}
	var walk func(uint32)
	walk = func(idx uint32) {
		fn(idx)
		for child := f.FlatTree.Nodes[idx].FirstChild; child != 0; child = f.FlatTree.Nodes[child].NextSib {
			walk(child)
		}
	}
	walk(root)
}

func (f *File) FlatCountNodes(root uint32, nodeType string) int {
	count := 0
	f.FlatWalkNodes(root, nodeType, func(uint32) {
		count++
	})
	return count
}

func (f *File) FlatFindModifierNode(idx uint32, modifier string) uint32 {
	mods := f.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return 0
	}
	var found uint32
	f.FlatForEachChild(mods, func(child uint32) {
		if found != 0 {
			return
		}
		if bytesEqualString(f.FlatNodeBytes(child), modifier) {
			found = child
			return
		}
		f.FlatForEachChild(child, func(grandChild uint32) {
			if found == 0 && bytesEqualString(f.FlatNodeBytes(grandChild), modifier) {
				found = grandChild
			}
		})
	})
	return found
}

func (f *File) FlatStartByte(idx uint32) uint32 {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Nodes) {
		return 0
	}
	return f.FlatTree.Nodes[idx].StartByte
}

func (f *File) FlatEndByte(idx uint32) uint32 {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Nodes) {
		return 0
	}
	return f.FlatTree.Nodes[idx].EndByte
}

func (f *File) FlatRow(idx uint32) int {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Nodes) {
		return 0
	}
	return int(f.FlatTree.Nodes[idx].StartRow)
}

func (f *File) FlatCol(idx uint32) int {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Nodes) {
		return 0
	}
	return int(f.FlatTree.Nodes[idx].StartCol)
}

func (f *File) FlatNamedDescendantForByteRange(startByte, endByte uint32) (uint32, bool) {
	if f == nil || f.FlatTree == nil || len(f.FlatTree.Nodes) == 0 {
		return 0, false
	}
	best := uint32(0)
	bestSpan := uint32(math.MaxUint32)
	found := false
	for idx, node := range f.FlatTree.Nodes {
		if !node.IsNamed() {
			continue
		}
		if node.StartByte > startByte || node.EndByte < endByte {
			continue
		}
		span := node.EndByte - node.StartByte
		if !found || span < bestSpan {
			best = uint32(idx)
			bestSpan = span
			found = true
		}
	}
	return best, found
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

func saturateUint16(v uint32) uint16 {
	if v > math.MaxUint16 {
		return math.MaxUint16
	}
	return uint16(v)
}

func internNodeType(nodeType string) uint16 {
	nodeTypeMu.RLock()
	if idx, ok := nodeTypeIndex[nodeType]; ok {
		nodeTypeMu.RUnlock()
		return idx
	}
	nodeTypeMu.RUnlock()

	nodeTypeMu.Lock()
	defer nodeTypeMu.Unlock()
	if idx, ok := nodeTypeIndex[nodeType]; ok {
		return idx
	}
	if len(NodeTypeTable) >= math.MaxUint16+1 {
		panic("scanner: node type table overflow")
	}
	idx := uint16(len(NodeTypeTable))
	NodeTypeTable = append(NodeTypeTable, nodeType)
	nodeTypeIndex[nodeType] = idx
	// Publish a snapshot for lock-free reads during dispatch. The slice header
	// is copied so readers always see a consistent (ptr, len, cap) triple even
	// if a future append reallocates the backing array.
	snapshot := NodeTypeTable
	nodeTypeSnapshot.Store(&snapshot)
	return idx
}

// NodeTypeTableSize returns the current size of the node type table via the
// lock-free snapshot. Safe to call concurrently with internNodeType.
func NodeTypeTableSize() int {
	if p := nodeTypeSnapshot.Load(); p != nil {
		return len(*p)
	}
	return 0
}

func lookupNodeType(nodeType string) (uint16, bool) {
	nodeTypeMu.RLock()
	defer nodeTypeMu.RUnlock()
	idx, ok := nodeTypeIndex[nodeType]
	return idx, ok
}

// LookupFlatNodeType resolves a node type string to its flattened type ID.
func LookupFlatNodeType(nodeType string) (uint16, bool) {
	return lookupNodeType(nodeType)
}

func nodeTypeName(typeID uint16) string {
	// Fast path: load the atomically-published snapshot without taking any lock.
	// The snapshot is always set before any typeID for that entry can be observed
	// by a caller (internNodeType publishes before returning the ID).
	if p := nodeTypeSnapshot.Load(); p != nil {
		tbl := *p
		if int(typeID) < len(tbl) {
			return tbl[typeID]
		}
		return ""
	}
	// Fallback for the rare case where snapshot hasn't been published yet
	// (before the first internNodeType call).
	nodeTypeMu.RLock()
	defer nodeTypeMu.RUnlock()
	if int(typeID) >= len(NodeTypeTable) {
		return ""
	}
	return NodeTypeTable[typeID]
}
