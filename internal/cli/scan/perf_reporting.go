package scan

import (
	"fmt"
	"io"

	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// hitRatePct returns 100 * hits / (hits+misses), clamped to 0 when there
// are no observations. Pulled out so the oracle/memo reporting paths
// share a single formula that's straightforward to unit test.
//
// Takes uint64 since both the oracle (int64 atomic counters) and the
// hashutil memo (uint64 atomic counters) feed it; int64 → uint64 is safe
// because these are non-negative monotonic counters.
func hitRatePct(hits, misses uint64) int {
	total := hits + misses
	if total == 0 {
		return 0
	}
	return int(hits * 100 / total)
}

// reportOracleLookupStats prints expr/class/func hit-miss counters from a
// CompositeResolver wrapping an oracle.Oracle. No-op when the resolver is
// not oracle-backed or when no lookups were observed. Stats are diagnostic
// only: they go to the supplied writer (typically os.Stderr).
func reportOracleLookupStats(w io.Writer, resolver typeinfer.TypeResolver) {
	cr, ok := resolver.(*oracle.CompositeResolver)
	if !ok {
		return
	}
	o, ok := cr.Oracle().(interface {
		Stats() oracle.Stats
	})
	if !ok {
		return
	}
	s := o.Stats()
	exprTotal := s.ExprHits + s.ExprMisses
	classTotal := s.ClassHits + s.ClassMisses
	funcTotal := s.FuncHits + s.FuncMisses
	if exprTotal+classTotal+funcTotal == 0 {
		return
	}
	fmt.Fprintf(w,
		"perf: oracle lookups — expr %d hit / %d miss (%d%%), class %d hit / %d miss (%d%%), func %d hit / %d miss (%d%%)\n",
		s.ExprHits, s.ExprMisses, hitRatePct(uint64(s.ExprHits), uint64(s.ExprMisses)),
		s.ClassHits, s.ClassMisses, hitRatePct(uint64(s.ClassHits), uint64(s.ClassMisses)),
		s.FuncHits, s.FuncMisses, hitRatePct(uint64(s.FuncHits), uint64(s.FuncMisses)))
}

// reportContentHashMemoStats prints the per-run hashutil.Memo hit/miss
// counters so SharedContentHashMemo (#305) redundancy elimination is
// observable. No-op when no observations were recorded.
func reportContentHashMemoStats(w io.Writer) {
	hits, misses := hashutil.Default().Stats()
	if hits+misses == 0 {
		return
	}
	fmt.Fprintf(w,
		"perf: content-hash memo — %d hit / %d miss (%d%%), %d unique files\n",
		hits, misses, hitRatePct(hits, misses), hashutil.Default().Len())
}
