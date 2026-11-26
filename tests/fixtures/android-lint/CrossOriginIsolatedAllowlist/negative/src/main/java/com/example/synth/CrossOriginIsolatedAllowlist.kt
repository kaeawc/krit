package com.example.synth

class Settings {
    fun setCrossOriginIsolatedAllowlist(origins: Set<String>) = Unit
}

fun allow(settings: Settings) {
    settings.setCrossOriginIsolatedAllowlist(setOf("https://example.com"))
}
