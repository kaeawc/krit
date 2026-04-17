# BaselineDrift

**Cluster:** [sdlc/build-config](README.md) · **Status:** shipped · **Severity:** n/a (subcommand)

## Concept

Flag baseline entries for findings that no longer exist (dead
entries) or baseline entries referencing rules that have been
removed from the rule set.

## Shape

```
$ krit baseline-audit
Dead baseline entries:
  src/.../RemovedFile.kt:42 LongMethod (file no longer exists)
  src/.../Other.kt:15 OldRuleName (rule deleted)
```

## Infra reuse

- Existing baseline parser.
- Rule registry lookup.

## Links

- Parent: [`../README.md`](../README.md)
