package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/filefacts"
)

// Gradle rules share one catalog discovery memo per request, so a repository with
// nested builds is not rewalked for every Gradle file and catalog lookup.
func TestCatalogDiscoveryIsSharedWithinARequest(t *testing.T) {
	if catalogDiscovery(nil) != nil {
		t.Fatal("without a cache, discovery should not memoize")
	}

	facts := filefacts.NewCache()
	first := catalogDiscovery(facts)
	if first == nil || catalogDiscovery(facts) != first {
		t.Fatal("rules in one request should share one discovery memo")
	}

	if catalogDiscovery(filefacts.NewCache()) == first {
		t.Fatal("a new run should get a fresh discovery memo")
	}
}

// A long-lived dispatcher keeps its cache across requests; BeginRequest gives
// the next request a fresh discovery memo (#1784), and only that memo.
func TestBeginRequestDropsTheCatalogDiscoveryMemo(t *testing.T) {
	d := NewDispatcher(nil, nil)
	first := catalogDiscovery(d.fileFacts)
	kept := filefacts.StringFact(d.fileFacts, "/repo/app", slotSourceModuleGradle, func() *int { return new(int) })

	d.BeginRequest()
	second := catalogDiscovery(d.fileFacts)
	if second == first {
		t.Fatal("BeginRequest kept the previous request's discovery memo")
	}
	if catalogDiscovery(d.fileFacts) != second {
		t.Fatal("rules in the new request should share its discovery memo")
	}
	if filefacts.StringFact(d.fileFacts, "/repo/app", slotSourceModuleGradle, func() *int { return new(int) }) != kept {
		t.Fatal("BeginRequest dropped an unrelated fact")
	}
}
