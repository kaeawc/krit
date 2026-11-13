plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.kapt")
}

android {
    namespace = "com.example.databinding"
    compileSdk = 34
    defaultConfig {
        applicationId = "com.example.databinding"
        minSdk = 26
        targetSdk = 34
    }
    buildFeatures {
        dataBinding = true
    }
}
