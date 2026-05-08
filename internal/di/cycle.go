package di

import "sort"

type Cycle struct {
	Bindings []*Binding
}

func (g *Graph) FindCycles() []Cycle {
	if g == nil {
		return nil
	}
	var (
		index   int
		stack   []*Binding
		onStack = map[string]bool{}
		indices = map[string]int{}
		lowlink = map[string]int{}
		cycles  []Cycle
	)
	for _, b := range g.sortedBindings("") {
		if _, ok := indices[b.FQN]; !ok {
			strongConnect(g, b, &index, &stack, onStack, indices, lowlink, &cycles)
		}
	}
	sort.Slice(cycles, func(i, j int) bool {
		return cycleKey(cycles[i]) < cycleKey(cycles[j])
	})
	return cycles
}

func strongConnect(g *Graph, v *Binding, index *int, stack *[]*Binding, onStack map[string]bool, indices, lowlink map[string]int, cycles *[]Cycle) {
	indices[v.FQN] = *index
	lowlink[v.FQN] = *index
	*index = *index + 1
	*stack = append(*stack, v)
	onStack[v.FQN] = true

	for _, dep := range v.Dependencies {
		if dep.Deferred || dep.Target == "" {
			continue
		}
		w := g.Binding(dep.Target)
		if w == nil {
			continue
		}
		if _, seen := indices[w.FQN]; !seen {
			strongConnect(g, w, index, stack, onStack, indices, lowlink, cycles)
			if lowlink[w.FQN] < lowlink[v.FQN] {
				lowlink[v.FQN] = lowlink[w.FQN]
			}
		} else if onStack[w.FQN] && indices[w.FQN] < lowlink[v.FQN] {
			lowlink[v.FQN] = indices[w.FQN]
		}
	}

	if lowlink[v.FQN] != indices[v.FQN] {
		return
	}
	var component []*Binding
	for {
		last := len(*stack) - 1
		w := (*stack)[last]
		*stack = (*stack)[:last]
		onStack[w.FQN] = false
		component = append(component, w)
		if w.FQN == v.FQN {
			break
		}
	}
	if len(component) > 1 || hasSelfDependency(component[0]) {
		sort.Slice(component, func(i, j int) bool { return component[i].FQN < component[j].FQN })
		*cycles = append(*cycles, Cycle{Bindings: component})
	}
}

func hasSelfDependency(b *Binding) bool {
	for _, dep := range b.Dependencies {
		if !dep.Deferred && dep.Target == b.FQN {
			return true
		}
	}
	return false
}

func cycleKey(c Cycle) string {
	if len(c.Bindings) == 0 || c.Bindings[0] == nil {
		return ""
	}
	return c.Bindings[0].FQN
}
