# Wadler-Lindig Pretty Printer

**Cluster:** [performance-infra](./README.md) · **Status:** planned ·
**Supersedes:** part of [`roadmap/27-performance-algorithms.md`](../../27-performance-algorithms.md)

## What it is

Replace string-concatenation-based output formatting with a Wadler-Lindig
pretty printer for structured finding output. The WL algorithm produces
optimal line breaks in O(n) time with bounded lookahead.

## Why

Current output formatting uses ad-hoc string building. A WL printer would
produce consistent, width-aware output for SARIF, JSON, and plain-text
formatters while being allocation-efficient.

## Implementation notes

- Target: `internal/output/` formatters
- Status: exploratory — no prototype yet
- Related: string interning (separate concept)
