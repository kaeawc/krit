# GumAutofixPass

**Cluster:** [onboarding](README.md) · **Status:** shipped 2026-04-15 ·
**Phase:** 1 · **Severity:** n/a (script)

## What it does

Step 6: run krit with the final config and `--fix` to apply all
safe autofixes. Show a summary of what changed.

## Shape

```
  Applying safe autofixes...
  ⠋ Running krit --config krit.yml --fix .

  ✓ Fixed 891 findings across 147 files
    - 312 MagicNumber → extracted to named constants
    - 201 TrailingWhitespace → trimmed
    - 142 NewLineAtEndOfFile → added
    - 89 RedundantConstructorKeyword → removed
    - 47 UnnecessaryParentheses → removed
    ... and 100 more

  Remaining: 741 findings (not auto-fixable)
```

## Implementation

```bash
gum spin --title "Applying safe autofixes..." -- \
    ./krit --config krit.yml --fix -f json "$target" \
    > "$tmpdir/postfix.json" 2>/dev/null

# Compare pre-fix vs post-fix totals
prefixed=$(jq '.summary.total' "$tmpdir/${selected}.json")
postfixed=$(jq '.summary.total' "$tmpdir/postfix.json")
fixed=$((prefixed - postfixed))

gum style --foreground 82 "✓ Fixed $fixed findings"
```

The autofix pass uses the user's final config, not the profile
template — so only enabled rules' fixes are applied.

## Links

- Cluster root: [`README.md`](README.md)
- Depends on: [`gum-write-config.md`](gum-write-config.md)
- Next step: [`gum-baseline.md`](gum-baseline.md)
