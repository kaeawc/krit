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

// Batch callers (the daemon's analyze_buffers) begin one request and run every
// file in it, so running a file must keep the request's memo.
func TestDispatcherRunKeepsTheRequestsCatalogMemo(t *testing.T) {
	d := NewDispatcher(nil, nil)
	d.BeginRequest()
	memo := catalogDiscovery(d.fileFacts)
	for range 2 {
		file := parseInlineForInternalTest(t, "package a\n\nclass C\n")
		d.Run(file)
		d.RunColumnsWithStats(file)
	}
	if catalogDiscovery(d.fileFacts) != memo {
		t.Fatal("running a file replaced the request's discovery memo")
	}
}
