# TestSelection

**Cluster:** [sdlc/testing-infra](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

Given a PR diff, compute which test files reference the touched
symbols; power a fast-test CI mode.

## Shape

```
$ krit select-tests --base main
feature/FooTest.kt
core/RepositoryTest.kt
```

## Infra reuse

- Reference index.
- Diff integration from
  [`../pr-workflow/diff-mode-reporting.md`](../pr-workflow/diff-mode-reporting.md).

## Links

- Parent: [`../README.md`](../README.md)
