package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/filefacts"
)

// Gradle rules share one catalog discovery memo per run, so a repository with
// nested builds is not rewalked for every Gradle file and catalog lookup.
func TestRunCatalogDiscoveryIsSharedWithinARun(t *testing.T) {
	prev := fileFactsCache()
	t.Cleanup(func() { setSharedFileFacts(prev) })

	setSharedFileFacts(nil)
	if runCatalogDiscovery() != nil {
		t.Fatal("outside a run, discovery should not memoize")
	}

	setSharedFileFacts(filefacts.NewCache())
	first := runCatalogDiscovery()
	if first == nil || runCatalogDiscovery() != first {
		t.Fatal("rules in one run should share one discovery memo")
	}

	setSharedFileFacts(filefacts.NewCache())
	if runCatalogDiscovery() == first {
		t.Fatal("a new run should get a fresh discovery memo")
	}
}
