package graph

import "testing"

func TestFanIn_EmptyGraph(t *testing.T) {
	g := NewGraph()
	in := FanIn(g)
	if len(in) != 0 {
		t.Fatalf("expected empty map, got %v", in)
	}
}

func TestFanIn_LinearChain(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	in := FanIn(g)
	assertDegree(t, "FanIn", in, "A", 0)
	assertDegree(t, "FanIn", in, "B", 1)
	assertDegree(t, "FanIn", in, "C", 1)
}

func TestFanOut_LinearChain(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	out := FanOut(g)
	assertDegree(t, "FanOut", out, "A", 1)
	assertDegree(t, "FanOut", out, "B", 1)
	assertDegree(t, "FanOut", out, "C", 0)
}

func TestFanIn_Star(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "B")
	g.AddEdge("A", "C")
	g.AddEdge("A", "D")

	in := FanIn(g)
	assertDegree(t, "FanIn", in, "A", 0)
	assertDegree(t, "FanIn", in, "B", 1)
	assertDegree(t, "FanIn", in, "C", 1)
	assertDegree(t, "FanIn", in, "D", 1)
}

func TestFanOut_Star(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "B")
	g.AddEdge("A", "C")
	g.AddEdge("A", "D")

	out := FanOut(g)
	assertDegree(t, "FanOut", out, "A", 3)
	assertDegree(t, "FanOut", out, "B", 0)
	assertDegree(t, "FanOut", out, "C", 0)
	assertDegree(t, "FanOut", out, "D", 0)
}

func TestFanOut_EmptyGraph(t *testing.T) {
	g := NewGraph()
	out := FanOut(g)
	if len(out) != 0 {
		t.Fatalf("expected empty map, got %v", out)
	}
}

func assertDegree(t *testing.T, label string, m map[string]int, node string, want int) {
	t.Helper()
	if got := m[node]; got != want {
		t.Errorf("%s(%q) = %d, want %d", label, node, got, want)
	}
}
