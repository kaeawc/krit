# GradleBuildContainsTodo

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`// TODO` inside `build.gradle(.kts)`.

## Triggers

```kotlin
// build.gradle.kts
dependencies {
    // TODO: finish wiring this
    implementation(libs.retrofit)
}
```

## Does not trigger

Same TODO in regular source — the existing `ForbiddenComment`
handles that.

## Dispatch

Line rule gated on file extension.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
