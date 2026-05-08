package graph

// FanIn returns the in-degree for every node in the graph.
func FanIn(g *Graph) map[string]int {
	in := make(map[string]int, len(g.nodes))
	for n := range g.nodes {
		in[n] = 0
	}
	for _, targets := range g.edges {
		for _, t := range targets {
			in[t]++
		}
	}
	return in
}

// FanOut returns the out-degree for every node in the graph.
func FanOut(g *Graph) map[string]int {
	out := make(map[string]int, len(g.nodes))
	for n := range g.nodes {
		out[n] = 0
	}
	for src, targets := range g.edges {
		out[src] += len(targets)
	}
	return out
}
