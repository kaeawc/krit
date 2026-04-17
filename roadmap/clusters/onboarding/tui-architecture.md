# TuiArchitecture

**Cluster:** [onboarding](README.md) · **Status:** shipped 2026-04-15 ·
**Phase:** 2 (unblocked) · **Severity:** n/a

## What it is

The bubbletea application architecture for `krit init --interactive`.
Embedded in `cmd/krit/`, reads the same data files as the gum script
(profile templates, controversial-rules registry, cascade map).

## Key design decisions

- Model/View/Update (bubbletea's Elm architecture)
- Pre-scan all profiles on startup, hold findings in memory
- Rule toggles re-filter in-memory findings (no re-scan)
- Shared state: `OnboardingModel` with profile, overrides, findings
- Views: profile picker, comparison table, rule questionnaire,
  autofix progress, baseline summary

## Links

- Cluster root: [`README.md`](README.md)
- Blocked on: [`gum-integration-test.md`](gum-integration-test.md)
