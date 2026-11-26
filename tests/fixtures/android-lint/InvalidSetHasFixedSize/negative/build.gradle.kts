plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.example.synth"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.example.synth"
        minSdk = 24
        targetSdk = 34
    }
}

dependencies {
    implementation("androidx.recyclerview:recyclerview:1.4.0")
}
