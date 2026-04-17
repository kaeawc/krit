# GumProfileScan

**Cluster:** [onboarding](README.md) · **Status:** shipped 2026-04-15 ·
**Phase:** 1 · **Severity:** n/a (script)

## What it does

Step 1 of the gum onboarding: scan the target directory with each
profile sequentially, caching parse results from the first scan.

## Shape

```
$ scripts/krit-init.sh [path]

  Scanning with 4 profiles...
  ⠋ strict     — 2847 findings (1204 fixable) across 417 rules
  ⠋ balanced   — 1632 findings (891 fixable) across 312 rules
  ⠋ relaxed    — 487 findings (302 fixable) across 198 rules
  ⠋ detekt     — 1401 findings (764 fixable) across 230 rules
```

Uses `gum spin` for each scan. First run: `krit --config strict.yml .`
which populates the incremental cache. Subsequent profiles:
`krit --config balanced.yml .` which reuses the cached parse and
only re-evaluates rules.

## Implementation

```bash
for profile in strict balanced relaxed detekt-compat; do
    gum spin --title "Scanning with $profile profile..." -- \
        ./krit --config "config/profiles/${profile}.yml" \
               -f json "$target" > "$tmpdir/${profile}.json" 2>/dev/null
done
```

Parse each JSON output to extract total findings, fixable count,
and rule count for the comparison table.

## Links

- Cluster root: [`README.md`](README.md)
- Next step: [`gum-comparison-table.md`](gum-comparison-table.md)
