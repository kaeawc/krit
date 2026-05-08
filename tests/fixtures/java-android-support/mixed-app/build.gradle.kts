plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.example.mixed"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.example.mixed"
        minSdk = 16
        targetSdk = 16
    }
}

dependencies {
    implementation("androidx.annotation:annotation:1.8.2")
}
