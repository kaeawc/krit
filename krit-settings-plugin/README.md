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
    id("dev.jasonpearson.krit.settings") version "0.5.0"
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
   Kotlin or Android plugin (`org.jetbrains.kotlin.jvm`,
   `org.jetbrains.kotlin.multiplatform`, `com.android.application`, or
   `com.android.library`) — except those you list under `skip(...)`. A plain
   `java` subproject is not auto-applied; apply `dev.jasonpearson.krit` there
   directly.
2. Seeds each subproject's `krit { }` block with the values configured above
   as conventions. Per-subproject overrides remain authoritative — set the
   property again in a subproject's `build.gradle.kts` and it wins.

## Apply it from an init script

To enable krit across builds without editing each `settings.gradle.kts`
(for example in CI), apply the plugin from a Gradle init script. Resolve it
from Maven Central on the init script's own classpath and apply it by class
(applying by plugin id does not work from an init script):

```kotlin
// krit.init.gradle.kts, passed with -I krit.init.gradle.kts
initscript {
    repositories {
        mavenCentral()
    }
    dependencies {
        classpath("dev.jasonpearson.krit:krit-settings-plugin:0.5.0")
    }
}

beforeSettings {
    pluginManager.apply(dev.jasonpearson.krit.settings.KritSettingsPlugin::class.java)
}
```

Applied this way the plugin runs with its defaults (every Kotlin/Android
subproject, `ignoreFailures = false`). To change `config`, `baseline`, or
`skip(...)`, apply the plugin from `settings.gradle.kts` instead (see above).
Pass the init script with `-I` only on the builds that should run krit; an
init script in `~/.gradle/init.d/` applies to every build on the machine,
including ones that already apply the plugin by id.

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
