# MainDispatcherInLibraryCode

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Dispatchers.Main` reference inside a module marked `com.android.library`
without a corresponding `kotlinx-coroutines-android` dependency.

## Triggers

Library module with `Dispatchers.Main.immediate` reference and no
`implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android")`.

## Does not trigger

Library module with the dependency, or a non-library module.

## Dispatch

Source rule + Gradle build file cross-reference.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
