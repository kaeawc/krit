---
name: krit-dogfood
description: Use when running Krit against an external Kotlin/Android repo (metro, kotlin-corpus, playground, or any user-provided path) to harvest false positives, find missed positives, or validate that a rule change behaves correctly on real code. Produces a triage list of rule findings worth fixing vs ignoring vs deferring.
---

# Krit Dogfood

Use this when validating Krit against a real codebase, not just fixtures. Cross-link with `krit-rule-vetting` for the per-rule audit step, `krit-tighten-rule` to land the fixes, and `krit-project-analysis` when the cause is a missing project-profile guard rather than rule logic.

## Pick A Target

Useful targets, in rough order of cost/coverage trade-off:

- **playground** (`playground/` in this repo) — fastest, regression-locked via `make regression`. Use first for any pipeline/rule change.
- **kotlin-corpus** — broader Kotlin coverage, used by `scripts/benchmark-oracle.sh` and `test(serve): report per-phase timings in kotlin-corpus benches`.
- **`~/github/metro`** or similar large Android app — real Android/Gradle/Compose surface, real catalog, real DI.
- **user-supplied path** — when investigating a reported FP on the reporter's codebase.

Record both the Krit revision and the target revision in any output you share.

## Run With Cache Disabled

For correctness checks, never trust cached findings. Use:

```bash
go build -o krit ./cmd/krit/

TARGET=/path/to/project
./krit -no-cache -perf -perf-rules -f json -q -o /tmp/krit_dogfood.json "$TARGET" || true
```

If the target's `krit.yml` is not auto-discovered, pass `--config`. The project profile drives most library-aware rule behavior — check it first:

```bash
jq '.projectProfile | {hasGradle, dependencyExtractionComplete, hasUnresolvedDependencyRefs, catalogCompleteness}' /tmp/krit_dogfood.json
```

If dependency extraction is incomplete, library-absence is not proof, and many "FPs" are actually project-context bugs — see `krit-project-analysis`.

## Triage

Group findings by rule, then by suspected verdict:

```bash
jq -r '.findings[] | .rule' /tmp/krit_dogfood.json | sort | uniq -c | sort -rn | head -30

jq -r '.findings[] | select(.rule=="<RuleName>") | [.file,.line,.message] | @tsv' /tmp/krit_dogfood.json | head -50
```

For each rule with surprising volume:

- **Likely FP cluster** — sample 5–10 findings, look for a single evidence-gap bucket (lexical state, local lookalike, missing receiver proof, scope boundary). Route to `krit-tighten-rule`.
- **Likely correct but undesired** — rule is firing as designed but the team disagrees with the policy. Downgrade `DefaultActive` or `Confidence`, or re-evaluate against `docs/rule-scope.md`.
- **Likely project-context bug** — library-absence-based finding on a project with incomplete dependency extraction. Route to `krit-project-analysis`.
- **Likely true positive** — leave the finding, file an issue if external.

Don't fix everything in one pass. Produce a triage list with explicit verdicts and tackle them as separate PRs (matches the user's preference for one branch per fix).

## Lock The Regression

For every FP you decide to fix, add the minimal negative fixture to `tests/fixtures/negative/<category>/<RuleName>/` *before* changing rule code. The fixture must fail on the pre-fix code and pass after. Same workflow as `krit-tighten-rule`.

For project-context bugs, additionally add a project-profile fixture if the missing fact is something `librarymodel` should expose.

## Re-Run After Changes

After landing fixes, re-run the dogfood scan against the same target revision and confirm:

- the FP cluster on that rule dropped to the expected count
- no other rule regressed (compare finding counts per rule before/after)
- per-phase timings did not regress meaningfully (use `krit-kaa-benchmarking` for the rigorous version)

## Reporting

When sharing dogfood results, always include:

- Krit revision and target revision
- the exact scan command and flags (especially `-no-cache`)
- whether project config was applied
- the per-rule finding counts before and after any change
- one representative example per rule cluster

Findings without revisions and flags are not reproducible and should not be acted on.
