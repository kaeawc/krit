package graph

import "sort"

// FindSCCs returns strongly connected components with size > 1 using
// Tarjan's algorithm. Each SCC is sorted alphabetically, and the list
// of SCCs is sorted by first element for deterministic output.
func FindSCCs(g *Graph) [][]string {
	nodes := g.Nodes() // sorted

	index := 0
	indices := make(map[string]int, len(nodes))
	lowlink := make(map[string]int, len(nodes))
	onStack := make(map[string]bool, len(nodes))
	stack := make([]string, 0, len(nodes))
	var sccs [][]string

	var strongConnect func(string)
	strongConnect = func(node string) {
		index++
		indices[node] = index
		lowlink[node] = index
		stack = append(stack, node)
		onStack[node] = true

		for _, neighbor := range g.Neighbors(node) { // sorted
			if indices[neighbor] == 0 {
				strongConnect(neighbor)
				if lowlink[neighbor] < lowlink[node] {
					lowlink[node] = lowlink[neighbor]
				}
				continue
			}
			if onStack[neighbor] && indices[neighbor] < lowlink[node] {
				lowlink[node] = indices[neighbor]
			}
		}

		if lowlink[node] != indices[node] {
			return
		}

		var scc []string
		for {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[top] = false
			scc = append(scc, top)
			if top == node {
				break
			}
		}
		if len(scc) > 1 {
			sort.Strings(scc)
			sccs = append(sccs, scc)
		}
	}

	for _, node := range nodes {
		if indices[node] == 0 {
			strongConnect(node)
		}
	}

	sort.Slice(sccs, func(i, j int) bool {
		return sccs[i][0] < sccs[j][0]
	})

	return sccs
}
