# Android Lint — Gradle Sub-Cluster

Rules that analyze Gradle build scripts (`build.gradle`, `build.gradle.kts`). Implemented via the `GradleRule` interface in the Gradle pipeline.

**Status: 19 shipped (13 canonical IDs + 6 AOSP aliases), 3 planned**

---

## Shipped Rules

Some rules are registered under multiple IDs for AOSP compatibility. Canonical IDs are listed first with aliases noted.

| Rule ID | AOSP Alias | Brief |
|---|---|---|
| GradleCompatibility | GradleCompatible | AGP version incompatible with Gradle version |
| StringInteger | StringShouldBeInt | String value where integer expected in Gradle DSL |
| RemoteVersion | — | Non-deterministic dependency version (+ or latest) |
| DynamicVersion | GradleDynamicVersion | Dynamic dependency version with partial wildcard |
| OldTargetApi | — | targetSdkVersion below recommended minimum |
| DeprecatedDependency | — | Deprecated library dependency |
| MavenLocal | — | mavenLocal() causes unreproducible builds |
| MinSdkTooLow | — | minSdkVersion below recommended minimum |
| GradleDeprecated | — | Deprecated Gradle construct |
| GradleGetter | — | Gradle implicit getter call |
| GradlePath | — | Gradle path issues |
| GradleOverrides | — | Value overridden by Gradle build script |
| GradleIdeError | — | Gradle IDE support issues |
| AndroidGradlePluginVersion | — | AGP version too old |
| NewerVersionAvailable | GradleDependency | Newer library version available |

---

## Planned Rules (3)

These three require version-database lookups or network access and are low priority.

| Rule ID | AOSP Detector | Description | Blocker |
|---|---|---|---|
| GradlePluginCompatibility | GradleDetector.GRADLE_PLUGIN_COMPATIBILITY | Flags plugin version incompatibilities beyond AGP (e.g., Kotlin Gradle plugin vs Kotlin stdlib) | Requires a version compatibility matrix |
| StringInteger (extended) | GradleDetector.STRING_INTEGER | Catches additional string-as-integer patterns in Groovy DSL that the current regex misses | Requires full Groovy/KTS parser |
| RemoteVersion (full) | GradleDetector.REMOTE_VERSION | Flags `latest.release` and Maven range syntax in addition to `+` wildcards | Low value; current rule covers common cases |

These are explicitly deprioritized. The existing `RemoteVersion` and `DynamicVersion` rules cover the highest-impact patterns without requiring external data.
