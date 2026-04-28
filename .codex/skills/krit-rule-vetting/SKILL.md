---
name: krit-rule-vetting
description: Use when auditing Krit static-analysis rules for false positives, missing project context, library lookalikes, config mismatches, Java/Kotlin coverage gaps, or comparing findings against real Android/Kotlin projects such as Signal-Android.
---

# Krit Rule Vetting

Use this workflow before trusting large finding counts or broadening a rule.

## Start With Evidence

Run the target repo with JSON and, for performance-sensitive work, per-rule stats:

```bash
go build -o krit ./cmd/krit/
./krit -no-cache -perf -perf-rules -f json -q \
  -o /tmp/krit_target.json \
  /path/to/project || true
```

Count and sample the rule:

```bash
jq -r '.findings[].rule' /tmp/krit_target.json | sort | uniq -c | sort -nr | head -40
jq -r '.findings[] | select(.rule=="RuleName") | [.file,.line,.message] | @tsv' /tmp/krit_target.json | head -80
jq -r '.perfRuleStats[] | select(.rule=="RuleName")' /tmp/krit_target.json
```

Before judging findings, verify project config is applied. If uncertain, pass `--config /path/to/project/krit.yml`.

## Vet A Rule

For each candidate rule:

1. Read the registry entry and implementation.
2. Identify its declared capabilities: `NodeTypes`, `Languages`, `Needs*`, `TypeInfo`, `Oracle*`.
3. Inspect existing fixtures and tests.
4. Sample real findings in the target repo with surrounding code.
5. Decide whether the rule has enough evidence:
   - real library import/FQN, not just a local class with the same name
   - correct language coverage for Kotlin and Java if the rule claims both
   - correct owner/lifecycle/interface context when names are overloaded
   - correct async/background/event-boundary handling
   - correct project configuration and suppression behavior
6. Add focused positive and negative tests before changing behavior.
7. Rerun the target repo and compare counts before/after.

## Common False-Positive Patterns

- Local lookalike APIs: `Room`, `Retrofit`, `OkHttpClient`, `HttpClient`, `Dispatchers`, etc.
- Event callbacks mistaken for immediate execution, especially Compose `onX = { ... }` lambdas.
- Lifecycle-name matching without verifying the owner is an Android lifecycle type.
- Main-thread rules that ignore background boundaries such as `Dispatchers.IO`, Rx `subscribeOn`, `SimpleTask.run`, executors, or worker annotations.
- Rules that inspect all node candidates and repeatedly scan whole-file text.
- Project conventions enforced despite config or `.editorconfig` saying otherwise.

## Performance Check

If a precision fix adds source text checks, cache per-file facts instead of rescanning content for every candidate node. Confirm with:

```bash
./krit -no-cache -no-type-oracle -perf -perf-rules -f json -q \
  -o /tmp/krit_perf_after.json /path/to/project || true
jq -r '.perfRuleStats[:30][] | [.rule,.invocations,.durationMs,.avgNs,.sharePct] | @tsv' /tmp/krit_perf_after.json
```

Do not accept a precision fix that moves a rule into the top cost list unless the cost is justified.

## Fixture Standard

Every rule-vetting fix should usually add:

- one positive fixture or test that proves the intended real API/use case
- one negative local-lookalike fixture
- one negative framework-boundary fixture if async, lifecycle, callback, or Compose behavior is involved
- one real-project-inspired regression test when a false positive came from Signal or another target

## Message Standard

Finding messages should state:

- what specific call/path is risky
- what evidence made the rule believe it runs in that context
- what to move/change
- where not to move code when lifecycle/UI code must remain on the main thread

Avoid vague messages like "move this method off the main thread" when only part of the method is unsafe.
