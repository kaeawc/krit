plugins {
    id("com.android.application")
}

dependencies {
    // Should flag: catalog has okhttp.
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    // Should flag: catalog has coreKtx with split group/name fields.
    implementation("androidx.core:core-ktx:1.12.0")
    // Should flag: catalog has gson via shorthand string form.
    api("com.google.code.gson:gson:2.10.1")
}
