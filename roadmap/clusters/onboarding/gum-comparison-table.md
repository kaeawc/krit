# GumComparisonTable

**Cluster:** [onboarding](README.md) · **Status:** shipped 2026-04-15 ·
**Phase:** 1 · **Severity:** n/a (script)

## What it does

Step 2: render a comparison table showing each profile's findings,
fixable count, rule count, and top 5 rules by finding count.

## Shape

```
┌──────────────┬──────────┬─────────┬───────┬─────────────────────────────────────┐
│ Profile      │ Findings │ Fixable │ Rules │ Top rules                           │
├──────────────┼──────────┼─────────┼───────┼─────────────────────────────────────┤
│ strict       │    2847  │   1204  │   417 │ MagicNumber(312) LongMethod(89)     │
│              │          │         │       │ ForbiddenComment(76) SpreadOp(61)   │
│              │          │         │       │ UnsafeCall(58)                      │
├──────────────┼──────────┼─────────┼───────┼─────────────────────────────────────┤
│ balanced     │    1632  │    891  │   312 │ MagicNumber(312) LongMethod(89)     │
│              │          │         │       │ UnsafeCall(58) NoNameShadow(42)     │
│              │          │         │       │ UnsafeCast(31)                      │
├──────────────┼──────────┼─────────┼───────┼─────────────────────────────────────┤
│ relaxed      │     487  │    302  │   198 │ LongMethod(89) UnsafeCast(31)       │
│              │          │         │       │ NoNameShadow(42) MatchDecl(28)      │
│              │          │         │       │ ReturnCount(19)                     │
├──────────────┼──────────┼─────────┼───────┼─────────────────────────────────────┤
│ detekt-compat│    1401  │    764  │   230 │ MagicNumber(312) LongMethod(89)     │
│              │          │         │       │ UnsafeCall(58) ForbiddenComment(76) │
│              │          │         │       │ SpreadOperator(61)                  │
└──────────────┴──────────┴─────────┴───────┴─────────────────────────────────────┘
```

Rendered using `gum style` with box borders. Top 5 extracted from
the JSON output's `summary.byRule` field, sorted by count descending.

## Implementation

Parse each profile's JSON output with `jq` to build the table data,
then render with `gum style --border rounded`.

## Links

- Cluster root: [`README.md`](README.md)
- Depends on: [`gum-profile-scan.md`](gum-profile-scan.md)
- Next step: [`gum-profile-selection.md`](gum-profile-selection.md)
