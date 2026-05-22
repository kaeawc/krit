package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestTryBundleLayer_HitEmitsOutcomeAttr is the regression guard for the
// diagnostic plumbing: a hit on any bundle layer must produce a perf
// entry tagged with outcome=hit. Before the diagnostic patch, hit/miss
// was implicit ("did the next layer's scope also show up?"), which the
// daemon path doesn't surface clearly because tracker.IsEnabled() and
// the per-layer scope creation interact in ways that obscured the
// answer. With explicit attribute tagging, perf JSON always answers
// the question on every call.
func TestTryBundleLayer_HitEmitsOutcomeAttr(t *testing.T) {
	tracker := perf.New(true)
	cached := &scanner.FindingColumns{}
	_, _, ok := tryBundleLayer(tracker, "dispatchBundleLoad", IndexResult{},
		func() (*scanner.FindingColumns, bool) { return cached, true })
	if !ok {
		t.Fatal("expected hit")
	}
	entries := tracker.GetTimings()
	found := false
	for _, e := range entries {
		if e.Name != "dispatchBundleLoad" {
			continue
		}
		found = true
		if got := e.Attributes["outcome"]; got != "hit" {
			t.Errorf("dispatchBundleLoad outcome = %q, want %q", got, "hit")
		}
	}
	if !found {
		t.Errorf("dispatchBundleLoad entry missing; got %v", entryNames(entries))
	}
}

// TestTryBundleLayer_MissEmitsOutcomeAttr is the symmetric guard: a
// missed bundle layer must still emit a perf entry so the next layer's
// presence isn't the only signal that this one was attempted.
func TestTryBundleLayer_MissEmitsOutcomeAttr(t *testing.T) {
	tracker := perf.New(true)
	_, _, ok := tryBundleLayer(tracker, "dispatchStableBundleLoad", IndexResult{},
		func() (*scanner.FindingColumns, bool) { return nil, false })
	if ok {
		t.Fatal("expected miss")
	}
	entries := tracker.GetTimings()
	found := false
	for _, e := range entries {
		if e.Name != "dispatchStableBundleLoad" {
			continue
		}
		found = true
		if got := e.Attributes["outcome"]; got != "miss" {
			t.Errorf("dispatchStableBundleLoad outcome = %q, want %q", got, "miss")
		}
	}
	if !found {
		t.Errorf("dispatchStableBundleLoad entry missing on miss; got %v", entryNames(entries))
	}
}

// TestTryBundleLayer_NilCachedTreatedAsMiss pins the edge case where
// the loader returns (nil, true) — historically this counts as a miss
// even though `ok` is true, because there's nothing to replay. The
// outcome attribute must reflect that.
func TestTryBundleLayer_NilCachedTreatedAsMiss(t *testing.T) {
	tracker := perf.New(true)
	_, _, ok := tryBundleLayer(tracker, "dispatchBundleLoad", IndexResult{},
		func() (*scanner.FindingColumns, bool) { return nil, true })
	if ok {
		t.Fatal("expected miss (nil cached treated as miss)")
	}
	entries := tracker.GetTimings()
	for _, e := range entries {
		if e.Name != "dispatchBundleLoad" {
			continue
		}
		if got := e.Attributes["outcome"]; got != "miss" {
			t.Errorf("nil-cached should record miss; got outcome=%q", got)
		}
	}
}

func entryNames(entries []perf.TimingEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name)
	}
	return out
}
