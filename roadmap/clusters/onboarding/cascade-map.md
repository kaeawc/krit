# CascadeMap

**Cluster:** [onboarding](README.md) · **Status:** shipped 2026-04-15 ·
**Phase:** 1 · **Severity:** n/a (data)

## What it is

The cascade logic that chains related controversial-rule questions.
Encoded in the `controversial-rules.json` via `cascade_from` fields.
When the user answers one question, downstream questions auto-resolve
or adjust their defaults.

## Cascade groups

| Parent question | Cascades to |
|-----------------|-------------|
| Parent `id` | Cascades to (child `id`s) |
|-------------|---------------------------|
| `strict-null-safety` → yes | `allow-bang-operator` (answer derived = no), `flag-unsafe-cast` (derived = yes) |
| `enforce-compose-stability` → yes | `flag-unstable-compose-params`, `flag-compose-lambda-captures`, `flag-compose-mutable-default` |
| `enforce-dispatcher-injection` → yes | `flag-global-scope` |

`enforce-di-scope-hygiene` is NOT a cascading parent in the shipped
registry. The original plan named three aspirational rules
(SingletonOnMutableClass, HiltSingletonWithActivityDep,
ScopeOnParameterizedClass) that don't exist yet; the registry now
references the four real di-hygiene rules
(AnvilContributesBindingWithoutScope, AnvilMergeComponentEmptyScope,
BindsMismatchedArity, HiltEntryPointOnNonInterface) directly in a
single leaf question with no cascades.

When the user answers a parent, children derive their answer from
the per-profile default of the "strict" bucket on yes and the
"relaxed" bucket on no. Cascaded children are skipped in the
user-facing flow so they're never prompted.

## Implementation

The cascade logic lives in two places that agree on the bucket
rule:

- `scripts/krit-init.sh` (`apply_cascades` function): reads
  `cascade_from` from the JSON via `jq` and looks up the derived
  answer in the parent's strict/relaxed defaults.
- `internal/onboarding/writer.go` (`ResolveAnswers`): the Go
  equivalent used by both `runHeadlessInit` and the bubbletea TUI's
  `applyAnswer` method.

The TUI's `cmd/krit/init.go:applyAnswer` additionally updates the
live finding count as each cascaded child is resolved, so the
running total is always accurate.

## Links

- Cluster root: [`README.md`](README.md)
- Data source: [`controversial-rules-registry.md`](controversial-rules-registry.md)
