# Regressions (historical)

Regressions discovered by the roadmap loop's integration-test branch
(`scripts/roadmap-loop.sh`, now retired). Each file records a snapshot
comparison between a stored krit baseline and a newer build, run against
open-source Android repos (`~/github/coil`, `~/github/dagger`, etc.).

These files are kept as historical reference — they document:

- **What kinds of regressions surfaced** (perf, false-positive count,
  rule behavior changes) and their thresholds.
- **How the detection worked** (1-in-20 random integration check per
  loop iteration, per-rule deltas, absolute thresholds).
- **What was learned** — matrix jitter (~8 findings) means small deltas
  are noise; trust deltas > 30 and re-run small ones with multiple
  experiment runs.

## Regression types detected

| Flag | Meaning |
|------|---------|
| `perf_regression` | Scan duration exceeded 30% above baseline |
| `total_regression` | Total finding count jumped above threshold |
| `rule_regression` | Individual rule finding count changed significantly |

## File format

```
roadmap/regressions/<YYYYMMDD-HHMMSS>-<repo>.md
```

Each file contains: repo path, baseline/current krit SHAs, total
finding counts, per-rule deltas, regression flags, and an action block.

## Lessons learned

1. **Noise floor matters.** The experiment matrix jitters ~8
   `UnsafeCallOnNullableType` findings between runs. Small deltas
   are unreliable — trust deltas > 30, re-run small ones with
   `-experiment-runs=3`.
2. **Integration baselines should pin tree-sitter-only** to avoid
   oracle instability causing false rule-failure regressions.
3. **Perf regressions on small repos are noisy.** A 36% delta on a
   200ms scan is 70ms — within OS scheduling jitter.
4. **Automated regression → fix loops need human judgment.** The
   roadmap loop tried to feed regression files directly to codex
   for auto-fix, but the signal-to-noise ratio was too low for
   fully autonomous resolution.
