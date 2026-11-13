plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.processor"
    compileSdk = 34
    defaultConfig {
        applicationId = "com.example.processor"
        minSdk = 26
        targetSdk = 34
    }
}

dependencies {
    implementation("com.google.dagger:dagger-compiler:2.50")
}
