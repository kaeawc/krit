# Krit Gradle Plugin

Gradle plugin that integrates the [krit](https://github.com/kaeawc/krit) Kotlin static analysis tool into your build. Krit is a Go binary that parses Kotlin with tree-sitter and runs 472 rules via single-pass AST dispatch, outputting JSON/SARIF/Checkstyle.

## Setup

Add the plugin to your `build.gradle.kts`:

```kotlin
plugins {
    id("dev.krit") version "0.2.0"
}

krit {
    // Krit binary version (downloaded automatically from GitHub Releases)
    toolVersion.set("0.2.0")

    // Path to krit.yml configuration file
    config.set(file("krit.yml"))

    // Source directories to analyze (defaults to src/main/kotlin + src/test/kotlin)
    source.setFrom("src/main/kotlin", "src/test/kotlin")

    // Fail the build on findings (default: false)
    ignoreFailures.set(false)

    // Enable all rules including opt-in ones
    allRules.set(false)

    // Auto-fix level: "cosmetic", "idiomatic" (default), or "semantic"
    fixLevel.set("idiomatic")

    // Baseline file for suppressing known issues
    baseline.set(file("krit-baseline.xml"))

    // Number of parallel analysis jobs (default: CPU count)
    parallel.set(Runtime.getRuntime().availableProcessors())

    // Use a local krit binary instead of downloading
    // binary.set(file("/usr/local/bin/krit"))

    // Configure report outputs
    reports {
        sarif {
            required.set(true)  // enabled by default
            outputLocation.set(file("build/reports/krit/krit.sarif"))
        }
        json {
            required.set(true)
            outputLocation.set(file("build/reports/krit/krit.json"))
        }
        plain {
            required.set(false)  // disabled by default
        }
        checkstyle {
            required.set(false)  // disabled by default
        }
    }
}
```

### Reports

The plugin supports four report formats: SARIF (default), JSON, plain text, and Checkstyle. Each format can be independently enabled or disabled, and output locations can be customized.

By default, only the SARIF report is enabled. Reports are written to `build/reports/krit/` unless overridden.

```kotlin
krit {
    reports {
        // Enable multiple formats
        sarif { required.set(true) }
        json { required.set(true) }
        checkstyle {
            required.set(true)
            outputLocation.set(file("build/reports/checkstyle/krit.xml"))
        }
    }
}
```

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
    baseline.set(file("build/reports/krit/baseline.xml"))
}
```

## Binary Resolution

The plugin automatically downloads the correct platform-specific krit binary from GitHub Releases and caches it in `~/.gradle/krit/`. Supported platforms:

- `darwin-arm64` (macOS Apple Silicon)
- `darwin-amd64` (macOS Intel)
- `linux-arm64`
- `linux-amd64`
- `windows-amd64`

To use a locally installed binary instead:

```kotlin
krit {
    binary.set(file("/usr/local/bin/krit"))
}
```

## Requirements

- Gradle 8.0+
- JDK 11+
