# JvmTargetMismatch

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

Gradle 8.0+ validates this at compilation time via
`kotlin.jvm.target.validation.mode=error`, but that check runs
minutes into a build. krit catches it in milliseconds — surfacing
the error in the editor via LSP or in a pre-commit hook before
`./gradlew` even starts.

## Catches

`kotlinOptions.jvmTarget` and `compileOptions.sourceCompatibility`
disagree within the same compilation target. A KMP module with
`android { jvmTarget = "11" }` and `jvm { jvmTarget = "17" }` is
fine — each target is internally consistent. The rule only flags
when the two values conflict within a single target.

## Triggers

```kotlin
android {
    compileOptions { sourceCompatibility = JavaVersion.VERSION_11 }
    kotlinOptions { jvmTarget = "17" }
}
```

## Does not trigger

```kotlin
// Same target, both agree
android {
    compileOptions { sourceCompatibility = JavaVersion.VERSION_17 }
    kotlinOptions { jvmTarget = "17" }
}

// KMP: different targets, each internally consistent
kotlin {
    jvm { compilations.all { kotlinOptions.jvmTarget = "17" } }
    android { compilations.all { kotlinOptions.jvmTarget = "11" } }
}
android {
    compileOptions { sourceCompatibility = JavaVersion.VERSION_11 }
}

// Toolchain set — jvmTarget and sourceCompatibility derived from it
kotlin { jvmToolchain(17) }
```

## Configuration

No configuration. The per-target comparison is always correct — a
mismatch between `jvmTarget` and `sourceCompatibility` within the
same compilation target is always a bug. Baseline if needed.

## Dispatch

Single-file scan. Parse `android { compileOptions { } }` and
`kotlinOptions { }` blocks, grouping by compilation target. For KMP
modules, each `kotlin { jvm { } }` and `kotlin { android { } }`
block is a separate target. Only compare values within the same
target scope.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
