package scanner

/*
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

// Layouts mirror the tree-sitter C API pinned by github.com/smacker/go-tree-sitter.
// The ts_* symbols are provided by that package's compiled C sources.
typedef struct TSTree TSTree;
typedef struct TSLanguage TSLanguage;
typedef struct { uint32_t row; uint32_t column; } TSPoint;
typedef struct { uint32_t context[4]; const void *id; const TSTree *tree; } TSNode;
typedef struct { const void *tree; const void *id; uint32_t context[3]; } TSTreeCursor;

extern TSTreeCursor ts_tree_cursor_new(TSNode node);
extern void ts_tree_cursor_delete(TSTreeCursor *self);
extern TSNode ts_tree_cursor_current_node(const TSTreeCursor *self);
extern bool ts_tree_cursor_goto_parent(TSTreeCursor *self);
extern bool ts_tree_cursor_goto_next_sibling(TSTreeCursor *self);
extern bool ts_tree_cursor_goto_first_child(TSTreeCursor *self);
extern uint16_t ts_node_symbol(TSNode self);
extern bool ts_node_is_named(TSNode self);
extern bool ts_node_is_missing(TSNode self);
extern bool ts_node_has_error(TSNode self);
extern uint32_t ts_node_child_count(TSNode self);
extern uint32_t ts_node_named_child_count(TSNode self);
extern uint32_t ts_node_start_byte(TSNode self);
extern uint32_t ts_node_end_byte(TSNode self);
extern TSPoint ts_node_start_point(TSNode self);
extern const TSLanguage *ts_tree_language(const TSTree *self);
extern uint32_t ts_language_symbol_count(const TSLanguage *self);
extern const char *ts_language_symbol_name(const TSLanguage *self, uint16_t symbol);

#define KRIT_ERROR_SYMBOL 65535

static uint32_t krit_count_nodes(TSNode root) {
	TSTreeCursor cur = ts_tree_cursor_new(root);
	uint32_t n = 1;
	for (;;) {
		if (ts_tree_cursor_goto_first_child(&cur)) { n++; continue; }
		for (;;) {
			if (ts_tree_cursor_goto_next_sibling(&cur)) { n++; break; }
			if (!ts_tree_cursor_goto_parent(&cur)) { ts_tree_cursor_delete(&cur); return n; }
		}
	}
}

static inline uint16_t krit_sat16(uint32_t v) { return v > 65535 ? 65535 : (uint16_t)v; }

static void krit_visit(TSNode node, uint32_t i, uint32_t parent,
	uint16_t *sym, uint32_t *parents, uint32_t *sb, uint32_t *eb,
	uint16_t *rows, uint16_t *cols, uint16_t *cc, uint16_t *nc, uint8_t *flags) {
	uint16_t s = ts_node_symbol(node);
	TSPoint p = ts_node_start_point(node);
	uint8_t f = 0;
	if (ts_node_is_named(node)) f |= 1;
	bool isErr = s == KRIT_ERROR_SYMBOL;
	if (isErr || ts_node_has_error(node)) f |= 2;
	if (isErr || ts_node_is_missing(node)) f |= 4;
	sym[i] = s;
	parents[i] = parent;
	sb[i] = ts_node_start_byte(node);
	eb[i] = ts_node_end_byte(node);
	rows[i] = krit_sat16(p.row);
	cols[i] = krit_sat16(p.column);
	cc[i] = krit_sat16(ts_node_child_count(node));
	nc[i] = krit_sat16(ts_node_named_child_count(node));
	flags[i] = f;
}

// krit_fill writes the preorder node arrays. Returns 0 on success, 1 on allocation failure.
static int krit_fill(TSNode root,
	uint16_t *sym, uint32_t *parents, uint32_t *first, uint32_t *next, uint32_t *prev,
	uint32_t *sb, uint32_t *eb, uint16_t *rows, uint16_t *cols, uint16_t *cc, uint16_t *nc, uint8_t *flags) {
	uint32_t depthCap = 256;
	uint32_t *stack = (uint32_t *)malloc(depthCap * sizeof(uint32_t));
	if (!stack) return 1;
	TSTreeCursor cur = ts_tree_cursor_new(root);
	uint32_t n = 0;
	uint32_t depth = 0;
	stack[0] = 0;
	krit_visit(root, n++, 0, sym, parents, sb, eb, rows, cols, cc, nc, flags);
	for (;;) {
		uint32_t prevSib = 0;
		bool moved = false;
		if (ts_tree_cursor_goto_first_child(&cur)) {
			depth++;
			if (depth >= depthCap) {
				depthCap *= 2;
				uint32_t *grown = (uint32_t *)realloc(stack, depthCap * sizeof(uint32_t));
				if (!grown) { free(stack); ts_tree_cursor_delete(&cur); return 1; }
				stack = grown;
			}
			stack[depth] = 0;
			moved = true;
		} else {
			for (;;) {
				if (ts_tree_cursor_goto_next_sibling(&cur)) { prevSib = stack[depth]; moved = true; break; }
				if (!ts_tree_cursor_goto_parent(&cur)) break;
				depth--;
			}
		}
		if (!moved) break;
		uint32_t i = n++;
		uint32_t parent = stack[depth - 1];
		krit_visit(ts_tree_cursor_current_node(&cur), i, parent, sym, parents, sb, eb, rows, cols, cc, nc, flags);
		if (first[parent] == 0) first[parent] = i;
		if (prevSib != 0) { next[prevSib] = i; prev[i] = prevSib; }
		stack[depth] = i;
	}
	ts_tree_cursor_delete(&cur);
	free(stack);
	return 0;
}

static uint32_t krit_symbol_count(TSNode root) { return ts_language_symbol_count(ts_tree_language(root.tree)); }
static const char *krit_symbol_name(TSNode root, uint16_t symbol) { return ts_language_symbol_name(ts_tree_language(root.tree), symbol); }
static uintptr_t krit_language_id(TSNode root) { return (uintptr_t)ts_tree_language(root.tree); }
*/
import "C"

import (
	"sync"
	"sync/atomic"
	"unsafe"

	sitter "github.com/smacker/go-tree-sitter"
)

// symbolTypeCache maps a grammar's tree-sitter symbols to process-wide node
// type IDs. Entries hold id+1 so zero means "not yet interned"; the last slot
// is the ERROR symbol.
type symbolTypeCache struct {
	ids []atomic.Uint32
}

var (
	symbolTypeCachesMu sync.RWMutex
	symbolTypeCaches   = map[uintptr]*symbolTypeCache{}
)

func symbolTypesFor(root C.TSNode) *symbolTypeCache {
	lang := uintptr(C.krit_language_id(root))
	symbolTypeCachesMu.RLock()
	c := symbolTypeCaches[lang]
	symbolTypeCachesMu.RUnlock()
	if c != nil {
		return c
	}
	symbolTypeCachesMu.Lock()
	defer symbolTypeCachesMu.Unlock()
	if c = symbolTypeCaches[lang]; c == nil {
		c = &symbolTypeCache{ids: make([]atomic.Uint32, int(C.krit_symbol_count(root))+1)}
		symbolTypeCaches[lang] = c
	}
	return c
}

// typeID returns the interned node type ID for symbol, interning it on first use.
func (c *symbolTypeCache) typeID(root C.TSNode, symbol uint16) uint16 {
	slot := int(symbol)
	switch {
	case symbol == 0xFFFF:
		slot = len(c.ids) - 1
	case slot >= len(c.ids)-1:
		return internNodeType(C.GoString(C.krit_symbol_name(root, C.uint16_t(symbol))))
	}
	if v := c.ids[slot].Load(); v != 0 {
		return uint16(v - 1)
	}
	id := internNodeType(C.GoString(C.krit_symbol_name(root, C.uint16_t(symbol))))
	c.ids[slot].Store(uint32(id) + 1)
	return id
}

// flattenTree converts a tree-sitter tree into the struct-of-arrays FlatTree
// with one cgo call to count and one to fill, instead of several per node.
func flattenTree(root *sitter.Node) *FlatTree {
	if root == nil {
		return &FlatTree{}
	}
	// sitter.Node begins with its C.TSNode, so the first 32 bytes are the node handle.
	node := *(*C.TSNode)(unsafe.Pointer(root))
	n := int(C.krit_count_nodes(node))
	t := &FlatTree{
		Types:         make([]uint16, n),
		Parents:       make([]uint32, n),
		FirstChildren: make([]uint32, n),
		NextSibs:      make([]uint32, n),
		PrevSibs:      make([]uint32, n),
		StartBytes:    make([]uint32, n),
		EndBytes:      make([]uint32, n),
		StartRows:     make([]uint16, n),
		StartCols:     make([]uint16, n),
		ChildCounts:   make([]uint16, n),
		NamedCounts:   make([]uint16, n),
		Flags:         make([]uint8, n),
	}
	if rc := C.krit_fill(node,
		(*C.uint16_t)(unsafe.Pointer(&t.Types[0])),
		(*C.uint32_t)(unsafe.Pointer(&t.Parents[0])),
		(*C.uint32_t)(unsafe.Pointer(&t.FirstChildren[0])),
		(*C.uint32_t)(unsafe.Pointer(&t.NextSibs[0])),
		(*C.uint32_t)(unsafe.Pointer(&t.PrevSibs[0])),
		(*C.uint32_t)(unsafe.Pointer(&t.StartBytes[0])),
		(*C.uint32_t)(unsafe.Pointer(&t.EndBytes[0])),
		(*C.uint16_t)(unsafe.Pointer(&t.StartRows[0])),
		(*C.uint16_t)(unsafe.Pointer(&t.StartCols[0])),
		(*C.uint16_t)(unsafe.Pointer(&t.ChildCounts[0])),
		(*C.uint16_t)(unsafe.Pointer(&t.NamedCounts[0])),
		(*C.uint8_t)(unsafe.Pointer(&t.Flags[0])),
	); rc != 0 {
		panic("scanner: out of memory flattening tree")
	}

	// Types holds raw grammar symbols until mapped to interned type IDs.
	cache := symbolTypesFor(node)
	for i, sym := range t.Types {
		t.Types[i] = cache.typeID(node, sym)
	}

	t.buildNodesByType()
	return normalizeBareReturnOperands(normalizeTopLevelAnnotations(t))
}
