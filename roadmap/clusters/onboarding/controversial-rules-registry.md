# ControversialRulesRegistry

**Cluster:** [onboarding](README.md) · **Status:** shipped 2026-04-15 ·
**Phase:** 1 · **Severity:** n/a (data)

## What it is

A JSON file at `config/onboarding/controversial-rules.json` that
maps each onboarding question to its rule cluster, cascade
dependencies, rationale text, fixture paths, and per-profile
defaults.

## Shape

```json
{
  "questions": [
    {
      "id": "allow-bang-operator",
      "question": "Allow the !! (not-null assertion) operator?",
      "rationale": "!! throws NPE on null. Some teams ban it; others consider it idiomatic for known-nonnull patterns.",
      "rules": ["UnsafeCallOnNullableType", "MapGetWithNotNullAssertionOperator"],
      "cascade_from": null,
      "defaults": {
        "strict": false,
        "balanced": true,
        "relaxed": true,
        "detekt-compat": true
      },
      "positive_fixture": "tests/fixtures/positive/potential-bugs/UnsafeCallOnNullableType.kt",
      "negative_fixture": "tests/fixtures/negative/potential-bugs/UnsafeCallOnNullableType.kt"
    }
  ]
}
```

Both the gum script and the bubbletea TUI read this file. Adding a
new controversial rule = adding one entry.

## Links

- Cluster root: [`README.md`](README.md)
- Consumed by: [`gum-controversial-rules.md`](gum-controversial-rules.md),
  [`tui-split-pane-explorer.md`](tui-split-pane-explorer.md)
