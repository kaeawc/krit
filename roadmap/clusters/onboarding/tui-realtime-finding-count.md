# TuiRealtimeFindingCount

**Cluster:** [onboarding](README.md) · **Status:** shipped 2026-04-15 ·
**Phase:** 2 (unblocked) · **Severity:** n/a

## What it is

As the user toggles rules or adjusts thresholds, the total finding
count, fixable count, and per-rule breakdown update immediately.
No re-scan — the full scan result is held in memory and re-filtered
on each interaction.

This is the core UX differentiator of the bubbletea TUI vs the gum
script: immediate feedback on every decision.

## Links

- Cluster root: [`README.md`](README.md)
- Blocked on: [`gum-integration-test.md`](gum-integration-test.md)
