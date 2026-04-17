# DependencyLicenseIncompatible

**Cluster:** [licensing](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** inactive

## Catches

Project `krit.yml` declares `license: Apache-2.0`, but a declared
`implementation` dependency is known to be `GPL-3.0` (embedded
registry).

## Triggers

Project license `Apache-2.0`; dependency is a GPL-3.0 artifact.

## Does not trigger

All dependency licenses compatible with the project license.

## Dispatch

`BuildGraph` walk + embedded license registry + project config.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
