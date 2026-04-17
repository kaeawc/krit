# Experiment Matrix Notes

## Current Infra

- Experiment definitions now carry:
  - `Intent`
  - `TargetRules`
- Matrix runs now:
  - force `--no-cache` for child runs
  - extract stable finding keys as `file:line:col:rule`
  - diff each case against the baseline
  - report:
    - `EliminatedByRule`
    - `IntroducedByRule`
    - `SampleEliminated`
    - `SampleIntroduced`
  - support `--experiment-intent`
  - support `--experiment-targets` for separate per-repo case runs

## Current Experiment Catalog

- `no-name-shadowing-prune`
- `magic-number-ancestor-scan`
- `unnecessary-safe-call-local-nullability`
- `unnecessary-safe-call-structural`
- `exceptions-allowlist-cache`
- `exceptions-throw-fastpath`

## First Kotlin Singles Matrix

File:
- `/tmp/krit_kotlin_experiment_matrix_1.json`

Baseline:
- duration: `40648ms`
- findings: `242002`

Headline results:
- `exceptions-allowlist-cache`
  - duration: `39887ms`
  - findings: `241999`
  - looks like a cheap perf experiment worth keeping around
- `exceptions-throw-fastpath`
  - duration: `39588ms`
  - findings: `242050`
  - faster, but findings moved; needs FP-style review instead of perf-only promotion
- `magic-number-ancestor-scan`
  - duration: `41643ms`
  - findings: `242046`
  - `MagicNumber` callback time improved, but total got worse
- `unnecessary-safe-call-local-nullability`
  - duration: `39552ms`
  - findings: `241866`
  - very strong perf improvement for `UnnecessarySafeCall`, but findings changed materially
- `unnecessary-safe-call-structural`
  - duration: `40460ms`
  - findings: `241802`
  - slight perf movement, findings changed
- `no-name-shadowing-prune`
  - duration: `151841ms`
  - findings: `241998`
  - catastrophic regression; keep only as an experiment, do not promote

## First Multi-Target FP Matrix

File:
- `/tmp/krit_fp_matrix_multitarget_1.json`

Targets:
- `/Users/jason/github/kotlin`
- `/Users/jason/github/intellij-community`

Intent filter:
- `fp-reduction`

Headline results:
- `unnecessary-safe-call-local-nullability`
  - duration: `30964ms` mean across targets
  - eliminated:
    - `SerialVersionUIDInSerializableClass: 63`
    - `UnsafeCallOnNullableType: 6`
    - `UnsafeCast: 5`
  - introduced:
    - `UnnecessarySafeCall: 7`
    - `AvoidReferentialEquality: 1`
    - `HasPlatformType: 1`
  - this experiment is clearly changing semantics outside its target rule
- `unnecessary-safe-call-structural`
  - duration: `32466ms`
  - eliminated:
    - `AbstractClassCanBeConcreteClass: 11`
    - `SerialVersionUIDInSerializableClass: 63`
    - `UnsafeCallOnNullableType: 18`
    - `UnsafeCast: 1`
  - introduced:
    - `UnsafeCallOnNullableType: 93`
    - `UnsafeCast: 2`
  - also clearly unsafe

## Immediate Next Work

1. Add more FP-oriented experiments behind flags rather than patch/revert.
2. Keep using multi-target runs before trusting a treatment.
3. Promote only experiments with:
   - stable perf wins
   - zero introduced findings
   - acceptable eliminated findings on at least two repos
