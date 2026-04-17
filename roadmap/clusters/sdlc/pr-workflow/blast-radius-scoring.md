# BlastRadiusScoring

**Cluster:** [sdlc/pr-workflow](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

Given a PR diff, resolve the changed symbols, use the reference
index to count downstream consumers, and emit a score.

## Shape

```
$ krit blast-radius --base main
Changed symbols: 12
Direct consumers: 47 files across 12 modules
High-fan-in changes:
  - com.example.UserRepository.save: 138 consumers
```

## Infra reuse

- Cross-file reference index.
- Bloom filter for O(1) consumer lookup.

## Links

- Parent: [`../README.md`](../README.md)
- Related: [`diff-mode-reporting.md`](diff-mode-reporting.md)
