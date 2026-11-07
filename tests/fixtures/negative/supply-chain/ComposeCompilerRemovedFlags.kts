// Negative fixture: stabilityConfigurationFiles replaces the removed singular option, and
// comments do not configure the plugin.
plugins {
    id("org.jetbrains.kotlin.plugin.compose") version "2.4.20"
}

// enableStrongSkippingMode = true was removed with the 2.4.0 option cleanup.
composeCompiler {
    stabilityConfigurationFiles.add(rootProject.layout.projectDirectory.file("stability.conf"))
    includeSourceInformation = true
}
