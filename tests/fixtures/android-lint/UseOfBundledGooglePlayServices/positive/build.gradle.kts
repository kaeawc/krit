plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.gms"
    compileSdk = 34
    defaultConfig {
        applicationId = "com.example.gms"
        minSdk = 26
        targetSdk = 34
    }
}

dependencies {
    implementation("com.google.android.gms:play-services:12.0.1")
}
