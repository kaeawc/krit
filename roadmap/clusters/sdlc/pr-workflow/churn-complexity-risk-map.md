# ChurnComplexityRiskMap

**Cluster:** [sdlc/pr-workflow](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

Files that change a lot AND are complex are the usual places
incidents happen. Combine git churn with cyclomatic complexity
scores.

## Shape

```
$ krit risk-map --since 90d
/src/.../UserRepository.kt: churn=42 complexity=38 → risk=1596
/src/.../AuthController.kt: churn=18 complexity=22 → risk=396
```

## Infra reuse

- Existing cyclomatic complexity rule output.
- New: git reader that counts commits per file in a window.

## Links

- Parent: [`../README.md`](../README.md)
