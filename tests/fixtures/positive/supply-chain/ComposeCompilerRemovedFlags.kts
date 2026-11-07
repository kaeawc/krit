// Positive fixture: these Compose compiler Gradle options are errors since Kotlin 2.4.0.
// Strong skipping and intrinsic remember are on by default.
plugins {
    id("org.jetbrains.kotlin.plugin.compose") version "2.4.20"
}

composeCompiler {
    enableStrongSkippingMode = true
    stabilityConfigurationFile = rootProject.layout.projectDirectory.file("stability.conf")
    featureFlags = setOf(ComposeFeatureFlag.IntrinsicRemember)
}
