# TuiThresholdSliders

**Cluster:** [onboarding](README.md) · **Status:** shipped 2026-04-15 ·
**Phase:** 2 (unblocked) · **Severity:** n/a

## What it is

Numeric input for threshold-based rules (LongMethod, LongParameterList,
TooManyFunctions, CyclomaticComplexMethod). Slider or number field
with live finding count update: "At threshold 60: 12 functions flagged.
At 80: 3. At 100: 0."

Uses the pre-scanned findings in memory — re-filter by threshold,
no re-scan.

## Links

- Cluster root: [`README.md`](README.md)
- Blocked on: [`gum-integration-test.md`](gum-integration-test.md)
