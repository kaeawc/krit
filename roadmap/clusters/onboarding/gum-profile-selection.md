# GumProfileSelection

**Cluster:** [onboarding](README.md) · **Status:** shipped 2026-04-15 ·
**Phase:** 1 · **Severity:** n/a (script)

## What it does

Step 3: the user picks a profile based on the comparison table.

## Shape

```
? Which profile fits your team?

  > strict       — 2847 findings, maximum guardrails
    balanced     — 1632 findings, krit defaults
    relaxed      —  487 findings, large codebase friendly
    detekt-compat — 1401 findings, matches detekt defaults
```

## Implementation

```bash
profile=$(gum choose \
    "strict       — $strict_count findings, maximum guardrails" \
    "balanced     — $balanced_count findings, krit defaults" \
    "relaxed      — $relaxed_count findings, large codebase friendly" \
    "detekt-compat — $detekt_count findings, matches detekt defaults")
selected=$(echo "$profile" | awk '{print $1}')
```

## Links

- Cluster root: [`README.md`](README.md)
- Depends on: [`gum-comparison-table.md`](gum-comparison-table.md)
- Next step: [`gum-controversial-rules.md`](gum-controversial-rules.md)
