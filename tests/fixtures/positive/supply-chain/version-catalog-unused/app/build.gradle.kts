plugins {
    alias(libs.plugins.android.application)
}

dependencies {
    implementation(libs.bundles.network)
    implementation(libs.okhttp)
}
