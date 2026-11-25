plugins {
    id("com.android.application")
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
    implementation("androidx.appcompat:appcompat:1.8.0")
}
