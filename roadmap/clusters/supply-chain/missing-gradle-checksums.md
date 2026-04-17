# MissingGradleChecksums

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** warning · **Default:** inactive

## Catches

Project declares `dependencyLocking { lockAllConfigurations() }` but
has no `gradle.lockfile` in the same directory.

## Triggers

`settings.gradle.kts` enables locking; no `gradle.lockfile` exists.

## Does not trigger

Locking declared and lockfile present.

## Dispatch

File presence check.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
