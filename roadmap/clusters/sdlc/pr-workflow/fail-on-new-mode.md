# FailOnNewMode

**Cluster:** [sdlc/pr-workflow](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

`krit --delta <base-sha>` emits only findings introduced since
that ref. Baselines already do "don't fail on existing"; this adds
"fail only on new".

## Shape

```
$ krit --delta main .
# exits 0 if no new findings, non-zero otherwise
```

## Infra reuse

- Existing baselines (`--baseline`).
- Existing diff integration from
  [`diff-mode-reporting.md`](diff-mode-reporting.md).

## Links

- Parent: [`../README.md`](../README.md)
