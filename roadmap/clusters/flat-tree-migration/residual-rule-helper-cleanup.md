# ResidualRuleHelperCleanup

**Cluster:** [flat-tree-migration](README.md) · **Status:** in progress ·
**Phase:** cleanup tail

## What it does

Delete remaining node-based helper code in rule files that no longer
participates in the live flat dispatch path.

This is mostly repo cleanup after the main rule migration landed.
Some helper code may need to remain temporarily while `internal/typeinfer`
still exposes node-based APIs; this item should iterate as those callers
disappear.

## Current state

This item is now downstream cleanup rather than a primary migration blocker.

Still left:

- rule/helper code that still assumes node-era `typeinfer` APIs
- helper code that still assumes scanner compatibility wrappers

This work should continue to shrink as the remaining `typeinfer` API and
scanner compatibility items are completed.

## Acceptance criteria

- Rule files no longer carry dead node-only helper paths that are unused by
  the live flat dispatch path.
- Remaining node-based helpers, if any, are clearly justified by active
  `typeinfer` or compatibility callers.
- Existing `internal/rules` tests pass.

## Links

- Parent: [`roadmap/68-flat-tree-migration.md`](../../68-flat-tree-migration.md)
- Parallel with:
  - [`migrate-typeinfer-resolve.md`](migrate-typeinfer-resolve.md)
  - [`migrate-typeinfer-scopes.md`](migrate-typeinfer-scopes.md)
  - [`scanner-compat-query-cleanup.md`](scanner-compat-query-cleanup.md)
