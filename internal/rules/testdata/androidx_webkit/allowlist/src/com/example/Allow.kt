package com.example

import androidx.webkit.Profile

class FakeProfile {
    fun setCrossOriginIsolatedAllowlist(origins: Set<String>) = Unit
    var crossOriginIsolatedAllowlist: Set<String> = emptySet()
}

fun allow(profile: Profile, fake: FakeProfile) {
    profile.setCrossOriginIsolatedAllowlist(setOf("https://example.com"))
    profile.crossOriginIsolatedAllowlist = setOf("https://a.com")
    fake.setCrossOriginIsolatedAllowlist(emptySet())
    with(profile) { setCrossOriginIsolatedAllowlist(emptySet()) }
}
