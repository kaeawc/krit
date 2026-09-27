# Krit Gradle Plugin

Gradle plugin that integrates the [krit](https://github.com/kaeawc/krit) Kotlin static analysis tool into your build. Krit is a Go binary that parses Kotlin with tree-sitter and runs its built-in rule set via single-pass AST dispatch, outputting JSON/SARIF/Checkstyle.

> **Multi-module repos:** for a single root config that covers every Kotlin /
> Java / Android subproject, use the [settings plugin](../krit-settings-plugin/README.md)
> instead of applying this one per module.

## Setup

Add the plugin to your `build.gradle.kts`:

```kotlin
plugins {
    id("dev.jasonpearson.krit") version "0.2.0"
}
```

That's it for most projects — drop a `krit.yml` at the repo root (or next
to the build file) and `./gradlew kritCheck` works. The CLI walks upward
from each analyzed directory looking for `krit.yml` / `.krit.yml`,
stopping at the worktree boundary (`.git` / `.hg` / `.jj`), so one config
at the root covers every subproject without per-module wiring.

### Common configuration

```kotlin
krit {
    config = file("krit.yml")               // optional; explicit override of walk-up discovery
    baseline = file("krit-baseline.xml")    // optional
    ignoreFailures = false                  // fail the build on findings

    reports {
        sarif.required = true               // default
        json.required = true
    }
}
```

### Custom rules

Wire a sibling rule project through the standard `dependencies` block:

```kotlin
dependencies {
    kritCustomRules(project(":my-rules"))
}
```

See [`krit-custom-rule-plugin`](../krit-custom-rule-plugin/README.md) for
authoring the rule module itself.

### Advanced / escape hatches

For overrides most projects never need:

```kotlin
krit {
    advanced {
        toolVersion = "0.2.0"                       // pin the binary version (defaults to the plugin version)
        binary = file("/usr/local/bin/krit")        // skip the download entirely
        parallel = 4                                // -j
        noCache = false                             // --no-cache
        typeInference = true
        allRules = false
        fixLevel = "idiomatic"                      // cosmetic | idiomatic | semantic
        source.setFrom("src/main/kotlin")           // override auto-detection
        reportsDir = layout.buildDirectory.dir("reports/krit")
    }
}
```

### Reports

Four formats are supported: SARIF (the default), JSON, plain text, and Checkstyle. Each can be toggled independently, and output locations can be customized:

```kotlin
krit {
    reports {
        sarif.required = true                                   // default
        json.required = true
        checkstyle {
            required = true
            outputLocation = file("build/reports/checkstyle/krit.xml")
        }
    }
}
```

Reports are written to `build/reports/krit/` unless overridden via `advanced.reportsDir`.

### Per-Source-Set Tasks

When the Kotlin JVM plugin or Android Gradle Plugin is applied, krit automatically registers per-source-set or per-variant tasks:

**Kotlin JVM projects:**
```bash
./gradlew kritCheckMain   # Analyze src/main/kotlin
./gradlew kritCheckTest   # Analyze src/test/kotlin
./gradlew kritCheck       # Analyze all sources
```

**Android projects:**
```bash
./gradlew kritCheckDebug    # Analyze debug variant sources
./gradlew kritCheckRelease  # Analyze release variant sources
./gradlew kritCheck         # Analyze all sources
```

## Tasks

### `kritExportModel`

Exports resolved compile classpaths for krit. Each applied JVM or Android project
registers one `kritExportModel` task and writes one file under the build root's
`.krit/gradle-model/` directory. The root project's file is `_root.json`;
`:feature:login` writes `feature__login.json`. Running `gradle kritExportModel`
selects the task in every project where it exists through ordinary Gradle task-name
selection. No separate aggregate step is needed. Each file contains that project's
entry, with external artifacts and project dependencies recorded separately. JVM
exports use the `main` source set; Android exports use `debug` or the alphabetically
first available production variant if it is absent.

```kotlin
krit {
    exportModel = true       // default: this project's kritCheck depends on its kritExportModel
    fir = true               // default: run the FIR checker pass during kritCheck
    androidVariant = "debug" // default; choose another Android variant if needed
}
```

Set `exportModel = false` to run `kritCheck` without generating the model.
The task can still be run explicitly.
Set `fir = false` to pass `--no-fir` to `kritCheck` when FIR is unavailable.

### `kritCheck`

Runs krit analysis on all configured Kotlin sources. Produces a SARIF report at `build/reports/krit/krit.sarif`. Wired into the `check` lifecycle automatically.

```bash
./gradlew kritCheck
```

### `kritFormat`

Applies krit auto-fixes to Kotlin sources. The fix level is controlled by the `fixLevel` extension property.

```bash
./gradlew kritFormat
```

### `kritBaseline`

Creates a baseline file that captures all current findings. Subsequent `kritCheck` runs with the baseline configured will only report new issues.

```bash
./gradlew kritBaseline
```

Then reference the baseline in your configuration:

```kotlin
krit {
    baseline = file("build/reports/krit/baseline.xml")
}
```

## Binary Resolution

The plugin downloads the correct platform-specific krit binary from GitHub Releases and caches it in `~/.gradle/krit/`. Supported platforms: `darwin-arm64`, `darwin-amd64`, `linux-arm64`, `linux-amd64`, `windows-amd64`. To skip the download and use a local binary, set `advanced.binary` (see [Advanced / escape hatches](#advanced--escape-hatches) above).

## Requirements

- Gradle 8.0+
- JDK 17+ for the plugin's JVM 17 bytecode. Default FIR-enabled `kritCheck` scans need Java 21+ at runtime; set `fir = false` in the `krit` extension for Go-only analysis on JDK 17.
