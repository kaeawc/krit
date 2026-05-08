package di

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestFindCycles(t *testing.T) {
	file := parseDIFile(t, `
package test
class A @Inject constructor(val b: B)
class B @Inject constructor(val c: C)
class C @Inject constructor(val a: A)
`)
	cycles := BuildGraph([]*scanner.File{file}, nil).FindCycles()
	if len(cycles) != 1 {
		t.Fatalf("cycles = %d, want 1", len(cycles))
	}
	if len(cycles[0].Bindings) != 3 {
		t.Fatalf("cycle bindings = %d, want 3", len(cycles[0].Bindings))
	}
}

func TestFindCyclesIgnoresDeferredEdges(t *testing.T) {
	file := parseDIFile(t, `
package test
class A @Inject constructor(val b: B)
class B @Inject constructor(val a: Lazy<A>)
`)
	cycles := BuildGraph([]*scanner.File{file}, nil).FindCycles()
	if len(cycles) != 0 {
		t.Fatalf("expected no cycles, got %+v", cycles)
	}
}
