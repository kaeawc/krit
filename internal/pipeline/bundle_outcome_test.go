package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/perf"
)

func TestRecordBundleOutcome(t *testing.T) {
	recordBundleOutcome(nil, true) // nil tracker is a no-op
	for _, hit := range []bool{true, false} {
		tr := perf.New(true)
		scope := tr.Serial("bundleHit")
		recordBundleOutcome(scope, hit)
		scope.End()
		want := "miss"
		if hit {
			want = "hit"
		}
		timings := tr.GetTimings()
		if len(timings) != 1 || timings[0].Name != "bundleHit" || len(timings[0].Children) != 1 ||
			timings[0].Children[0].Attributes["outcome"] != want {
			t.Fatalf("hit=%v timings = %+v", hit, timings)
		}
	}
}
