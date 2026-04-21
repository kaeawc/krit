# Rule Hotspots And Shared Indexes

## Benchmark Anchors

### Kotlin Repo

Source:

- [`/tmp/krit_kotlin_rulefocus_returncount.json`](/tmp/krit_kotlin_rulefocus_returncount.json)

Major buckets:

- `ruleExecution`: `23765ms`
- `typeIndex`: `12475ms`

Top dispatch rules:

1. `NoNameShadowing` `24835ms`
2. `MagicNumber` `17612ms`
3. `UnnecessarySafeCall` `13073ms`
4. `ThrowingExceptionsWithoutMessageOrCause` `7454ms`
5. `ReturnCount` `6694ms`
6. `LongParameterList` `4474ms`
7. `UnusedParameter` `4443ms`
8. `CyclomaticComplexMethod` `3758ms`
9. `LongMethod` `3575ms`
10. `NestedBlockDepth` `3563ms`

### Signal Repo

Source:

- [`/tmp/signal_perf_2026-04-10_lineindex_coarse_b.json`](/tmp/signal_perf_2026-04-10_lineindex_coarse_b.json)

Major buckets:

- `ruleExecution`: `1052ms`
- `androidProjectAnalysis`: `310ms`
- `typeIndex`: `208ms`

Top dispatch rules:

1. `NoNameShadowing` `2424ms`
2. `ReturnCount` `893ms`
3. `MagicNumber` `777ms`
4. `UnnecessarySafeCall` `764ms`
5. `ThrowsCount` `654ms`
6. `LongParameterList` `477ms`
7. `UseValueOf` `444ms`
8. `CyclomaticComplexMethod` `407ms`
9. `NestedBlockDepth` `390ms`
10. `VarCouldBeVal` `357ms`

## Rule Classification By Analysis Type

### A. Lexical Scope / Name Resolution

Rules:

- `NoNameShadowing`
- `UnusedParameter`
- parts of naming family
- parts of `VarCouldBeVal`

Core analysis:

- lexical scope stack
- declaration visibility
- shadow relationships
- local mutation / assignment tracking

Algorithmic shape:

- tree traversal with scope push/pop
- set / multiset membership lookups
- repeated declaration collection

Best shared index candidate:

- `ScopeSummaryIndex`

Contains:

- scope tree
- declared names per scope
- declaration kinds
- shadow edges
- assignment/reassignment flags by declaration

Expected leverage:

- very high, but implementation risk is also high
- `NoNameShadowing` alone may justify a dedicated pass

### B. Control-Flow / Jump Counting

Rules:

- `ReturnCount`
- `ThrowsCount`
- `LongMethod`
- `CyclomaticComplexMethod`
- `NestedBlockDepth`
- parts of `LongParameterList` are nearby structurally but not the same analysis

Core analysis:

- jump expressions
- branch nodes
- nesting depth
- guard-clause detection
- local function / lambda boundary handling

Algorithmic shape:

- function-subtree traversal
- counting with boundary pruning

Best shared index candidate:

- `FunctionFlowSummaryIndex`

Contains:

- returns count
- throws count
- guard-clause jump set
- branch counts by kind
- max block depth
- nested lambda/local function boundaries

Expected leverage:

- very high
- many hot rules would consume the same summary
- lower risk than a full scope-analysis redesign

### C. Literal Classification

Rules:

- `MagicNumber`
- possibly related numeric-literal style rules

Core analysis:

- numeric literal normalization
- contextual exemptions

Algorithmic shape:

- visit literal nodes
- inspect ancestors / local syntactic context

Best shared index candidate:

- `LiteralContextIndex`

Contains:

- normalized literal value
- parent declaration / call context
- flags:
  - annotation
  - enum entry
  - const declaration
  - named argument
  - range endpoint
  - compose unit
  - color literal

Expected leverage:

- medium
- currently mostly a single-rule win, but the rule is hot enough to justify it

### D. Local Nullability / Guard Reasoning

Rules:

- `UnnecessarySafeCall`
- `UnnecessaryNotNullOperator`
- nearby null-safety rules in [`internal/rules/potentialbugs_nullsafety.go`](/Users/jason/kaeawc/krit/internal/rules/potentialbugs_nullsafety.go)

Core analysis:

- explicit nullable vs non-null locals
- smart-cast style guards
- direct null-check dominance
- local immutable declaration facts

Algorithmic shape:

- syntax inspection plus light semantic lookup
- repeated local scans inside functions/files

Best shared index candidate:

- `LocalNullabilityIndex`

Contains:

- immutable local declarations
- initializer nullability hints
- guard predicates and guarded regions
- type-predicate guard summaries

Expected leverage:

- high if scoped carefully
- prior ad hoc file-level caches regressed, so this must be designed around the actual query patterns

### E. Declaration / Signature / Modifier Summary

Rules:

- `ThrowingExceptionsWithoutMessageOrCause`
- `LongParameterList`
- `UnusedParameter`
- many style/naming rules

Core analysis:

- direct members
- parameters
- modifiers
- thrown expression shapes

Algorithmic shape:

- repeated declaration header scans
- direct-child extraction

Best shared index candidate:

- `DeclarationSummaryIndex`

Contains:

- declarations by file
- name
- modifiers
- direct parameters
- direct member lists
- signature and return-type header nodes

Expected leverage:

- high and broadly reusable
- lower risk than scope analysis

## Which Shared Indexes Collapse The Most Repeated Work

### 1. FunctionFlowSummaryIndex

Why first:

- covers multiple current hotspots at once
- repeated function-subtree walks are common and expensive
- easier to validate semantically than full scope analysis

Rules helped immediately:

- `ReturnCount`
- `ThrowsCount`
- `CyclomaticComplexMethod`
- `NestedBlockDepth`
- `LongMethod`

### 2. DeclarationSummaryIndex

Why second:

- many rules repeatedly inspect the same declaration headers and direct members
- this can cut a large amount of child scanning and modifier extraction

Rules helped immediately:

- `ThrowingExceptionsWithoutMessageOrCause`
- `LongParameterList`
- `UnusedParameter`
- many naming/style rules

### 3. LocalNullabilityIndex

Why third:

- `UnnecessarySafeCall` remains expensive
- null-safety rules repeatedly rebuild overlapping local facts

Risk:

- easy to overbuild
- needs tight scope and measured query APIs

Latest experiment:

- baseline [`/tmp/krit_kotlin_declsummary_pass1_rerun.json`](/tmp/krit_kotlin_declsummary_pass1_rerun.json)
  - total `37997ms`
  - `ruleExecution` `22878ms`
  - `UnnecessarySafeCall` `12238ms`
- attempt [`/tmp/krit_kotlin_localnull_pass1.json`](/tmp/krit_kotlin_localnull_pass1.json)
  - total `39860ms`
  - `ruleExecution` `22126ms`
  - `UnnecessarySafeCall` `2541ms`

Outcome:

- rejected
- the rule callback got much cheaper
- whole-run wall clock regressed by about `1863ms`
- finding totals also changed, so the design is not safe to keep
- if retried, it should be much narrower and query-specific

### 4. ScopeSummaryIndex

Why fourth:

- highest upside because of `NoNameShadowing`
- highest implementation cost and semantics risk

This probably deserves its own pass rather than being bundled with lighter shared summaries.

### 5. LiteralContextIndex

Why fifth:

- `MagicNumber` is hot, but the analysis family is narrower
- still worth doing if the implementation remains small and cheap

## Recommended Execution Order

1. `DeclarationSummaryIndex`
2. `LocalNullabilityIndex` retry, but only in a narrower query-specific form
3. dedicated `NoNameShadowing` scope-analysis pass
4. `FunctionFlowSummaryIndex` retry only if split into smaller summaries
5. `LiteralContextIndex` only if there is another real consumer

## Concrete Migration Candidates

### First Batch

- `ReturnCount`
- `ThrowsCount`
- `CyclomaticComplexMethod`
- `NestedBlockDepth`

Target:

- consume a shared function summary instead of each walking the subtree

### Second Batch

- `LongParameterList`
- `UnusedParameter`
- `ThrowingExceptionsWithoutMessageOrCause`

Target:

- consume declaration summaries and parameter/member lists

### Third Batch

- `UnnecessarySafeCall`
- `UnnecessaryNotNullOperator`

Target:

- consume local nullability and guard summaries

### Fourth Batch

- `MagicNumber`

Target:

- consume cached literal-context classification

### Fifth Batch

- `NoNameShadowing`

Target:

- dedicated scope-summary / shadow-edge analysis

## Progress Tracking

### Planned

- [ ] define shared summary API shapes
- [ ] add perf timing for new summary phases
- [ ] land `FunctionFlowSummaryIndex`
- [ ] migrate first control-flow batch
- [ ] land `DeclarationSummaryIndex`
- [ ] migrate declaration/signature batch
- [x] attempt `LocalNullabilityIndex`
- [ ] retry `LocalNullabilityIndex` with a narrower query-specific design
- [ ] migrate null-safety batch
- [ ] evaluate `LiteralContextIndex`
- [ ] design dedicated scope-analysis pass for `NoNameShadowing`

### Rejected

- [x] broad shared `FunctionFlowSummaryIndex` pass on `ReturnCount` + complexity family
  repo-scale `kotlin` regression; reverted
- [x] `LiteralContextIndex`-style ancestor cache for `MagicNumber`
  `MagicNumber` improved locally, but repo-scale `kotlin` regressed; reverted
- [x] query-specific single-pass ancestor scan for `MagicNumber`
  repo-scale `kotlin` improved, but `MagicNumber` finding count changed from `31077` to `31120`; reverted
- [x] broad file/function `LocalNullabilityIndex` cache for `UnnecessarySafeCall`
  `UnnecessarySafeCall` improved locally, but repo-scale `kotlin` regressed and finding totals changed; reverted
- [x] narrow structural `UnnecessarySafeCall` rewrite for receiver/parameter nullability checks
  preserved `UnnecessarySafeCall` finding count, but repo-scale `kotlin` got slightly slower; reverted
- [x] aggressive descendant whitelist for `NoNameShadowing`
  wall clock improved and `NoNameShadowing` callback time dropped, but rule findings on `kotlin` collapsed from `3135` to `368`; reverted
- [x] ancestor-type-set coalescing for `MagicNumber`
  replaces 11 `FlatHasAncestorOfType` walks per literal with one walk
  populating a flag struct; semantics preserved and finding equivalence
  verified on both `kotlin/kotlin` (1444 findings) and `Signal-Android`
  (171 findings); kotlin `MagicNumber` warm callback was 368ms before
  and 365ms after (wash, within run-to-run variance); not worth the
  code churn. Confirms PR #320's conclusion that `MagicNumber` is
  already out of the top-10 hotspots and per-callback narrowing is
  not the right lever — the heuristic-driven skip count (31077 →
  1444 findings over project history) is what moved the needle.

### Accepted

- [x] narrow `DeclarationSummaryIndex` for function/class headers
  currently used by `LongParameterList` and `UnusedParameter`
- [x] narrow `UnnecessarySafeCall` per-file null-safety summary (PR #320)
  per-receiver regex scans collapsed into one lazy per-file summary;
  kotlin/kotlin 11201ms → 2384ms (4.7×) + 1401ms bonus on
  `UnnecessaryNotNullOperator`; closes #301 for this rule.

### #301 Status

Issue #301 asked to narrow three cross-rule hotspots. Final disposition:

- `UnnecessarySafeCall` — narrowed in PR #320 (b4e267e), 4.7× speedup
- `NoNameShadowing` — rejected as above; 1.6s warm on kotlin is below the
  threshold that would justify re-attempting after the finding-collapse
  incident, and the dedicated scope-analysis pass listed in the execution
  order would be the right next step if/when it becomes hot enough
- `MagicNumber` — rejected as above; already <400ms warm on kotlin, out
  of top 10; ancestor-walk coalescing is semantics-preserving but
  measurement-wash

Per the issue's "document why not" clause and PR #290's precedent,
the two deferred rules are closed out with logged reasons.

### Notes

- repo-scale `kotlin` benchmarks are the acceptance signal
- local microbench wins are not enough
- new shared summaries must be cheaper than the callback work they replace
