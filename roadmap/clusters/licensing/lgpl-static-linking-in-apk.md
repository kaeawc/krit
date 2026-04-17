# LgplStaticLinkingInApk

**Cluster:** [licensing](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** inactive

## Catches

Android app with an `implementation` dependency known to be LGPL.
Static linking of LGPL libraries creates redistribution ambiguity.

## Triggers

`com.android.application` module depends on a known-LGPL artifact.

## Does not trigger

Dependency isolated to a `com.android.dynamic-feature` delivered
separately, or not LGPL.

## Dispatch

`BuildGraph` + registry lookup.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
