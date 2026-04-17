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

    dynamicFeatures += setOf(":feature:maps")
}

dependencies {
    implementation("org.jetbrains.kotlin:kotlin-stdlib:2.0.0")
}
