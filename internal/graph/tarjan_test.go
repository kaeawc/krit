package graph

import (
	"testing"
)

func TestFindSCCs_EmptyGraph(t *testing.T) {
	g := NewGraph()
	sccs := FindSCCs(g)
	if len(sccs) != 0 {
		t.Fatalf("expected no SCCs, got %v", sccs)
	}
}

func TestFindSCCs_SingleNodeNoEdges(t *testing.T) {
	g := NewGraph()
	g.AddNode("A")
	sccs := FindSCCs(g)
	if len(sccs) != 0 {
		t.Fatalf("expected no SCCs for single node, got %v", sccs)
	}
}

func TestFindSCCs_DAG(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	sccs := FindSCCs(g)
	if len(sccs) != 0 {
		t.Fatalf("expected no SCCs for DAG, got %v", sccs)
	}
}

func TestFindSCCs_SimpleCycle(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "A")
	sccs := FindSCCs(g)
	if len(sccs) != 1 {
		t.Fatalf("expected 1 SCC, got %d: %v", len(sccs), sccs)
	}
	assertSCC(t, sccs[0], []string{"A", "B"})
}

func TestFindSCCs_TriangleCycle(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "A")
	sccs := FindSCCs(g)
	if len(sccs) != 1 {
		t.Fatalf("expected 1 SCC, got %d: %v", len(sccs), sccs)
	}
	assertSCC(t, sccs[0], []string{"A", "B", "C"})
}

func TestFindSCCs_TwoDisjointCycles(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "A")
	g.AddEdge("X", "Y")
	g.AddEdge("Y", "X")
	sccs := FindSCCs(g)
	if len(sccs) != 2 {
		t.Fatalf("expected 2 SCCs, got %d: %v", len(sccs), sccs)
	}
	assertSCC(t, sccs[0], []string{"A", "B"})
	assertSCC(t, sccs[1], []string{"X", "Y"})
}

func TestFindSCCs_SelfLoop(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "A")
	sccs := FindSCCs(g)
	if len(sccs) != 0 {
		t.Fatalf("expected no SCCs for self-loop (size 1), got %v", sccs)
	}
}

func TestFindSCCs_Complex(t *testing.T) {
	// Graph with one cycle (A-B-C) and DAG nodes hanging off it.
	g := NewGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "A") // cycle A-B-C
	g.AddEdge("C", "D") // D is a DAG tail
	g.AddEdge("E", "A") // E feeds into cycle
	g.AddEdge("F", "G")
	g.AddEdge("G", "F") // separate cycle F-G

	sccs := FindSCCs(g)
	if len(sccs) != 2 {
		t.Fatalf("expected 2 SCCs, got %d: %v", len(sccs), sccs)
	}
	assertSCC(t, sccs[0], []string{"A", "B", "C"})
	assertSCC(t, sccs[1], []string{"F", "G"})
}

func assertSCC(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("SCC length mismatch: got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("SCC mismatch at index %d: got %v, want %v", i, got, want)
		}
	}
}
