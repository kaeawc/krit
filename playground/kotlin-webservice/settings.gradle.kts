pluginManagement {
    repositories {
        mavenCentral()
        gradlePluginPortal()
    }
    includeBuild("../../krit-gradle-plugin")
    includeBuild("../../krit-custom-rule-plugin")
}

dependencyResolutionManagement {
    repositories {
        mavenCentral()
    }
}

// Composite-build substitution for the Kotlin custom-rule SPI used by
// :custom-rules. With this in place the `dev.jasonpearson.krit:krit-rule-api`
// dependency resolves to the local source build instead of requiring a
// published artifact.
includeBuild("../../tools/krit-rule-api")

rootProject.name = "kotlin-webservice"

include(":custom-rules")
project(":custom-rules").projectDir = file("../custom-rules")
