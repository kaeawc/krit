plugins {
    alias(libs.plugins.android.application)
}

dependencies {
    implementation(libs.okhttp)
    implementation(libs.kotlin.stdlib)
    implementation(libs.bundles.network)
}
