package com.example.synth

import androidx.webkit.Profile

fun allow(profile: Profile) {
    profile.setCrossOriginIsolatedAllowlist(setOf("https://example.com"))
}
