plugins {
    id("com.android.dynamic-feature")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.example.feature.maps"
    compileSdk = 34

    defaultConfig {
        minSdk = 24
    }
}

dependencies {
    implementation(project(":app"))
    // Intended future embedded-registry match: LGPL-2.1-only
    implementation("fixture.registry:lgpl21-only-lib:1.0.0")
}
