# OpenForTestingCallerInNonTest

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`@OpenForTesting` (kotlin-allopen) type subclassed outside a test
source set.

## Triggers

Production subclass of an `@OpenForTesting` class.

## Does not trigger

Subclass only in test code.

## Dispatch

Cross-file supertype reference walk.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
