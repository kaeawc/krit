package fixtures.androidlint

import androidx.webkit.Profile

fun allow(profile: Profile) {
    profile.setCrossOriginIsolatedAllowlist(setOf("https://example.com"))
}
