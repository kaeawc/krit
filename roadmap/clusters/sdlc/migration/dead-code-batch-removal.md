# DeadCodeBatchRemoval

**Cluster:** [sdlc/migration](README.md) · **Status:** shipped · **Severity:** n/a (subcommand)

## Concept

`krit --remove-dead-code` — safe-delete mode that walks the
existing `DeadCode` / `ModuleDeadCode` findings and removes them in
bulk, stopping at any ambiguous site (reflection, DI graph root,
test-only reference).

## Shape

```
$ krit --remove-dead-code --dry-run
removing 42 functions, 12 classes, 3 files
```

## Infra reuse

- Existing dead-code detection.
- Existing binary-fix engine.

## Links

- Parent: [`../README.md`](../README.md)
