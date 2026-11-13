plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.bom"
    compileSdk = 34
    defaultConfig {
        applicationId = "com.example.bom"
        minSdk = 26
        targetSdk = 34
    }
}

dependencies {
    implementation("com.google.firebase:firebase-bom:32.0.0")
}
