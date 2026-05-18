rootProject.name = "krit-settings-plugin"

// Two `includeBuild(...)` calls deliberately: the one inside `pluginManagement`
// exposes `dev.jasonpearson.krit` for the functional test's TestKit consumer
// to apply via `plugins { ... }`. The one at the top level participates in
// the main composite for dependency substitution, so this build's
// `implementation("dev.jasonpearson.krit:krit-gradle-plugin")` resolves
// against the local source. Modern Gradle does not promote pluginManagement
// composites into the main resolution graph automatically.

pluginManagement {
    repositories {
        mavenCentral()
        gradlePluginPortal()
    }
    includeBuild("../krit-gradle-plugin")
}

dependencyResolutionManagement {
    repositories {
        mavenCentral()
    }
}

includeBuild("../krit-gradle-plugin")
