package module

import (
	"strings"
	"testing"
)

func buildTestGraph(modules map[string][]string) *ModuleGraph {
	g := NewModuleGraph("/tmp/test")
	for path, deps := range modules {
		m := &Module{Path: path, Dir: "/tmp/test/" + strings.TrimPrefix(path, ":")}
		for _, dep := range deps {
			m.Dependencies = append(m.Dependencies, Dependency{ModulePath: dep, Configuration: "implementation"})
		}
		g.Modules[path] = m
	}
	return g
}

func TestFindCycles_NoModules(t *testing.T) {
	g := NewModuleGraph("/tmp/test")
	cycles := g.FindCycles()
	if len(cycles) != 0 {
		t.Fatalf("expected nil, got %v", cycles)
	}
}

func TestFindCycles_DAG(t *testing.T) {
	g := buildTestGraph(map[string][]string{
		":app":  {":lib"},
		":lib":  {":core"},
		":core": {},
	})
	cycles := g.FindCycles()
	if len(cycles) != 0 {
		t.Fatalf("expected no cycles in DAG, got %v", cycles)
	}
}

func TestFindCycles_SimpleCycle(t *testing.T) {
	g := buildTestGraph(map[string][]string{
		":a": {":b"},
		":b": {":a"},
	})
	cycles := g.FindCycles()
	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %d: %v", len(cycles), cycles)
	}
	assertCycle(t, cycles[0], []string{":a", ":b"})
}

func TestFindCycles_Triangle(t *testing.T) {
	g := buildTestGraph(map[string][]string{
		":a": {":b"},
		":b": {":c"},
		":c": {":a"},
	})
	cycles := g.FindCycles()
	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %d: %v", len(cycles), cycles)
	}
	assertCycle(t, cycles[0], []string{":a", ":b", ":c"})
}

func TestFindCycles_Diamond(t *testing.T) {
	g := buildTestGraph(map[string][]string{
		":app":  {":a", ":b"},
		":a":    {":core"},
		":b":    {":core"},
		":core": {},
	})
	cycles := g.FindCycles()
	if len(cycles) != 0 {
		t.Fatalf("expected no cycles in diamond, got %v", cycles)
	}
}

func TestFindCycles_Mixed(t *testing.T) {
	g := buildTestGraph(map[string][]string{
		":a":   {":b"},
		":b":   {":a"},    // cycle :a <-> :b
		":c":   {":d"},    // no cycle
		":d":   {},
		":x":   {":y"},
		":y":   {":z"},
		":z":   {":x"},    // cycle :x -> :y -> :z -> :x
	})
	cycles := g.FindCycles()
	if len(cycles) != 2 {
		t.Fatalf("expected 2 cycles, got %d: %v", len(cycles), cycles)
	}
	assertCycle(t, cycles[0], []string{":a", ":b"})
	assertCycle(t, cycles[1], []string{":x", ":y", ":z"})
}

func assertCycle(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("cycle length mismatch: got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("cycle mismatch at index %d: got %v, want %v", i, got, want)
		}
	}
}
