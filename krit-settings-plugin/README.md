# Krit Settings Plugin

Apply [krit](https://github.com/kaeawc/krit) static analysis to every JVM
subproject in a multi-module Gradle build from a single root configuration.

## Use it

```kotlin
// settings.gradle.kts
pluginManagement {
    repositories {
        mavenCentral()
        gradlePluginPortal()
    }
}

plugins {
    id("dev.jasonpearson.krit.settings") version "0.2.0"
}

krit {
    config = file("krit.yml")              // shared krit.yml at repo root
    baseline = file("krit-baseline.xml")   // optional
    ignoreFailures = false                 // gate the build on findings

    // Don't auto-apply krit to rule-producer or generated-code modules.
    skip(":rule-bundle", ":generated-stubs")
}
```

The plugin then:

1. Auto-applies `dev.jasonpearson.krit` to every subproject that brings in a
   JVM language plugin (`org.jetbrains.kotlin.jvm`, `org.jetbrains.kotlin.android`,
   `com.android.application`, `com.android.library`, or `java`) — except those
   you list under `skip(...)`.
2. Seeds each subproject's `krit { }` block with the values configured above
   as conventions. Per-subproject overrides remain authoritative — set the
   property again in a subproject's `build.gradle.kts` and it wins.

## Per-subproject overrides

A subproject can still customize anything without giving up the inherited
defaults:

```kotlin
// libs/auth/build.gradle.kts
krit {
    baseline = file("custom-auth-baseline.xml")   // overrides the root value
    advanced {
        parallel = 8                              // root inherited everything else
    }
}
```

## Configuration cache & Project Isolation

The auto-application hook uses `gradle.lifecycle.beforeProject` — the
Project-Isolation-safe replacement for `subprojects { }` and
`gradle.beforeProject`. The plugin works under
`--configuration-cache --no-configuration-cache-problems`. Project
Isolation (incubating) is also supported because the hook reads the
settings extension via simple value capture inside each project's
configuration phase.

## What it doesn't do

The settings plugin only owns auto-application and root-level config
propagation. Per-project task surface, report formats, custom-rule wiring,
and escape-hatch settings all live on the underlying project plugin — see
[krit-gradle-plugin](../krit-gradle-plugin/README.md) for the full task
list (`kritCheck`, `kritFormat`, `kritBaseline`, per-source-set / per-
variant tasks) and the `advanced { }` block.

## Requirements

- Gradle 8.5+ (for `gradle.lifecycle.beforeProject`)
- JDK 21+
