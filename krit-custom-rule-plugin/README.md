# Krit Custom Rule Plugin

Gradle plugin that scaffolds a module for authoring custom [Krit](https://github.com/kaeawc/krit) rules. It wires up the Kotlin compile classpath, generates the `META-INF/services` registration, and produces a properly-stamped jar consumed by Krit's `--custom-rule-jars` flag (or the host plugin's `kritCustomRules` configuration in the `dependencies` block).

## Usage

```kotlin
// build.gradle.kts in your rules module
plugins {
    id("dev.jasonpearson.krit.custom") version "<version>"
}

kritCustomRules {
    // Optional — defaults to the plugin's own version.
    ruleApiVersion = "0.2.0"

    // Optional — written to the Krit-SDK-Version manifest attribute.
    sdkVersion = "0.2.0"

    // Optional — recorded as Krit-Vendor-Id.
    vendorId = "acme"

    // Optional — fallback severity recorded as Krit-Default-Severity.
    defaultSeverity = "warning"
}
```

Write a rule:

```kotlin
package com.example

import dev.jasonpearson.krit.api.Finding
import dev.jasonpearson.krit.api.KritFile
import dev.jasonpearson.krit.api.KritRule
import dev.jasonpearson.krit.api.KritRuleInfo
import dev.jasonpearson.krit.api.RuleContext
import dev.jasonpearson.krit.api.Severity

@KritRuleInfo(id = "acme.NoTodo", category = "custom", severity = Severity.WARNING)
class NoTodoRule : KritRule {
    override fun check(file: KritFile, ctx: RuleContext): List<Finding> {
        // Inspect file.text / file.ktFile and return findings.
        return emptyList()
    }
}
```

Build the jar:

```bash
./gradlew kritRuleJar
```

Consume it from the host project:

```kotlin
// build.gradle.kts in the app/library applying dev.jasonpearson.krit
plugins {
    id("dev.jasonpearson.krit") version "<version>"
}

dependencies {
    kritCustomRules(project(":my-rules"))
}
```

`dev.jasonpearson.krit.custom` publishes the stamped `kritRuleJar` as an
outgoing variant with a `krit-rule-bundle` category attribute, and the
host plugin's `kritCustomRules` configuration resolves that variant
through Gradle's dependency graph — no `evaluationDependsOn`, no
cross-project task lookup, Project-Isolation safe. See
[docs/external-rules.md](../docs/external-rules.md#5-wire-the-jar-into-the-consumer)
for the longer write-up.

For raw jars or task outputs that don't fit the dependency-block model,
append directly to `krit.customRuleJars`:

```kotlin
krit { customRuleJars.from(file("libs/my-rules-0.1.0-krit-rules.jar")) }
```

## What the plugin does

1. Auto-applies `org.jetbrains.kotlin.jvm`.
2. Adds `dev.jasonpearson.krit:krit-rule-api:<ruleApiVersion>` to `implementation`.
3. Registers the `kritCustomRules { }` DSL block (all properties optional).
4. Registers `generateKritRuleServices`, which:
   - Scans compiled classes for `KritRule` implementations and writes
     `META-INF/services/dev.jasonpearson.krit.api.KritRule` automatically.
   - Merges with any manual entries you ship in `src/main/resources`.
   - Fails the build with a pointer to this README if no implementations are
     found, leaving a commented placeholder so the next iteration sees the
     scaffold.
5. Registers `kritRuleJar`, a `Jar` task whose manifest stamps the values
   Krit's daemon reads at load time:
   - `Krit-SDK-Version`
   - `Krit-Plugin-Version`
   - `Krit-Vendor-Id`
   - `Krit-Default-Severity`

## Requirements

- Gradle 8.0+
- JDK 21+
- Kotlin 2.x (auto-applied via the Kotlin JVM plugin)
