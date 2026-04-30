# Rule Execution Leverage Plan

## Goal

Reduce `ruleExecution` wall-clock by attacking the highest-leverage structural costs instead of continuing with isolated per-rule micro-optimizations.

The current rule engine already has the right coarse architecture:

- one dispatcher walk per file
- O(1) symbol-indexed rule dispatch
- parallel execution across files

The remaining cost is mostly inside rule callbacks. The leverage areas are:

1. shared fact extraction
2. fewer repeated subtree rescans
3. more named-child / pruned traversal
4. replacing per-rule ad hoc analysis with reusable indexes

## Current Model

Execution path:

1. parse file into tree-sitter AST
2. optionally build type index
3. build suppression index
4. run dispatch walk
5. run line rules
6. run project-scope rule phases
7. suppression filter

Observed behavior:

- dispatcher framework cost is not the primary problem
- callback work dominates
- the hottest rules repeatedly rediscover local facts:
  - lexical scope names
  - jump counts
  - literal classifications
  - modifier facts
  - local nullability hints

## Strategy Overview

### Track A: Shared Fact Extraction

Build small per-file analysis products once, then let multiple rules query them.

Candidate shared facts:

- scope/declaration summaries
- function control-flow summaries
- literal metadata
- modifier/declaration summaries
- simple local nullability summaries

Success criteria:

- at least 2 hot rules consume the same fact product
- file-level preprocessing is cheaper than the repeated callback work it replaces

### Track B: Fewer Repeated Subtree Rescans

Move rules away from:

- callback receives node
- callback recursively scans the same subtree again

Toward:

- callback receives node
- callback performs direct-child inspection or queries precomputed summary

Patterns to remove:

- `scanner.WalkAllNodes(...)` inside `CheckNode`
- repeated `scanner.NodeText(...)` over large subtrees
- repeated regex scans over full node text when AST structure is already enough

### Track C: More Named-Child / Pruned Traversal

Reduce tree-sitter churn from punctuation and irrelevant syntax nodes.

Tactics:

- prefer `NamedChildCount` / `NamedChild`
- stop recursion at known leaf or irrelevant node types
- centralize traversal helpers so pruning policy is shared

This is mostly a constant-factor optimization, but it matters because the engine is node-volume heavy.

### Track D: Reusable Indexes Instead Of Ad Hoc Analysis

Introduce reusable per-file indexes for recurring analysis families.

Target shape:

- file gets analyzed once into small immutable summaries
- rules query those summaries cheaply
- summaries are keyed by file path + root node range where needed

This is the strongest medium-term direction for the hot rules.

## Work Plan

### Phase 1: Build Inventory And Shared Primitives

1. classify top 20 hot rules by analysis type
2. identify repeated facts across those rules
3. define minimal shared summaries, not a giant generic framework
4. add perf timing around any new preprocessing phase

Deliverable:

- hotspot classification doc
- first 2-3 shared summary APIs

### Phase 2: Land One Shared Index Per Rule Family

Recommended order:

1. control-flow summary index
2. declaration / modifier summary index
3. literal summary index
4. local nullability summary index

Rules should migrate in small batches so repo-level perf can be measured after each batch.

### Phase 3: Normalize Traversal Helpers

1. audit hot paths for `ChildCount` vs `NamedChildCount`
2. add shared pruned traversal helpers
3. replace bespoke recursive walkers in hot rules

This is lower-risk than full architectural refactors and should happen continuously.

### Phase 4: Convert High-Cost Rules To Query Shared Facts

Priority order should follow repo-scale hotspots, not local benchmarks.

Recommended order for `kotlin`:

1. `MagicNumber`
2. `UnnecessarySafeCall`
3. `ThrowingExceptionsWithoutMessageOrCause`
4. `ReturnCount` / `ThrowsCount`
5. complexity family

`NoNameShadowing` is still special enough that it likely needs its own dedicated scope-analysis pass rather than being bundled with a lighter shared index.

## Proposed Data Products

### 1. Declaration Summary Index

Per file:

- classes / objects / functions / properties
- declaration names
- modifiers
- direct member lists
- parameter lists

Use for:

- `UnusedParameter`
- `LongParameterList`
- `ThrowingExceptionsWithoutMessageOrCause`
- many naming and style rules

### 2. Function Control-Flow Summary

Per function:

- total returns
- total throws
- guard-clause returns/throws
- nested lambda presence
- cyclomatic-ish branch counters
- block depth summary

Use for:

- `ReturnCount`
- `ThrowsCount`
- `CyclomaticComplexMethod`
- `NestedBlockDepth`
- `LongMethod`

### 3. Literal Summary Index

Per file or per declaration:

- numeric literals by normalized value
- context tags:
  - annotation
  - const property
  - enum entry
  - named argument
  - range endpoint
  - compose unit
  - color literal

Use for:

- `MagicNumber`
- related formatting/style literal rules

### 4. Local Nullability Summary

Per function / lexical scope:

- explicitly nullable locals
- obviously non-null immutable locals
- direct null-check guards
- type-predicate guards

Use for:

- `UnnecessarySafeCall`
- `UnnecessaryNotNullOperator`
- adjacent null-safety rules

Status:

- tried in a narrow cached form inside [`internal/rules/potentialbugs_nullsafety.go`](/Users/jason/kaeawc/krit/internal/rules/potentialbugs_nullsafety.go)
- rejected after 1 full `~/github/kotlin` repo run
- result:
  - baseline [`/tmp/krit_kotlin_declsummary_pass1_rerun.json`](/tmp/krit_kotlin_declsummary_pass1_rerun.json): total `37997ms`, `ruleExecution` `22878ms`, `UnnecessarySafeCall` `12238ms`
  - attempt [`/tmp/krit_kotlin_localnull_pass1.json`](/tmp/krit_kotlin_localnull_pass1.json): total `39860ms`, `ruleExecution` `22126ms`, `UnnecessarySafeCall` `2541ms`
- interpretation:
  - the rule callback got much cheaper
  - total wall clock regressed by about `1.86s`
  - unrelated bucket shifts (`typeIndex`, Android, and non-target rule timings) made it a net loss
  - the finding totals also changed, so the experiment is not safe to keep as-is
- next version, if retried, should be narrower and query-shaped rather than file-wide caching

## Guardrails

- prefer file-local immutable summaries
- avoid generic “analysis context” abstractions until multiple consumers exist
- benchmark on `~/github/kotlin` after every accepted migration
- keep perf timing for new shared phases visible in `--perf`
- avoid making a rule locally faster if it increases total dispatcher contention or allocation pressure
- reject traversal-pruning passes that materially change finding counts, even if wall clock improves

## Definition Of Success

Short term:

- reduce repeated callback CPU for the current top non-`NoNameShadowing` rules
- keep total wall-clock improvements visible on `kotlin`

Medium term:

- shift rule execution from callback-heavy subtree rescans to cheap summary lookups
- make new optimizations apply to clusters of rules, not just one rule at a time

## Experiment Log

### 2026-04-10: Shared Function Summary Attempt

Tried:

- one cached per-function summary to unify:
  - jump counts
  - nested depth
  - cyclomatic
  - cognitive complexity

Migrated:

- `ReturnCount` jump metrics source
- `ThrowsCount` jump metrics source
- `NestedBlockDepth`
- `CyclomaticComplexMethod` for the non-`IgnoreSimpleWhenEntries` path
- `CognitiveComplexMethod` via shared complexity accessor

Result:

- rejected

Repo-scale `kotlin` benchmark regressed:

- baseline: `/tmp/krit_kotlin_rulefocus_returncount.json`
  - total `40646ms`
  - `ruleExecution` `23765ms`
- attempt rerun: `/tmp/krit_kotlin_functionsummary_pass1_rerun.json`
  - total `42453ms`
  - `ruleExecution` `24592ms`

Interpretation:

- the unified summary increased total work or cache pressure enough to lose at repo scale
- local sharing across these rules was not enough to offset the heavier combined pass
- future shared-summary work should stay narrower and be benchmarked as a smaller batch

### 2026-04-10: Header-Only Declaration Summary

Tried:

- cached function/class header summary only
- no body-wide summary

Migrated:

- `LongParameterList`
- `UnusedParameter`

Result:

- accepted

Repo-scale `kotlin` benchmark improved in two serial runs:

- baseline: `/tmp/krit_kotlin_rulefocus_returncount.json`
  - total `40646ms`
  - `ruleExecution` `23765ms`
- run 1: `/tmp/krit_kotlin_declsummary_pass1.json`
  - total `39660ms`
  - `ruleExecution` `22486ms`
- run 2: `/tmp/krit_kotlin_declsummary_pass1_rerun.json`
  - total `38005ms`
  - `ruleExecution` `22878ms`

Interpretation:

- narrow declaration/header reuse works better than a broad function-body summary
- this is the current accepted shared-index direction

### 2026-04-10: Literal Context Cache For MagicNumber

Tried:

- cached ancestor/context flags for numeric literals
- used only by `MagicNumber`

Result:

- rejected

Repo-scale `kotlin` regressions versus the accepted declaration-summary baseline:

- baseline: `/tmp/krit_kotlin_declsummary_pass1_rerun.json`
  - total `38005ms`
  - `ruleExecution` `22878ms`
- run 1: `/tmp/krit_kotlin_literalctx_pass1.json`
  - total `38698ms`
  - `ruleExecution` `23149ms`
- run 2: `/tmp/krit_kotlin_literalctx_pass1_rerun.json`
  - total `38554ms`
  - `ruleExecution` `23089ms`

Interpretation:

- `MagicNumber` itself got cheaper
- whole-run cost still went up
- likely not enough payoff to justify the extra cache/build overhead
