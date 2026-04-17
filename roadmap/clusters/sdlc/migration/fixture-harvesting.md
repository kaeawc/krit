# FixtureHarvesting

**Cluster:** [sdlc/migration](README.md) · **Status:** shipped · **Severity:** n/a (subcommand)

## Concept

Given a rule finding, extract the minimum-span AST subtree into a
fixture file. Useful for building krit's own test corpus.

## Shape

```
$ krit harvest /path/to/source.kt:42 --rule MagicNumber \
    --out tests/fixtures/positive/forbidden/MagicNumber-extra.kt
```

## Infra reuse

- Tree-sitter node-range extraction.
- Existing fixture test harness.

## Links

- Parent: [`../README.md`](../README.md)
