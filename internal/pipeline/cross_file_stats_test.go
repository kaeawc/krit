package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// TestCrossFileStats_ReportsAllSlots confirms CrossFileStats exposes
// every resident slot the daemon's analyze-project response surfaces.
// Drives the per-slot bools by populating each cache method and
// reading the snapshot back.
func TestCrossFileStats_ReportsAllSlots(t *testing.T) {
	ws := NewWorkspaceState(t.TempDir())

	// Cold: every slot should be empty.
	stats := ws.CrossFileStats()
	if stats.HasLibraryFacts || stats.HasCodeIndex || stats.HasDependents ||
		stats.HasResolver || stats.HasOracleFilter {
		t.Fatalf("cold stats not all-false: %+v", stats)
	}

	// Populate each slot via its cache method.
	ws.LibraryFacts("lf-fp", func() *librarymodel.Facts {
		return &librarymodel.Facts{}
	})
	ws.CodeIndex("ci-fp", func() *scanner.CodeIndex {
		return &scanner.CodeIndex{}
	})
	ws.Dependents("dep-fp", func() *scanner.DependentsIndex {
		return &scanner.DependentsIndex{}
	})
	ws.Resolver("res-fp", func() typeinfer.TypeResolver {
		return typeinfer.NewResolver()
	})
	ws.OracleFilter("of-fp", func() *oracle.CallTargetFilterSummary {
		return &oracle.CallTargetFilterSummary{Enabled: true}
	})

	stats = ws.CrossFileStats()
	if !stats.HasLibraryFacts {
		t.Errorf("HasLibraryFacts = false; expected true after LibraryFacts call")
	}
	if !stats.HasCodeIndex {
		t.Errorf("HasCodeIndex = false; expected true after CodeIndex call")
	}
	if !stats.HasDependents {
		t.Errorf("HasDependents = false; expected true after Dependents call")
	}
	if !stats.HasResolver {
		t.Errorf("HasResolver = false; expected true after Resolver call")
	}
	if !stats.HasOracleFilter {
		t.Errorf("HasOracleFilter = false; expected true after OracleFilter call")
	}

	// Invalidate the new slots and re-check.
	ws.InvalidateResolver()
	ws.InvalidateOracleFilter()
	stats = ws.CrossFileStats()
	if stats.HasResolver {
		t.Errorf("HasResolver still true after InvalidateResolver")
	}
	if stats.HasOracleFilter {
		t.Errorf("HasOracleFilter still true after InvalidateOracleFilter")
	}
	// Pre-existing slots should be untouched.
	if !stats.HasLibraryFacts || !stats.HasCodeIndex {
		t.Errorf("pre-existing slots dropped on Resolver/OracleFilter invalidate: %+v", stats)
	}
}
