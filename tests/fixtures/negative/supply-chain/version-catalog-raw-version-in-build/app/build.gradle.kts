plugins {
    alias(libs.plugins.android.application)
}

dependencies {
    // Catalog accessor — clean.
    implementation(libs.okhttp)
    implementation(libs.coreKtx)

    // Coordinate not in the catalog — clean.
    implementation("com.example:not-in-catalog:1.0.0")

    /*
     * Block comment with a stale coordinate that must NOT be flagged:
     * implementation("com.squareup.okhttp3:okhttp:4.12.0")
     */
    // Line comment with a coordinate must NOT be flagged: "androidx.core:core-ktx:1.12.0"
}
