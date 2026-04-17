# Test Coverage Backlog

**Cluster:** [sdlc](./README.md) · **Sub-cluster:** testing-infra ·
**Status:** in-progress ·
**Supersedes:** roadmap items 29–42 (test coverage tranche)

## What it is

Consolidated backlog of test coverage gaps identified in a single audit pass.
Originally split across 14 individual roadmap files (items 29-42), now tracked
as one work stream.

## Coverage areas (by priority)

### High priority
- **Config package** (item 29): YAML loading, merge, override edge cases
- **CLI flag integration** (item 31): end-to-end flag parsing, `--diff`, `--baseline`
- **Scanner package** (item 34): cross-file index, bloom filter, baseline merge
- **Remaining rule tests** (item 35): ~40 rules with no unit tests
- **LSP server** (item 36): incremental reparse, multi-file workspace

### Medium priority
- **Cache package** (item 30): content-hash invalidation, concurrent access
- **Suppression edge cases** (item 32): nested scopes, `@SuppressWarnings`, `detekt:` prefix
- **Core rule unit tests** (item 33): dispatch ordering, confidence, fix safety
- **MCP suggest_fixes** (item 37): tool invocation, finding-to-fix mapping
- **Untested exported functions** (item 38): public API surface coverage
- **Untested source files** (item 42): files with 0% test coverage

### Low priority
- **Final coverage gaps** (item 39): long-tail uncovered branches
- **Remaining function tests** (item 40): individual untested functions

## Implementation notes

- Item 41 (coverage-summary) is a reference doc, not actionable
- Many gaps have partially closed since the audit (2,540 test functions now exist)
- Progress tracked incrementally — no single milestone needed
