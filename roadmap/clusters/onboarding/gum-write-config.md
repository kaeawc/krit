# GumWriteConfig

**Cluster:** [onboarding](README.md) · **Status:** shipped 2026-04-15 ·
**Phase:** 1 · **Severity:** n/a (script)

## What it does

Step 5: generate the final `krit.yml` from the profile template plus
the user's controversial-rule overrides.

## Shape

```
  Writing krit.yml...
  ✓ Based on: balanced profile
  ✓ Overrides: 6 rules adjusted
    - UnsafeCallOnNullableType: disabled
    - MagicNumber: disabled
    - InjectDispatcher: enabled
    - ComposeUnstableParameter: enabled
    - ComposeLambdaCapturesUnstableState: enabled (linked)
    - ComposeMutableDefaultArgument: enabled (linked)
```

## Implementation

1. Copy the selected profile template to `krit.yml`.
2. For each rule override from the controversial-rules pass, use
   `yq` or `sed` to toggle `active: true/false` in the appropriate
   ruleset section.
3. Print the summary of overrides applied.

Fallback if `yq` is not installed: build the override block as
a YAML fragment and append it to the profile template, relying on
krit's config merge behavior (later keys override earlier ones).

## Links

- Cluster root: [`README.md`](README.md)
- Depends on: [`gum-controversial-rules.md`](gum-controversial-rules.md)
- Next step: [`gum-autofix-pass.md`](gum-autofix-pass.md)
