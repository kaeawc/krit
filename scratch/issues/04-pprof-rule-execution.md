**Cluster:** performance-core · **Est.:** 1–2 days (profiling + targeted fixes)

## What

`ruleExecution` is ~1,650 ms on Signal-Android and **does not change**
between cold, warm, and single-file-edit runs. Every other phase
shrinks by 10×–50× on warm because its work is either cached or
served from page cache; ruleExecution is pure CPU work with no
cache between runs (by design — the incremental rule cache is
disabled with `--no-cache` in our benchmark and is only active for
SFE-style editor flows).

This means ruleExecution is now a hard floor on wall time. On warm
steady-state it's 63% of total wall time (1,650 / 2,611 ms). We don't
know its internal distribution: 481 rules run per file across 3,442
Kotlin files; some rules are known-expensive (regex-heavy, deep AST
walks) but we have no measurement to rank them.

This issue is a **profiling-first** issue: measure, then fix only
the outliers. The deliverable is a ranked hotspot table plus
targeted fixes for any rule consuming >5% of `ruleExecution` total.

## Measurement so far

Phase breakdown on Signal-Android (benchmark runbook, commit
`e1257dc`, `--no-type-oracle --no-cache`):

| Run | ruleExecution | Total | ruleExec % of total |
|---|---:|---:|---:|
| Cold | 1,638 ms | 18,177 ms | 9.0% |
| Warm-1 | 1,649 ms | 2,642 ms | 62.4% |
| Warm-2 | 1,650 ms | 2,611 ms | 63.2% |
| Warm-3 | 1,653 ms | 2,622 ms | 63.0% |
| SFE | 1,675 ms | 4,177 ms | 40.1% |

The ~25 ms std-dev between runs suggests the bottleneck is
deterministic CPU work (AST walks, regex matches, allocations), not
I/O or scheduling variance.

## Plan

1. **Instrument per-rule CPU & invocation counts.** The dispatcher at
   `internal/pipeline/dispatch.go:88-143` already threads
   `RunWithStats(file)` that returns per-rule stats; audit what's
   collected today and wire any missing fields (elapsed ns per rule,
   invocation count, bytes allocated via
   `runtime.ReadMemStats`-adjacent sampling). If per-rule timing is
   already produced, surface it via `--perf-rules` or similar.
2. **Capture a pprof profile on Signal-Android warm run.**
   `./krit --cpuprofile=/tmp/krit.prof --report json --perf
   --no-type-oracle --no-cache ~/github/Signal-Android`. Three warm
   runs, merge profiles. Generate `go tool pprof -top -cum` and
   flame graph.
3. **Produce a ranked hotspot table.** Top 20 rules by cumulative
   time and by allocations. Any rule >5% of total ruleExecution
   (i.e. >80 ms) goes on the fix list.
4. **Fix the outliers.** Each fix is its own small PR — do not
   bundle. Typical patterns to look for:
   - Rules that compile regex inside `Check()` instead of `init()`
   - Rules that walk the full AST when a `DispatchRule` type filter
     would work
   - Rules that `FlatNodeText()` excessively (each call allocates)
   - Rules that re-derive file-level properties (is-test-file,
     package name) per node instead of once
   - Rules that don't respect the `@Suppress` fast-path (should be
     filtered by the dispatcher, not the rule body)
5. **Add a perf guard.** After the fixes, lock in a `ruleExecution
   ≤ X ms` canary in CI for Signal-Android benchmark corpus, so
   future rule additions that regress the bound are caught.

## Expectations

- Ranked hotspot table committed to `scratch/rule-hotspots-*.md`
  (extending the existing `scratch/rule-hotspots-and-shared-
  indexes.md` if shape permits).
- For each rule consuming >5% of ruleExecution, either:
  - A fix PR that drops its share below 2%, or
  - An explicit accept-as-is note explaining why (e.g. "this rule
    requires a full AST walk by nature; pre-indexing would regress
    parse").
- Aggregate target: cold/warm ruleExecution drops 200–400 ms (12–24%)
  after top fixes. Warm total wall should then fall below 2,400 ms
  (vs. current 2,611 ms).

## Validate

- pprof CPU profile merged from 3 warm runs, attached to the issue
  as `scratch/rule-profile-YYYYMMDD.md` with the top-cum ranking
  reproduced inline.
- Bench-to-bench comparison after each outlier fix: before / after
  `ruleExecution` on Signal-Android, 3 warm runs each, report
  median.
- `go test ./internal/rules/ -count=1` — zero behavioural regressions.
- Finding-equivalence: `./krit --report json` unchanged.

## Risks

- **Rules that look slow are actually correct.** Some rules are
  intrinsically expensive (full-AST data-flow). Optimising to the
  point of cutting corners produces false negatives. Treat a rule as
  fixable only when the fix preserves its test fixtures.
- **Per-rule instrumentation overhead.** If rule timing adds its own
  cost to ruleExecution (e.g., `time.Now()` per invocation × 481
  rules × 3,442 files = 1.6M calls), the measurement itself skews.
  Use `runtime.nanotime` via per-batch sampling, not per-invocation.
- **Ranked-list staleness.** New rules land regularly; the ranking
  will shift. The CI canary is what keeps the bound honoured over
  time, not the ranked list itself.

## Opportunities

- If the profile shows `FlatNodeText()` as a hot allocator, lift it
  to return `[]byte` with a zero-copy path to the file's source
  buffer — the string allocation on every identifier read is
  plausibly several hundred ms across 3k files.
- If the profile shows regex domination, the existing
  `internal/rules/*.go` LineRule implementations could move to a
  unified regex-set that walks file lines once across N patterns
  (Hyperscan-style, but pure Go via `regexp.MatchReader` multiplex).
- Results feed the existing `scratch/rule-execution-leverage-plan.md`
  if that file's assumptions need updating.

## Dependencies

- **Blocked by:** none. Pure measurement + targeted fixes.
- **Related:** #338 (NeedsConcurrent capability gate) — affects how
  rule batches are scheduled, may change hot-path distribution.
- **Related:** existing `scratch/rule-execution-leverage-plan.md` and
  `scratch/rule-hotspots-and-shared-indexes.md` — prior thinking.

## References

- Dispatcher: `internal/pipeline/dispatch.go:88-143`
- Rule registry: `internal/rules/` (481 rules across dispatch, line,
  manifest, resource, gradle, icon, source subdirs)
- Measurement: benchmark runbook `scratch/benchmark-runbook.md`
- Prior notes: `scratch/rule-execution-leverage-plan.md`,
  `scratch/rule-hotspots-and-shared-indexes.md`
