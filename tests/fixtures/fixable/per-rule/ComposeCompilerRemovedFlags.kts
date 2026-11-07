plugins {
    id("org.jetbrains.kotlin.plugin.compose") version "2.4.20"
}

composeCompiler {
    enableStrongSkippingMode = true
    enableIntrinsicRemember.set(true)
    includeSourceInformation = true
}
