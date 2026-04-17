# RenameRefactoring

**Cluster:** [sdlc/migration](README.md) · **Status:** shipped · **Severity:** n/a (subcommand)

## Concept

`krit rename <from-fqn> <to-fqn>` — cross-file rename powered by
the reference index and bloom filter.

## Shape

```
$ krit rename com.example.OldName com.example.NewName
Updating 47 references in 12 files.
```

## Infra reuse

- Reference index + bloom filter (same path as dead-code).
- Binary-fix engine for byte-level replacements.

## Links

- Parent: [`../README.md`](../README.md)
