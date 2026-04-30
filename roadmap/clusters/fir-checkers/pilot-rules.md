# B.4 — Pilot rule migration

**Cluster:** [fir-checkers](README.md) · **Status:** planned · **Track:** B · **Severity:** n/a (tool mode)

## Catches

Track B only has value if real rules ship on it. This concept
picks a small set of pilot rules — ones where the tree-sitter +
oracle path is demonstrably insufficient — ports them to FIR
checkers, and proves the end-to-end pipeline works by running
them alongside their Go counterparts with a parity oracle.

## Rule selection criteria

A rule is a good FIR pilot candidate if **all** of these hold:

1. **Type-shape dependent.** The rule's correctness depends on
   resolving a callee, receiver, or parameter type through the
   classpath — not something tree-sitter can see. Examples:
   "collect on a `Flow`", "annotation is @Composable", "receiver
   is a `LiveData<T>`".
2. **Currently Go + heuristic.** The rule exists today in the Go
   rule set but relies on substring matches, simple-name lookup,
   or the oracle fallback. It has a known false-positive or
   false-negative rate documented somewhere (FP hunt sessions,
   depth-over-breadth doc, etc.).
3. **Has positive and negative fixtures.** The rule is already
   in `tests/fixtures/` with both a positive and a negative case,
   so the parity oracle has something to compare against.
4. **No complex fix logic.** Fix emission from FIR checkers is
   underspecified in Track B. Pilot rules should be detect-only;
   fixes can come later.

### Candidate pilots

These are rules flagged as type-inference-dependent in
[`roadmap/17-depth-over-breadth.md`](../../17-depth-over-breadth.md)
§2 Tier 2 ("Good, known edges"). Pick 3–5 for Track B.4.

| Candidate | Today | Why FIR helps | Fixtures? |
|---|---|---|---|
| `FlowCollectInOnCreate` | tree-sitter walks `function_call_expression` looking for `.collect` literal | Resolves callee to `kotlinx.coroutines.flow.Flow.collect` via symbol graph — no false hits on unrelated `.collect` methods | yes |
| `ComposeRememberWithoutKey` | oracle-based; detects `remember { ... }` missing a key argument | FIR's `FirFunctionCall` knows the callee is `androidx.compose.runtime.remember` and can inspect resolved argument list precisely | yes |
| `InjectDispatcher` | heuristic; needs DI annotation awareness | FIR sees resolved annotations on constructor params through full symbol resolver — no string matching | yes |
| `IgnoredReturnValue` | currently heuristic | FIR knows the resolved return type of any call and whether it's `Unit`/`Nothing`/`@IgnorableReturnValue` | yes |
| `SuspendFunctionWithoutContext` | not currently implemented reliably | FIR's `CheckerContext` tracks containing suspend-function scope directly | partial |
| `HasPlatformType` | works but needs type inference | Analysis API provides `KaType` including platform-type marker | yes |

**Recommended starting set:** `FlowCollectInOnCreate`,
`ComposeRememberWithoutKey`, `InjectDispatcher`. Three rules,
three distinct checker base classes (`FirFunctionCallChecker`,
`FirFunctionCallChecker`, `FirCallableDeclarationChecker`), three
existing Go implementations to parity-test against. Enough
variety to exercise the whole Track B machinery; small enough to
ship in a week.

## Parity oracle

**Do not delete the Go implementations when the FIR version
ships.** For each pilot rule, both implementations coexist during
Track B and feed a cross-implementation parity test:

```go
// tests/parity/fir_parity_test.go
func TestFlowCollectInOnCreate_FirParity(t *testing.T) {
  for _, fixture := range fixturesFor("FlowCollectInOnCreate") {
    goFindings := runGoRule("FlowCollectInOnCreate", fixture)
    firFindings := runFirRule("FlowCollectInOnCreate", fixture)
    assertSameFindings(t, goFindings, firFindings)
  }
}
```

The oracle runs in CI on every change. If they diverge, that's a
real signal — either the FIR version caught something the Go
version missed (good, FIR wins, document), or the FIR version
has a false positive (fix before promoting).

**Why this discipline matters:** migration cleanups that delete
the "old side" before the new side has behavioral coverage are
how silent divergences slip through. When the Go rule is retired
later (Track B end-state), its fixture set must have been lifted
into the compiler-test framework under `compiler-tests/src/test/data/diagnostic/`
first, with at least the same behaviors covered.

## Promotion criteria

A pilot rule promotes from `--fir` opt-in to the default catalog
when **all** of these hold:

- Parity oracle passes on 100% of existing fixtures for ≥1 week
- Real-repo run on the kotlin multi-repo audit set
  ([item 46](../../46-kotlin-multi-repo-audit.md)) shows equal or
  better precision than the Go version
- Warm-path cache hit rate on `internal/firchecks/` cache is
  ≥90% on repeat runs
- The rule's diagnostic appears in the compiler-test data files
  with at least one positive and one negative case

Once a rule is promoted, the Go implementation stays in place until the FIR
version has run in production for at least one release cycle. After that, remove
the replaced Go implementation in a focused cleanup PR with fixture parity
evidence.

## Definition of done

- 3 pilot rules live under `tools/krit-fir/src/main/kotlin/dev/krit/fir/rules/`
- Each has a diagnostic in `KritDiagnostics.kt`
- Each has at least one positive + one negative compiler-test
  data file
- `tests/parity/fir_parity_test.go` passes on all three for the
  full existing Go fixture set
- Running `krit check --fir` on Signal-Android shows the pilot
  rules' findings alongside the tree-sitter pass's findings,
  deduplicated correctly

## Non-goals (for this concept)

- Mass migration of all type-inference rules — pilots only
- Publishing as a kotlinc plugin — see
  [plugin-packaging.md](plugin-packaging.md)
- Retiring the Go implementations — deferred until after
  Track C.1
