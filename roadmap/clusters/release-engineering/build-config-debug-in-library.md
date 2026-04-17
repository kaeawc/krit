# BuildConfigDebugInLibrary

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

`BuildConfig.DEBUG` inside a module whose `build.gradle` declares
`com.android.library`. Library `BuildConfig.DEBUG` is false at
consumer-merge time; the guard silently drops its body in release.

## Triggers

Library module referencing `BuildConfig.DEBUG`.

## Does not trigger

Same reference in an `com.android.application` module.

## Dispatch

Source rule with cross-reference to the owning module's
`build.gradle(.kts)`. Uses `BuildGraph` from supply-chain infra.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
