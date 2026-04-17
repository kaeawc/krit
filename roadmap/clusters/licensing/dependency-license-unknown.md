# DependencyLicenseUnknown

**Cluster:** [licensing](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

Dependency whose coordinates aren't in the embedded license
registry and `krit.yml` requires license verification.

## Triggers

`com.example:proprietary-lib:1.0` not in the registry.

## Does not trigger

All dependencies covered, or verification disabled in config.

## Dispatch

`BuildGraph` + registry lookup; config-gated.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
