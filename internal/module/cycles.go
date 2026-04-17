package module

import "github.com/kaeawc/krit/internal/graph"

// FindCycles returns all module dependency cycles as strongly connected
// components with size > 1. Each cycle is a sorted list of module paths.
func (g *ModuleGraph) FindCycles() [][]string {
	dg := graph.NewGraph()
	for path, m := range g.Modules {
		dg.AddNode(path)
		for _, dep := range m.Dependencies {
			dg.AddEdge(path, dep.ModulePath)
		}
	}
	return graph.FindSCCs(dg)
}
