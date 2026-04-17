package graph

import "sort"

// Graph is a directed graph with string-labeled nodes.
type Graph struct {
	edges map[string][]string
	nodes map[string]struct{}
}

// NewGraph creates an empty directed graph.
func NewGraph() *Graph {
	return &Graph{
		edges: make(map[string][]string),
		nodes: make(map[string]struct{}),
	}
}

// AddNode adds a node to the graph. Duplicate calls are safe.
func (g *Graph) AddNode(name string) {
	g.nodes[name] = struct{}{}
}

// AddEdge adds a directed edge from -> to, creating both nodes if absent.
func (g *Graph) AddEdge(from, to string) {
	g.nodes[from] = struct{}{}
	g.nodes[to] = struct{}{}
	g.edges[from] = append(g.edges[from], to)
}

// Nodes returns all node names sorted alphabetically.
func (g *Graph) Nodes() []string {
	out := make([]string, 0, len(g.nodes))
	for n := range g.nodes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Neighbors returns the sorted successors of a node.
func (g *Graph) Neighbors(node string) []string {
	adj := g.edges[node]
	if len(adj) == 0 {
		return nil
	}
	out := make([]string, len(adj))
	copy(out, adj)
	sort.Strings(out)
	return out
}
