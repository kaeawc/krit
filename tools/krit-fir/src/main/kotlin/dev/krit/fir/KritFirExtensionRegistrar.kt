package dev.krit.fir

import org.jetbrains.kotlin.fir.extensions.FirExtensionRegistrar

class KritFirExtensionRegistrar : FirExtensionRegistrar() {
    override fun ExtensionRegistrarContext.configurePlugin() {
        +::KritFirCheckers
    }
}
