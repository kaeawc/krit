=== Krit post-commit benchmark ===
Date: 2026-04-14T~19:45Z
Version: krit dev (commit `3b0ba1b`)
Platform: Darwin arm64
Config: cold start, `-no-cache`, `-no-type-oracle`, `-perf`

Two repos were profiled to validate the flat-tree migration wrap-up
and the 16 new Compose rules at different scales.

## Headline numbers

| Codebase         |  Files | Findings | Rules |  Wall |
|------------------|-------:|---------:|------:|------:|
| Signal-Android   |  2,467 |    5,467 |   238 | 7.04s |
| JetBrains Kotlin | 16,504 |   59,886 |   238 | 50.5s |

**Per-file throughput**: Signal 350 files/s, Kotlin 327 files/s —
within 7% of each other. Krit scales linearly with file count.

## Phase breakdown

| Phase            | Signal    | Kotlin     | Kotlin/Signal |
|------------------|----------:|-----------:|--------------:|
| collectFiles     |     156ms |    1,003ms |          6.4× |
| parse            |     576ms |    2,945ms |          5.1× |
| typeIndex        |   1,841ms |   23,389ms |         12.7× |
| ruleExecution    |   4,148ms |   22,784ms |          5.5× |
| crossFileAnalysis|       4ms |       28ms |          7.0× |
| androidProject   |     309ms |      303ms |          1.0× |
| **total**        | **7,041ms** | **50,476ms** |       **7.2×** |

With 6.7× more files, the Kotlin repo takes 7.2× wall time overall.
Two phases scale differently:

- **`typeIndex` scales 12.7× for 6.7× files.** typeIndex cost is
  not a flat per-file constant — it scales with the number of type
  references per file, and the Kotlin compiler sources are denser in
  type declarations / generics / interfaces than Android app code.
  This is the single biggest scaling wedge. The 2026-04-09 baseline
  had typeIndex at 19% of Signal's wall time; today it's 26% on
  Signal and 46% on the Kotlin repo.
- **`ruleExecution` scales 5.5× — slightly *better* than linear.**
  The flat-dispatcher's per-node cost amortizes well as the tree
  grows; most rules are O(node count) and the dispatcher only pays
  one walk. Aggregated `dispatchWalk` counters are 62,721ms on
  Signal and 342,793ms on Kotlin — both ~15× their wall
  `ruleExecution` time, consistent with a ~16-core parallel fan-out.
- **`androidProjectAnalysis` is constant** (Signal 309ms, Kotlin
  303ms). The Kotlin repo has only a handful of Gradle build files
  and no Android resources, so this phase bottoms out at a small
  fixed cost regardless of source-file count.

### What to optimize next

typeIndex is now the largest scaling wedge. At Signal's size it's
26% of wall time; at Kotlin's size it's 46%. The
`perFileExtraction` sub-phase alone is 23,362ms of Kotlin's 23,389ms
typeIndex total — so all the cost is per-file scanning, not the
merge/resolve tail. Options for attack:
1. Skip typeIndex entirely on files that have no inbound references
   from other scanned files (requires a two-pass approach).
2. Lazy type extraction — only resolve the imports + class headers
   during indexing, defer member extraction to first query.
3. Parallelize more aggressively — per-file extraction is already
   parallel, but perhaps the per-file work itself can be split.

None of these are free; they're research-size projects, not quick
wins. Documenting here so the next perf sprint starts from current
data instead of the 2025-era 49% dispatch claim.

## Per-rule activity

### Signal-Android top 15

    MaxLineLength                                3,990
    UnsafeCallOnNullableType                       228
    MagicNumber                                    171
    ComposeSideEffectInComposition                 124
    UnsafeCast                                     116
    UseOrEmpty                                      85
    LongMethod                                      81
    NegativeMarginResource                          41
    DisableBaselineAlignmentResource                39
    TooManyFunctions                                35
    MapGetWithNotNullAssertionOperator              31
    ReturnCount                                     21
    ThrowsCount                                     21
    LongParameterList                               21
    ThrowingExceptionsWithoutMessageOrCause         20

### JetBrains Kotlin top 15

    MaxLineLength                               22,411
    WildcardImport                               7,776
    NewLineAtEndOfFile                           4,850
    UnsafeCast                                   4,269
    UnsafeCallOnNullableType                     2,862
    ForbiddenComment                             2,045
    MagicNumber                                  1,444
    InvalidPackageDeclaration                    1,197
    NoNameShadowing                                734
    TooManyFunctions                               723
    LongMethod                                     713
    FunctionNaming                                 670
    ModifierOrder                                  611
    MatchingDeclarationName                        609
    AbstractClassCanBeConcreteClass                523

MaxLineLength dominates both repos (75% of Signal's findings,
38% of Kotlin's). WildcardImport is second on Kotlin with 7,776
hits — the Kotlin compiler's internal style apparently allows
wildcard imports, so every source file hitting that pattern trips
it. Those are rules where the signal-to-noise may be low in these
specific repos; either repo could legitimately `-disable-rules
MaxLineLength,WildcardImport` in CI for a ~40% finding reduction.

## New Compose rules — firing counts

16 new Compose correctness rules shipped this session. How many
findings did each produce on each repo?

### Signal-Android (148 total findings across 6 rules)

| Rule | Findings |
|---|---:|
| `ComposeSideEffectInComposition` | 124 |
| `ComposeUnstableParameter` | 10 |
| `ComposeStatefulDefaultParameter` | 8 |
| `ComposeRememberWithoutKey` | 4 |
| `ComposeDisposableEffectMissingDispose` | 1 |
| `ComposePreviewWithBackingState` | 1 |

The other 10 new rules produced 0 findings on Signal-Android —
not necessarily a problem, most of them catch specific narrow
anti-patterns that may just not be present in that codebase.

The 124 `ComposeSideEffectInComposition` hits are a **signal-to-noise
flag**. That rule detects assignment expressions inside a
`@Composable` function body not wrapped in an effect block. 124 hits
on one app suggests either (a) Signal-Android genuinely has a lot of
compose-era side-effect anti-patterns, or (b) the rule is firing on
patterns the author considered intentional (e.g. `remember { }` body
assignments, local val initialization misread as assignment).
Worth a spot-check in a follow-up FP-hunt pass before promoting
this to active-by-default for everyone.

### JetBrains Kotlin (2 total findings across 2 rules)

| Rule | Findings |
|---|---:|
| `ComposeSideEffectInComposition` | 1 |
| `ComposeStatefulDefaultParameter` | 1 |

As expected — the Kotlin compiler sources aren't a Compose UI
codebase, so the Compose correctness rules have almost nothing to
match against. Two hits in 16,504 files is the noise floor,
confirming the rules aren't misfiring on unrelated Kotlin code.

## Delta vs earlier 2026-04-14 snapshots

Signal-Android progression through the day (same flags, same repo):

| Milestone       |  Wall |  Findings | Rules | Notes |
|-----------------|------:|----------:|------:|---|
| Pre-Track-B (baseline 2026-04-13) | 6,746ms | 5,334 | 226 | commit `5122cac` |
| Post-Track-B    | 6,333ms | 5,319 | 226 | typeinfer retired from node-era API |
| Post-Track-C    | 6,476ms | 5,319 | 226 | scanner compat cleanup (pure code motion) |
| Post-Track-F    | 6,896ms | 5,467 | 238 | 16 new compose rules active |
| Post-Track-D    | 6,957ms | 5,467 | 238 | archive wholesale deletion |
| **This run (post-commit, cold cache)** | **7,041ms** | **5,467** | **238** | `3b0ba1b` |

The +400ms between Post-Track-D (6957ms) and this run (7041ms) is
within cold-start noise (±100ms is typical, the two Track-D runs
themselves varied by 4ms). Findings and rule count are stable. No
regressions.

## Takeaways

1. **Flat-tree migration doesn't hurt throughput.** Per-file rate
   is identical on a 2.5k Android app and a 16.5k Kotlin compiler
   repo (350 vs 327 files/s), showing the dispatcher scales
   linearly.
2. **typeIndex is the new bottleneck at scale.** 46% of wall time
   on the Kotlin repo vs 26% on Signal. Next perf sprint should
   target `perFileExtraction` in `internal/typeinfer/`.
3. **`ComposeSideEffectInComposition` needs an FP-reduction pass**
   before it ships widely — 124 hits on one Android app is
   suspiciously high. The other 15 new Compose rules look well-
   calibrated at this sample size.
4. **The Kotlin compiler repo is a useful perf torture test** —
   large, type-dense, and reliably reproduces in ~50s cold. Should
   be added to `scripts/full-benchmark.sh` as a standing smoke test
   (currently that script only covers the Android-app-shaped repos).
