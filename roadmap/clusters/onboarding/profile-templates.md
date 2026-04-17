# ProfileTemplates

**Cluster:** [onboarding](README.md) · **Status:** shipped 2026-04-15 ·
**Phase:** 1 · **Severity:** n/a (data)

## What it is

Four `krit.yml` files under `config/profiles/` — one per profile
(strict, balanced, relaxed, detekt-compat). Each is a complete,
valid config that krit can load directly with `--config`.

## Shape

```
config/profiles/strict.yml      — all rules active, tight thresholds
config/profiles/balanced.yml    — current krit defaults
config/profiles/relaxed.yml     — noisy rules off, thresholds raised
config/profiles/detekt-compat.yml — matches detekt default set
```

Each file is a full `krit.yml` including every rule's `active` state
and any threshold overrides. Generated from the existing
`config/default-krit.yml` with per-profile deltas.

## Delivery

Extract the current defaults into `balanced.yml`. Derive `strict.yml`
by enabling all `DefaultInactive` rules and tightening thresholds.
Derive `relaxed.yml` by disabling the controversial rules and raising
thresholds. Derive `detekt-compat.yml` from detekt's documented
defaults.

## Links

- Cluster root: [`README.md`](README.md)
