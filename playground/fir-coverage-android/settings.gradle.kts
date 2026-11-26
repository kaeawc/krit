// Dual-build (Gradle and Bazel) Android sample that krit's FIR coverage CI
// (.github/workflows/fir-coverage.yml) analyzes. See README.md.
pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
    // krit's Gradle plugins come from this checkout, not a release.
    includeBuild("../../krit-gradle-plugin")
    includeBuild("../../krit-settings-plugin")
}

plugins {
    id("dev.jasonpearson.krit.settings")
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "fir-coverage-android"

include(":model", ":proto", ":data", ":app", ":benchmark")

krit {
    // The sample measures coverage; findings must not fail the build.
    ignoreFailures = true
}

// CI passes -PkritBinary=<krit built from this checkout> so the plugin does
// not download a release. Generators run during kritExportModel, so FIR
// sees view binding, KSP and Wire sources on an unbuilt checkout.
gradle.lifecycle.beforeProject {
    pluginManager.withPlugin("dev.jasonpearson.krit") {
        extensions.configure<dev.jasonpearson.krit.gradle.KritExtension> {
            exportGenerated.set(true)
            providers.gradleProperty("kritBinary").orNull?.let { advanced.binary.set(file(it)) }
        }
    }
}
