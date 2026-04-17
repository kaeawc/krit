# TestFixtureAccessedFromProduction

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

Usage of a type declared under `src/testFixtures/` from a non-test
file.

## Triggers

Production module referencing `FakeUser` from
`src/testFixtures/kotlin/.../FakeUser.kt`.

## Does not trigger

Reference is only from a test file.

## Dispatch

Cross-file reference walk gated on the declaring file's path.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
