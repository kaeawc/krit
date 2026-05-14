# Krit Dogfood

Run Krit against an external Kotlin/Android repo and triage findings.

ARGUMENTS: `$ARGUMENTS` — target repo path (e.g. `~/github/metro`, `playground`, `kotlin-corpus`). Required.

## Instructions

Invoke the `krit-dogfood` skill against the target path in ARGUMENTS.

1. **Build a fresh binary.**
   ```bash
   go build -o krit ./cmd/krit/
   ```
2. **Scan with cache disabled.** Correctness checks must not trust cached findings.
   ```bash
   TARGET="$ARGUMENTS"
   ./krit -no-cache -perf -perf-rules -f json -q -o /tmp/krit_dogfood.json "$TARGET" || true
   ```
   Pass `--config` if the target's `krit.yml` is not auto-discovered.
3. **Check project profile first.**
   ```bash
   jq '.projectProfile | {hasGradle, dependencyExtractionComplete, hasUnresolvedDependencyRefs, catalogCompleteness}' /tmp/krit_dogfood.json
   ```
   If dependency extraction is incomplete, library-absence findings are suspect — route those through `krit-project-analysis`.
4. **Group findings by rule.**
   ```bash
   jq -r '.findings[] | .rule' /tmp/krit_dogfood.json | sort | uniq -c | sort -rn | head -30
   ```
5. **For each rule with surprising volume**, sample 5–10 findings and classify each cluster:
   - likely FP → route to `/krit-tighten <RuleName>`
   - correct but undesired policy → propose `DefaultActive` / `Confidence` change
   - project-context bug → route to `krit-project-analysis`
   - true positive → leave, file external issue if applicable
6. **Produce a triage table** in the response: rule | count | verdict | next action. Do not fix everything in one pass — propose one branch per fix (matches user preference).
7. **After any landing fixes**, re-scan the same target revision and report before/after finding counts.

Always include in the report: Krit revision, target revision, scan command, cache flags, whether config was applied.
