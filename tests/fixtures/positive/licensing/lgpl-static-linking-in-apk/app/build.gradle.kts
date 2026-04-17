plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.example.app"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.example.app"
        minSdk = 24
        targetSdk = 34
    }
}

dependencies {
    // Intended future embedded-registry match: LGPL-2.1-only
    implementation("fixture.registry:lgpl21-only-lib:1.0.0")
    implementation("org.jetbrains.kotlin:kotlin-stdlib:2.0.0")
}
