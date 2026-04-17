# GumBaseline

**Cluster:** [onboarding](README.md) · **Status:** shipped 2026-04-15 ·
**Phase:** 1 · **Severity:** n/a (script)

## What it does

Step 7: write a baseline file from the post-autofix findings so the
team starts clean. Only new findings from this point forward will
be flagged.

## Shape

```
  Writing baseline...
  ✓ Baseline written to .krit/baseline.xml
    741 existing findings suppressed
    New findings from this point forward will be flagged.

  Next steps:
    git add krit.yml .krit/baseline.xml
    git commit -m "chore: configure krit"
```

## Implementation

```bash
./krit --config krit.yml --baseline .krit/baseline.xml . >/dev/null 2>&1

gum style --foreground 82 "✓ Baseline written to .krit/baseline.xml"
gum style "  $postfixed existing findings suppressed"
gum style "  New findings from this point forward will be flagged."
echo ""
gum style --bold "Next steps:"
gum style "  git add krit.yml .krit/baseline.xml"
gum style "  git commit -m \"chore: configure krit\""
```

The baseline uses detekt's XML format for compatibility.

## Links

- Cluster root: [`README.md`](README.md)
- Depends on: [`gum-autofix-pass.md`](gum-autofix-pass.md)
