---
name: krit-kaa-benchmarking
description: Use when benchmarking Krit's Kotlin Analysis API oracle on a large Kotlin/Android repo, comparing cold vs warm KAA behavior, or investigating KAA regressions, oracle filters, call-target filters, and rule-driven KAA workload.
---

# Krit KAA Benchmarking

Use this workflow for cold/warm Kotlin Analysis API benchmarking on a representative target repository.

## Setup

Build a fresh Krit binary and verify the KAA helper artifact exists:

```bash
go build -o krit ./cmd/krit/
test -f tools/krit-types/build/libs/krit-types.jar
```

Record the Krit revision and target revision in prose when reporting results.

Set a target path explicitly:

```bash
TARGET=/path/to/project
```

If the target config is not discovered from the scan root, pass `--config`.

## Cold Oracle Run

Use the checked-in benchmark script when possible. It clears the oracle cache for each run and uses cache-disabling flags appropriate for cold KAA measurement.

```bash
KRIT="$PWD/krit" scripts/benchmark-oracle.sh "$TARGET"
```

Report phase timings and oracle workload fields from JSON output, especially:

- total duration
- type-oracle phase
- JVM analyze phase
- KAA files analyzed
- oracle call/declaration filter activity
- active rules that requested oracle facts

## Warm Oracle Run

After a cold run has populated oracle artifacts, measure warm behavior while disabling the incremental findings cache:

```bash
./krit -no-cache -perf -f json -q -o /tmp/krit_kaa_warm.json "$TARGET" || true
```

Warm KAA should avoid a JVM analyze phase when the oracle cache is valid. If total runtime is high, inspect rule execution, cross-file analysis, Android/Gradle project analysis, and config before blaming KAA.

## Regression Search

When KAA cost regresses:

1. Compare active rules that declare `NeedsOracle`, oracle call targets, declaration needs, or oracle diagnostics.
2. Look for broad call-target filters, missing lexical hints, or declaration needs that force wider extraction than the rule requires.
3. Separate Go-side rule cost from oracle cost with `-no-type-oracle -perf -perf-rules`.
4. Confirm target config, cache flags, and environment are stated with the result.

## Reporting Standard

Always include:

- Krit revision
- target revision
- scan command
- cache flags
- whether target config was applied
- the key phase timings
- any rule/config change being evaluated

Do not treat cached warm findings as behavioral truth after rule or config changes; use cache-disabling flags for correctness checks.
