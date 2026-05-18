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

// FlatNode is a value-type view of a single flattened node. The canonical
// storage is the parallel slices on FlatTree (struct-of-arrays); this
// view materializes those slices into a 40-byte value for callers that
// prefer struct semantics. Hot paths should index FlatTree's parallel
// slices directly to avoid the per-access struct construction.
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

// FlatTree holds a preorder-flattened syntax tree in struct-of-arrays form.
// Field arrays are parallel: each is the same length, indexed by node ID
// in document order. PrevSibs is stored explicitly so prev-sibling access
// is O(1); without it, walking from FirstChild would be O(sibling_index)
// per call and O(N²) across adjacent callers.
//
// NodeTypeOffsets/NodeTypeIndices form a CSR (compressed sparse row)
// posting list indexed by node type ID, giving direct access to all
// nodes of a given type without a full tree walk. The dispatcher uses
// this to skip nodes whose type has no subscribed rules.
type FlatTree struct {
	Types         []uint16
	Parents       []uint32
	FirstChildren []uint32
	NextSibs      []uint32
	PrevSibs      []uint32
	StartBytes    []uint32
	EndBytes      []uint32
	StartRows     []uint16
	StartCols     []uint16
	ChildCounts   []uint16
	NamedCounts   []uint16
	Flags         []uint8

	// NodeTypeOffsets and NodeTypeIndices form a CSR posting list:
	// nodes of type t are at NodeTypeIndices[NodeTypeOffsets[t]:NodeTypeOffsets[t+1]]
	// in document order. NodeTypeOffsets has length maxType+2 with the
	// trailing entry equal to len(NodeTypeIndices) so [t:t+1] slicing
	// works for every valid typeID. Both are nil for an empty tree or
	// an ad-hoc tree that never had the posting list built.
	NodeTypeOffsets []uint32
	NodeTypeIndices []uint32
}

// NodesOfType returns the indices of every node with the given type ID
// in document order. Zero-allocation: the result is a slice view into
// NodeTypeIndices. Returns nil when the posting list is absent or
// typeID is out of range.
func (t *FlatTree) NodesOfType(typeID uint16) []uint32 {
	if t == nil {
		return nil
	}
	off := t.NodeTypeOffsets
	if int(typeID)+1 >= len(off) {
		return nil
	}
	return t.NodeTypeIndices[off[typeID]:off[typeID+1]]
}

// NumNodeTypes returns the number of type buckets in the posting list
// (maxType+1). Zero when the posting list has not been built.
func (t *FlatTree) NumNodeTypes() int {
	if t == nil || len(t.NodeTypeOffsets) == 0 {
		return 0
	}
	return len(t.NodeTypeOffsets) - 1
}

// Len returns the number of nodes in the tree. Safe on a nil receiver.
func (t *FlatTree) Len() int {
	if t == nil {
		return 0
	}
	return len(t.Types)
}

// Node materializes a FlatNode value at the given index. Hot paths
// should prefer direct field access via t.Types[idx], etc.
func (t *FlatTree) Node(idx uint32) FlatNode {
	if t == nil || int(idx) >= len(t.Types) {
		return FlatNode{}
	}
	return FlatNode{
		Type:       t.Types[idx],
		Parent:     t.Parents[idx],
		FirstChild: t.FirstChildren[idx],
		NextSib:    t.NextSibs[idx],
		PrevSib:    t.PrevSibs[idx],
		StartByte:  t.StartBytes[idx],
		EndByte:    t.EndBytes[idx],
		StartRow:   t.StartRows[idx],
		StartCol:   t.StartCols[idx],
		ChildCount: t.ChildCounts[idx],
		NamedCount: t.NamedCounts[idx],
		Flags:      t.Flags[idx],
	}
}

// NodeIsNamed reports whether the node at idx is a named tree-sitter
// node. O(1) field read against t.Flags — preferred over Node(idx).IsNamed()
// in hot paths to avoid materializing the full FlatNode value.
func (t *FlatTree) NodeIsNamed(idx uint32) bool {
	if t == nil || int(idx) >= len(t.Flags) {
		return false
	}
	return t.Flags[idx]&flatNodeFlagNamed != 0
}

func flattenTree(root *sitter.Node) *FlatTree {
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
	return t
}

// buildNodesByType reconstructs the CSR posting list from t.Types.
// Used by the parse cache after remapping local type IDs back to the
// process-global table.
//
// Counting sort with offsets-as-write-cursor. Forward iteration in the
// scatter pass outperformed the textbook reverse-scatter variant in
// benchmarks (better branch prediction / instruction-level parallelism),
// at the cost of a final right-shift restore pass.
func (t *FlatTree) buildNodesByType() {
	if t == nil || len(t.Types) == 0 {
		t.NodeTypeOffsets = nil
		t.NodeTypeIndices = nil
		return
	}
	var maxType uint16
	for _, tID := range t.Types {
		if tID > maxType {
			maxType = tID
		}
	}
	// Length maxType+2 so the trailing entry is the closing bound and
	// callers can slice indices[offsets[t]:offsets[t+1]] for any
	// valid typeID without a length check.
	offsets := make([]uint32, int(maxType)+2)
	for _, tID := range t.Types {
		offsets[int(tID)+1]++
	}
	for i := 1; i < len(offsets); i++ {
		offsets[i] += offsets[i-1]
	}
	indices := make([]uint32, len(t.Types))
	// Use offsets as a write cursor; restore by shifting right one
	// slot after the scatter. Post-scatter offsets[t-1] equals the
	// original offsets[t], so a right shift recovers the bucket
	// starts. Trailing offsets[maxType+1] stays put — no bucket
	// maxType+1 exists, so it was never advanced.
	for i, tID := range t.Types {
		slot := offsets[tID]
		indices[slot] = uint32(i)
		offsets[tID] = slot + 1
	}
	for i := len(offsets) - 1; i > 0; i-- {
		offsets[i] = offsets[i-1]
	}
	offsets[0] = 0
	t.NodeTypeOffsets = offsets
	t.NodeTypeIndices = indices
}

// FlatNodeText returns the source text spanned by the flattened node.
func FlatNodeText(tree *FlatTree, idx uint32, content []byte) string {
	return string(FlatNodeBytes(tree, idx, content))
}

// FlatNodeBytes returns the source bytes spanned by the flattened node.
// The returned slice aliases content and is only valid while content is live.
func FlatNodeBytes(tree *FlatTree, idx uint32, content []byte) []byte {
	if tree == nil || int(idx) >= len(tree.Types) {
		return nil
	}
	return content[tree.StartBytes[idx]:tree.EndBytes[idx]]
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

// FlatWalkNodes calls fn for every node of the given type. Uses the
// CSR posting list when available for direct O(matches) access; falls
// back to a linear scan over t.Types when the posting list has not
// been built (e.g. ad-hoc test trees).
func FlatWalkNodes(tree *FlatTree, nodeType string, fn func(uint32)) {
	if tree == nil {
		return
	}
	typeID, ok := lookupNodeType(nodeType)
	if !ok {
		return
	}
	if len(tree.NodeTypeOffsets) > 0 {
		for _, idx := range tree.NodesOfType(typeID) {
			fn(idx)
		}
		return
	}
	for i, t := range tree.Types {
		if t == typeID {
			fn(uint32(i))
		}
	}
}

// FlatFindChild finds the first direct child with the given type.
// The second return reports whether a matching child was found; when false,
// the first return is 0 and must not be used as a node index. Before this
// returned a bare uint32 and conflated "not found" with "node 0" (the
// source_file root), which silently produced whole-file source reads when
// the result was passed to FlatNodeText.
func FlatFindChild(tree *FlatTree, parent uint32, childType string) (uint32, bool) {
	if tree == nil || int(parent) >= len(tree.Types) {
		return 0, false
	}
	typeID, ok := lookupNodeType(childType)
	if !ok {
		return 0, false
	}
	for child := tree.FirstChildren[parent]; child != 0; child = tree.NextSibs[child] {
		if tree.Types[child] == typeID {
			return child, true
		}
	}
	return 0, false
}

// FlatHasModifier checks whether a flattened declaration has the given modifier.
func FlatHasModifier(tree *FlatTree, idx uint32, content []byte, modifier string) bool {
	mods, ok := FlatFindChild(tree, idx, "modifiers")
	if !ok {
		return false
	}
	for child := tree.FirstChildren[mods]; child != 0; child = tree.NextSibs[child] {
		if bytesEqualString(FlatNodeBytes(tree, child, content), modifier) {
			return true
		}
		for grandChild := tree.FirstChildren[child]; grandChild != 0; grandChild = tree.NextSibs[grandChild] {
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

func (f *File) FlatFindChild(parent uint32, childType string) (uint32, bool) {
	return FlatFindChild(f.FlatTree, parent, childType)
}

// FlatChildTextOrEmpty returns the text of the first child of the given type,
// or "" if no such child exists. Replaces the sentinel-zero pattern where
// FlatNodeText(FlatFindChild(...)) silently returned the entire file source.
func (f *File) FlatChildTextOrEmpty(parent uint32, childType string) string {
	idx, ok := f.FlatFindChild(parent, childType)
	if !ok {
		return ""
	}
	return f.FlatNodeText(idx)
}

// FlatChildBytesOrNil mirrors FlatChildTextOrEmpty for the []byte form.
func (f *File) FlatChildBytesOrNil(parent uint32, childType string) []byte {
	idx, ok := f.FlatFindChild(parent, childType)
	if !ok {
		return nil
	}
	return f.FlatNodeBytes(idx)
}

func (f *File) FlatHasModifier(idx uint32, modifier string) bool {
	return FlatHasModifier(f.FlatTree, idx, f.Content, modifier)
}

func (f *File) FlatType(idx uint32) string {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Types) {
		return ""
	}
	return nodeTypeName(f.FlatTree.Types[idx])
}

func (f *File) FlatChildCount(idx uint32) int {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Types) {
		return 0
	}
	return int(f.FlatTree.ChildCounts[idx])
}

func (f *File) FlatChild(parent uint32, childIdx int) uint32 {
	if f == nil || f.FlatTree == nil || int(parent) >= len(f.FlatTree.Types) || childIdx < 0 {
		return 0
	}
	t := f.FlatTree
	child := t.FirstChildren[parent]
	for i := 0; child != 0; i++ {
		if i == childIdx {
			return child
		}
		child = t.NextSibs[child]
	}
	return 0
}

func (f *File) FlatNamedChildCount(idx uint32) int {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Types) {
		return 0
	}
	return int(f.FlatTree.NamedCounts[idx])
}

func (f *File) FlatNamedChild(parent uint32, childIdx int) uint32 {
	if f == nil || f.FlatTree == nil || int(parent) >= len(f.FlatTree.Types) || childIdx < 0 {
		return 0
	}
	t := f.FlatTree
	namedIdx := 0
	for child := t.FirstChildren[parent]; child != 0; child = t.NextSibs[child] {
		if t.Flags[child]&flatNodeFlagNamed == 0 {
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
	if f == nil || f.FlatTree == nil || int(parent) >= len(f.FlatTree.Types) || fn == nil {
		return
	}
	t := f.FlatTree
	for child := t.FirstChildren[parent]; child != 0; child = t.NextSibs[child] {
		fn(child)
	}
}

func (f *File) FlatHasChildOfType(parent uint32, childType string) bool {
	_, ok := f.FlatFindChild(parent, childType)
	return ok
}

func (f *File) FlatParent(idx uint32) (uint32, bool) {
	if f == nil || f.FlatTree == nil || idx == 0 || int(idx) >= len(f.FlatTree.Types) {
		return 0, false
	}
	return f.FlatTree.Parents[idx], true
}

func (f *File) FlatNextSibling(idx uint32) (uint32, bool) {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Types) {
		return 0, false
	}
	next := f.FlatTree.NextSibs[idx]
	if next == 0 {
		return 0, false
	}
	return next, true
}

func (f *File) FlatPrevSibling(idx uint32) (uint32, bool) {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Types) {
		return 0, false
	}
	prev := f.FlatTree.PrevSibs[idx]
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
	if f == nil || f.FlatTree == nil || int(parent) >= len(f.FlatTree.Types) {
		return 0
	}
	return f.FlatTree.FirstChildren[parent]
}

// FlatNextSib returns the next sibling index, or 0 if idx is the last child.
// O(1). Simpler variant of FlatNextSibling (which returns (uint32, bool))
// for the linked-list iteration idiom.
func (f *File) FlatNextSib(idx uint32) uint32 {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Types) {
		return 0
	}
	return f.FlatTree.NextSibs[idx]
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
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Types) {
		return false
	}
	return f.FlatTree.Flags[idx]&flatNodeFlagNamed != 0
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
		if f.FlatTree.Types[current] == typeID {
			return true
		}
	}
	return false
}

func (f *File) FlatHasAnyAncestorOfType(idx uint32, ancestorTypes ...uint16) bool {
	if f == nil || f.FlatTree == nil || len(ancestorTypes) == 0 {
		return false
	}
	for current, ok := f.FlatParent(idx); ok; current, ok = f.FlatParent(current) {
		currentType := f.FlatTree.Types[current]
		for _, ancestorType := range ancestorTypes {
			if currentType == ancestorType {
				return true
			}
		}
	}
	return false
}

func (f *File) FlatWalkNodes(root uint32, nodeType string, fn func(uint32)) {
	if f == nil || f.FlatTree == nil || fn == nil || int(root) >= len(f.FlatTree.Types) {
		return
	}
	typeID, ok := lookupNodeType(nodeType)
	if !ok {
		return
	}
	t := f.FlatTree
	var walk func(uint32)
	walk = func(idx uint32) {
		if t.Types[idx] == typeID {
			fn(idx)
		}
		for child := t.FirstChildren[idx]; child != 0; child = t.NextSibs[child] {
			walk(child)
		}
	}
	walk(root)
}

func (f *File) FlatWalkAllNodes(root uint32, fn func(uint32)) {
	if f == nil || f.FlatTree == nil || fn == nil || int(root) >= len(f.FlatTree.Types) {
		return
	}
	t := f.FlatTree
	var walk func(uint32)
	walk = func(idx uint32) {
		fn(idx)
		for child := t.FirstChildren[idx]; child != 0; child = t.NextSibs[child] {
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
	mods, ok := f.FlatFindChild(idx, "modifiers")
	if !ok {
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
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Types) {
		return 0
	}
	return f.FlatTree.StartBytes[idx]
}

func (f *File) FlatEndByte(idx uint32) uint32 {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Types) {
		return 0
	}
	return f.FlatTree.EndBytes[idx]
}

func (f *File) FlatRow(idx uint32) int {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Types) {
		return 0
	}
	return int(f.FlatTree.StartRows[idx])
}

func (f *File) FlatCol(idx uint32) int {
	if f == nil || f.FlatTree == nil || int(idx) >= len(f.FlatTree.Types) {
		return 0
	}
	return int(f.FlatTree.StartCols[idx])
}

func (f *File) FlatNamedDescendantForByteRange(startByte, endByte uint32) (uint32, bool) {
	if f == nil || f.FlatTree == nil || len(f.FlatTree.Types) == 0 {
		return 0, false
	}
	t := f.FlatTree
	best := uint32(0)
	bestSpan := uint32(math.MaxUint32)
	found := false
	for idx := range t.Types {
		if t.Flags[idx]&flatNodeFlagNamed == 0 {
			continue
		}
		sb := t.StartBytes[idx]
		eb := t.EndBytes[idx]
		if sb > startByte || eb < endByte {
			continue
		}
		span := eb - sb
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
