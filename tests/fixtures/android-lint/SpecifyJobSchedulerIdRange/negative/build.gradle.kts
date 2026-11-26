plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.example.synth"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.example.synth"
        minSdk = 23
        targetSdk = 35
    }
}

dependencies {
    implementation("androidx.work:work-runtime-ktx:2.12.0")
}
