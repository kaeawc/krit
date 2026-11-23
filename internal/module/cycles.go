package module

import "github.com/kaeawc/krit/internal/graph"

// FindCycles returns all module dependency cycles as strongly connected
// components with size > 1. Each cycle is a sorted list of module paths.
// Test dependencies are left out: a test source set compiles against the
// main output of its own module, so `:a` testing with `:b` while `:b`
// depends on `:a` is not a cycle Gradle rejects.
func (g *Graph) FindCycles() [][]string {
	dg := graph.NewGraph()
	for path, m := range g.Modules {
		dg.AddNode(path)
		for _, dep := range m.Dependencies {
			if !dep.Test {
				dg.AddEdge(path, dep.ModulePath)
			}
		}
	}
	return graph.FindSCCs(dg)
}
