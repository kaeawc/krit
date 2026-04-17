# CrossRuleHotspotVisibleForTesting

**Cluster:** [performance-infra](README.md) · **Status:** open ·
**Severity:** n/a (infra) · **Default:** n/a · **Est.:** 1–2 days

## What it does

Optimizes the `VisibleForTestingCallerInNonTest` cross-file rule,
which is currently the single-largest cross-rule cost on Signal-Android
warm runs.

## Current cost

On Signal-Android warm (from `--perf`):

```
crossRules: 729ms
  VisibleForTestingCallerInNonTest: 723ms    ← 99% of crossRules phase
```

**723ms for one rule = 10% of the entire 7s warm run.** Every other
cross-file rule combined is 6ms. This is a pure hotspot — the rule's
algorithm is expensive in its current form.

## Proposed design

### Investigate current implementation

Rule file (confirm path at start):

```bash
grep -rn 'VisibleForTestingCallerInNonTest' internal/rules/
```

Expected shape: walks every `call_expression` in every non-test file,
checks whether the callee has `@VisibleForTesting`, flags if so.

### Likely issues and mitigations

1. **No bloom filter short-circuit.** If the bloom filter already
   tracks `@VisibleForTesting`-annotated declaration names,
   `bloom.TestString(calleeName)` can skip 95%+ of call sites in O(1).
   Current rule may walk every call unconditionally.

2. **Per-call-expression string lookups** into `DeclarationIndex`
   instead of a precomputed set of "all VisibleForTesting FQNs".
   Construct that set once per run, check each call against it.

3. **Redundant ancestor walks.** Each call-expression match may walk
   its ancestors to determine test-file status. If the file is
   test-file, the rule should return early at the file boundary, not
   re-decide on every call.

4. **String allocation in hot loop.** `FlatNodeText` on large
   call-expression chains allocates; interning or byte-range comparison
   avoids it.

### Measurement plan

Before any change:

```bash
./krit --perf ~/github/Signal-Android 2>/dev/null | \
  jq '.perfTiming[] | .. | select(.name? == "VisibleForTestingCallerInNonTest").durationMs'
# expected: ~720ms warm
```

After each candidate change:

```bash
./krit -clear-cache ~/github/Signal-Android    # invalidate type cache
./krit ~/github/Signal-Android                  # warm up
./krit --perf ~/github/Signal-Android 2>/dev/null | \
  jq '.perfTiming[] | .. | select(.name? == "VisibleForTestingCallerInNonTest").durationMs'
```

Also run `go test ./internal/rules/ -run VisibleForTesting -count=1` to
verify fixture tests still pass.

### Finding-equivalence gate

**Critical.** The scratch notes (`krit-shared-index-narrow-vs-broad.md`)
document multiple past optimization attempts that made one rule faster
while repo-scale regressed or finding counts drifted. Every
optimization in this item must preserve finding counts on Signal and
kotlin/kotlin.

```bash
./krit --report json ~/github/Signal-Android > /tmp/before.json
# make change
./krit --report json ~/github/Signal-Android > /tmp/after.json
diff <(jq '.findings | sort_by(.file + ":" + (.line|tostring))' /tmp/before.json) \
     <(jq '.findings | sort_by(.file + ":" + (.line|tostring))' /tmp/after.json)
# expected: empty diff
```

## Files to touch

- `internal/rules/<wherever VisibleForTestingCallerInNonTest lives>` —
  rule logic
- `internal/scanner/index.go` — if bloom-filter hook is needed, extend
  the existing filter with annotation-aware entries
- `internal/rules/<...>_test.go` — keep fixtures green

## Measured target

Reduce rule cost from 723ms to <100ms warm on Signal → ~8× on this
hotspot → overall warm run from 7s to ~6.4s. Small on its own, but
this is the kind of single-rule hotspot that's cheap to fix and
represents the easy half of the optimization curve.

Once this one is shipped, use the same profiling approach
(`VisibleForTestingCallerInNonTest` is simply whatever the `--perf`
output names as its biggest `crossRules` child) to work down the
hotspot list. The rule-hotspots scratch doc identified several other
rules with high per-repo callback costs on the kotlin corpus.

## Risks

- **Correctness regression.** Finding-equivalence diff is the gate.
  Don't land any change that shifts findings on Signal.
- **False-negative via overaggressive skip.** Bloom filter false
  positives are safe (cost: redundant check). False negatives from a
  too-narrow skip predicate silently miss real findings. Test each
  skip path against the positive fixture.
- **Shape of the optimization doesn't generalize.** Other cross-file
  rules may have entirely different hotspot profiles. Do one rule at a
  time, measure each.

## Blocking

- None.

## Blocked by

- None. Independent of the two cache items.

## Links

- Parent cluster: [`performance-infra/README.md`](README.md)
- Related scratch analysis (if retained):
  [`scratch/rule-hotspots-and-shared-indexes.md`](../../../scratch/rule-hotspots-and-shared-indexes.md)
  — identifies `NoNameShadowing`, `UnnecessarySafeCall`, `MagicNumber`
  as the corresponding hotspots on kotlin/kotlin
- Related: [`scratch/krit-shared-index-narrow-vs-broad.md`](../../../scratch/krit-shared-index-narrow-vs-broad.md)
  — documents past failed attempts; narrow hotspot fixes have higher
  success rate than broad shared-summary rewrites
