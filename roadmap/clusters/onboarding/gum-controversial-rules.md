# GumControversialRules

**Cluster:** [onboarding](README.md) · **Status:** shipped 2026-04-15 ·
**Phase:** 1 · **Severity:** n/a (script)

## What it does

Step 4: iterate through the ~15 controversial-rule questions with
cascade logic. Each question controls a cluster of related rules.
Defaults depend on the selected profile.

## Shape

```
? Allow the !! (not-null assertion) operator?
  !! throws NPE on null. Some teams ban it entirely; others consider
  it idiomatic for known-nonnull patterns like requireNotNull()!!.
  Controls: UnsafeCallOnNullableType, MapGetWithNotNullAssertionOperator

  > Yes (allow — disable the rule)
    No (flag — enable the rule)

  → also disabled: MapGetWithNotNullAssertionOperator (linked)

? Enforce Compose stability?
  Flag @Composable functions with unstable parameters (List, Map, Set
  without @Immutable). Catches unnecessary recomposition but is noisy
  without kotlinx.collections.immutable.
  Controls: ComposeUnstableParameter, ComposeLambdaCapturesUnstableState,
            ComposeMutableDefaultArgument

  [skipped — implied by "Strict null safety?" = no]
```

## Cascade behavior

After each `gum confirm`, check `cascade_from` in the registry. If
the answer triggers cascaded rules:
- Log "→ also enabled/disabled: <rules> (linked)"
- Skip the cascaded question when it comes up in the sequence
- The skipped question shows as "[skipped — implied by ...]"

## Implementation

```bash
# Read questions from the registry
questions=$(jq -r '.questions[].id' "$registry")

for qid in $questions; do
    # Check if this question was cascaded by a prior answer
    if is_cascaded "$qid"; then
        log_skipped "$qid"
        continue
    fi

    question=$(jq -r ".questions[] | select(.id == \"$qid\") | .question" "$registry")
    rationale=$(jq -r ".questions[] | select(.id == \"$qid\") | .rationale" "$registry")
    default=$(jq -r ".questions[] | select(.id == \"$qid\") | .defaults.$selected" "$registry")

    echo ""
    gum style --foreground 212 "$question"
    gum style --faint "$rationale"

    if [ "$default" = "true" ]; then
        answer=$(gum confirm --default=yes "$question" && echo "yes" || echo "no")
    else
        answer=$(gum confirm --default=no "$question" && echo "yes" || echo "no")
    fi

    apply_answer "$qid" "$answer"
    apply_cascades "$qid" "$answer"
done
```

## Links

- Cluster root: [`README.md`](README.md)
- Data source: [`controversial-rules-registry.md`](controversial-rules-registry.md),
  [`cascade-map.md`](cascade-map.md)
- Next step: [`gum-write-config.md`](gum-write-config.md)
