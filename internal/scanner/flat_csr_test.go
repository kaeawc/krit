package scanner

import (
	"slices"
	"testing"
)

// TestBuildNodesByType_EmptyTree asserts that an empty tree leaves the
// CSR posting list nil — callers rely on len(NodeTypeOffsets) == 0 to
// detect "no posting list" (e.g. FlatWalkNodes' fallback path).
func TestBuildNodesByType_EmptyTree(t *testing.T) {
	tree := &FlatTree{}
	tree.buildNodesByType()
	if tree.NodeTypeOffsets != nil {
		t.Errorf("expected nil NodeTypeOffsets for empty tree, got %v", tree.NodeTypeOffsets)
	}
	if tree.NodeTypeIndices != nil {
		t.Errorf("expected nil NodeTypeIndices for empty tree, got %v", tree.NodeTypeIndices)
	}
	if got := tree.NumNodeTypes(); got != 0 {
		t.Errorf("expected NumNodeTypes()=0 for empty tree, got %d", got)
	}
}

func TestBuildNodesByType_SingleType(t *testing.T) {
	typeID := internNodeType("identifier")
	tree := &FlatTree{Types: []uint16{typeID, typeID, typeID}}
	tree.buildNodesByType()
	if got := len(tree.NodeTypeOffsets); got != int(typeID)+2 {
		t.Fatalf("expected NodeTypeOffsets length %d, got %d", int(typeID)+2, got)
	}
	got := tree.NodesOfType(typeID)
	want := []uint32{0, 1, 2}
	if !slices.Equal(got, want) {
		t.Errorf("expected indices %v, got %v", want, got)
	}
}

// TestBuildNodesByType_DocumentOrderPreserved is the core correctness
// guard for the CSR scatter pass. The order of indices within a bucket
// must match document order — rules depend on this for stable dispatch
// ordering and reproducible findings.
func TestBuildNodesByType_DocumentOrderPreserved(t *testing.T) {
	aID := internNodeType("identifier")
	bID := internNodeType("integer_literal")
	cID := internNodeType("string_literal")
	tree := &FlatTree{Types: []uint16{
		aID, bID, aID, cID, aID, bID, cID, aID, bID,
	}}
	tree.buildNodesByType()

	cases := []struct {
		name   string
		typeID uint16
		want   []uint32
	}{
		{"identifier", aID, []uint32{0, 2, 4, 7}},
		{"integer_literal", bID, []uint32{1, 5, 8}},
		{"string_literal", cID, []uint32{3, 6}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tree.NodesOfType(tc.typeID)
			if !slices.Equal(got, tc.want) {
				t.Errorf("expected indices %v in document order, got %v", tc.want, got)
			}
		})
	}
}

// TestNodesOfType_OutOfRange asserts that NodesOfType is safe for
// typeIDs that do not appear in this tree. FlatWalkNodes relies on
// this when looking up a node type that was interned by another
// parser pass but doesn't appear in this particular file.
func TestNodesOfType_OutOfRange(t *testing.T) {
	typeID := internNodeType("identifier")
	tree := &FlatTree{Types: []uint16{typeID}}
	tree.buildNodesByType()

	beyond := uint16(len(tree.NodeTypeOffsets))
	if got := tree.NodesOfType(beyond); got != nil {
		t.Errorf("expected nil for typeID past max, got %v", got)
	}
	if got := tree.NodesOfType(^uint16(0)); got != nil {
		t.Errorf("expected nil for max uint16 typeID, got %v", got)
	}

	var nilTree *FlatTree
	if got := nilTree.NodesOfType(0); got != nil {
		t.Errorf("expected nil for nil tree, got %v", got)
	}
}

// TestBuildNodesByType_PartitionsTypes confirms that every node index
// appears in exactly one bucket and the union of buckets covers all
// nodes. This is the property a tree walk relies on: dispatching by
// type visits each node once and only once.
func TestBuildNodesByType_PartitionsTypes(t *testing.T) {
	root, _ := parseKotlin(t, `
package com.example

import kotlin.collections.List

class Container(val items: List<String>) {
    fun describe(): String {
        val parts = items.map { it.uppercase() }
        return parts.joinToString(", ")
    }
}
`)
	tree := flattenTree(root)
	if tree.Len() == 0 {
		t.Fatal("expected non-empty tree")
	}

	seen := make(map[uint32]bool, tree.Len())
	for typeID := uint16(0); int(typeID) < tree.NumNodeTypes(); typeID++ {
		for _, idx := range tree.NodesOfType(typeID) {
			if seen[idx] {
				t.Fatalf("index %d appears in multiple buckets", idx)
			}
			seen[idx] = true
			if got := tree.Types[idx]; got != typeID {
				t.Errorf("bucket %d contains index %d with type %d", typeID, idx, got)
			}
		}
	}
	if len(seen) != tree.Len() {
		t.Fatalf("expected every node (%d) covered by buckets, got %d", tree.Len(), len(seen))
	}
}

// TestBuildNodesByType_MatchesLinearScan checks the posting list
// against a brute-force linear scan over t.Types for every typeID,
// ensuring complete behavioural equivalence with the prior [][]uint32
// shape and with FlatWalkNodes' fallback path.
func TestBuildNodesByType_MatchesLinearScan(t *testing.T) {
	root, _ := parseKotlin(t, `
fun fizzbuzz(n: Int): String = when {
    n % 15 == 0 -> "FizzBuzz"
    n % 3 == 0 -> "Fizz"
    n % 5 == 0 -> "Buzz"
    else -> n.toString()
}
`)
	tree := flattenTree(root)
	if tree.Len() == 0 {
		t.Fatal("expected non-empty tree")
	}

	for typeID := uint16(0); int(typeID) < tree.NumNodeTypes(); typeID++ {
		var want []uint32
		for i, tID := range tree.Types {
			if tID == typeID {
				want = append(want, uint32(i))
			}
		}
		got := tree.NodesOfType(typeID)
		if !slices.Equal(got, want) {
			t.Errorf("typeID %d (%s): got %v, want %v",
				typeID, nodeTypeName(typeID), got, want)
		}
	}
}

// TestFlatWalkNodes_FallbackWithoutPostingList asserts the fallback
// path still works for ad-hoc trees that skip buildNodesByType. Rule
// authors hand-construct FlatTrees in unit tests; this path must keep
// working.
func TestFlatWalkNodes_FallbackWithoutPostingList(t *testing.T) {
	typeID := internNodeType("identifier")
	tree := &FlatTree{Types: []uint16{typeID, typeID + 1, typeID, typeID + 1, typeID}}
	// Deliberately do NOT call buildNodesByType.
	if len(tree.NodeTypeOffsets) != 0 {
		t.Fatal("preconditions violated: posting list should be empty")
	}

	var got []uint32
	FlatWalkNodes(tree, nodeTypeName(typeID), func(idx uint32) {
		got = append(got, idx)
	})
	want := []uint32{0, 2, 4}
	if !slices.Equal(got, want) {
		t.Errorf("expected fallback to yield %v, got %v", want, got)
	}
}

// TestFlatWalkNodes_PostingListMatchesFallback runs both code paths on
// the same tree and verifies the results match. Catches future drift
// between the posting-list and fallback dispatch paths.
func TestFlatWalkNodes_PostingListMatchesFallback(t *testing.T) {
	root, _ := parseKotlin(t, `
class A {
    fun b() = c().d()
    fun e() = f()
}
`)
	tree := flattenTree(root)
	fallback := &FlatTree{Types: append([]uint16(nil), tree.Types...)}

	for typeID := uint16(0); int(typeID) < tree.NumNodeTypes(); typeID++ {
		if len(tree.NodesOfType(typeID)) == 0 {
			continue
		}
		name := nodeTypeName(typeID)
		var fastResult, slowResult []uint32
		FlatWalkNodes(tree, name, func(idx uint32) { fastResult = append(fastResult, idx) })
		FlatWalkNodes(fallback, name, func(idx uint32) { slowResult = append(slowResult, idx) })
		if !slices.Equal(fastResult, slowResult) {
			t.Errorf("type %q: posting-list %v vs fallback %v", name, fastResult, slowResult)
		}
	}
}

// TestBuildNodesByType_AllocBudget locks in the 2-alloc-per-rebuild
// budget. Before CSR this was 2 + distinct_types (≈50 for typical
// Kotlin files); regressing back would reintroduce GC pressure on
// every warm cache hit. If this test starts failing, look for code
// that re-introduces per-bucket [][]uint32 slices or a third
// per-rebuild scratch slice.
func TestBuildNodesByType_AllocBudget(t *testing.T) {
	root, _ := parseKotlin(t, `
package com.example

import kotlin.collections.List

class Repository(
    private val items: List<String>,
    private val onChange: (String) -> Unit,
) {
    fun add(value: String) {
        if (value.isNotBlank()) {
            onChange(value.trim())
        }
    }

    fun describe(): String {
        return items.joinToString(prefix = "[", postfix = "]", separator = ", ") {
            it.uppercase()
        }
    }
}
`)
	tree := flattenTree(root)
	avg := testing.AllocsPerRun(50, func() {
		tree.buildNodesByType()
	})
	const budget = 2
	if avg > budget {
		t.Errorf("buildNodesByType allocated %.2f allocs/op, budget is %d", avg, budget)
	}
}

// BenchmarkBuildNodesByType reports allocs/op and ns/op so CI dashboards
// can track regressions over time. Pairs with the alloc-budget test
// above as a softer signal — the test is the hard gate.
func BenchmarkBuildNodesByType(b *testing.B) {
	root, _ := parseKotlinBench(b, `
package com.example

class Service(
    private val items: List<String>,
    private val cache: MutableMap<String, Int>,
) {
    fun process(input: String): String {
        val tokens = input.split(",").map { it.trim() }
        for (token in tokens) {
            cache[token] = (cache[token] ?: 0) + 1
        }
        return tokens.joinToString(separator = "|") { it.uppercase() }
    }
}
`)
	tree := flattenTree(root)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.NodeTypeOffsets = nil
		tree.NodeTypeIndices = nil
		tree.buildNodesByType()
	}
}
