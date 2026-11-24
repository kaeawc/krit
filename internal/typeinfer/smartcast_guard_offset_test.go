package typeinfer

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

const postGuardTypeFactSource = `
fun guard(x: Any) {
    val beforeGuard = x
    if (x !is String) return
    val afterGuard = x
}

fun prefixGuard(x: Any) {
    val beforePrefixGuard = x
    if (!(x is String)) {
        return
    }
    val afterPrefixGuard = x
}

fun inBranch(x: Any) {
    val beforeElse = x
    if (x !is String) {
        println()
    } else {
        val inElse = x
    }
}
`

// resolveGuardProbe resolves the initializer of `val <probe> = expr` with resolver r.
func resolveGuardProbe(t *testing.T, r *defaultResolver, file *scanner.File, probe string) *ResolvedType {
	t.Helper()
	var init uint32
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if init != 0 || file.FlatType(idx) != "property_declaration" {
			return
		}
		varDecl := flatFindNamedChildOfType(file, idx, "variable_declaration")
		if varDecl == 0 || flatFirstIdentifierText(file, varDecl) != probe {
			return
		}
		init = file.FlatNamedChild(idx, file.FlatNamedChildCount(idx)-1)
	})
	if init == 0 {
		t.Fatalf("no `val %s = ...` in source", probe)
	}
	got := r.ResolveFlatNode(init, file)
	if got == nil {
		t.Fatalf("%s: resolver returned nil", probe)
	}
	return got
}

func assertGuardProbes(t *testing.T, r *defaultResolver, file *scanner.File, label string) {
	t.Helper()
	for probe, want := range map[string]string{
		"beforeGuard":       "Any",
		"afterGuard":        "String",
		"beforePrefixGuard": "Any",
		"afterPrefixGuard":  "String",
		"beforeElse":        "Any",
		"inElse":            "String",
	} {
		if got := resolveGuardProbe(t, r, file, probe); got.Name != want {
			t.Errorf("%s %s: want %s, got %+v", label, probe, want, got)
		}
	}
}

// An early-exit `!is` guard narrows only the code after it (#1520).
func TestPostGuardTypeFactAppliesOnlyAfterGuard(t *testing.T) {
	file := parseTestFile(t, postGuardTypeFactSource)
	r := NewResolver()
	r.IndexFilesParallel([]*scanner.File{file}, 1)
	assertGuardProbes(t, r, file, "fresh")
}

// The guard's start offset survives the type-index cache.
func TestPostGuardTypeFactStartSurvivesCache(t *testing.T) {
	file := parseTestFile(t, postGuardTypeFactSource)
	cacheDir := t.TempDir()
	for scan := 0; scan < 2; scan++ {
		r := NewResolver()
		hits, _ := r.IndexFilesParallelCachedWithTracker([]*scanner.File{file}, 1, cacheDir, nil)
		if scan == 1 && hits != 1 {
			t.Fatalf("second scan: want a cache hit, got %d", hits)
		}
		assertGuardProbes(t, r, file, map[int]string{0: "cold", 1: "cached"}[scan])
	}
}

func TestSetSmartCastTypeAfterKeepsEarliestGuard(t *testing.T) {
	scope := &ScopeTable{Entries: map[string]*ResolvedType{"x": {Name: "Any", Kind: TypeClass, Resolved: true}}}
	str := &ResolvedType{Name: "String", Kind: TypeClass, Resolved: true}
	scope.SetSmartCastTypeAfter("x", str, 50)
	scope.SetSmartCastTypeAfter("x", str, 20)
	scope.SetSmartCastTypeAfter("x", str, 80)
	if got := scope.LookupAt("x", 19); got.Name != "Any" {
		t.Errorf("before the earliest guard: want Any, got %+v", got)
	}
	if got := scope.LookupAt("x", 20); got.Name != "String" {
		t.Errorf("at the earliest guard: want String, got %+v", got)
	}
}
