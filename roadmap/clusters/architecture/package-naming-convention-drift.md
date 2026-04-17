# PackageNamingConventionDrift

**Cluster:** [architecture](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

A class declared in `src/main/kotlin/com/example/feature/foo/Bar.kt`
whose package doesn't start with `com.example.feature.foo`.

## Triggers

```kotlin
// in feature/foo/ui/Screen.kt
package com.example.other.location
```

## Does not trigger

Package matches the directory path.

## Dispatch

`package_header` + file path comparison. Extends existing
`InvalidPackageDeclaration`; this adds project-level convention
enforcement on top.

## Links

- Parent: [`../README.md`](../README.md)
