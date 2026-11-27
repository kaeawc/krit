plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.customsplashscreen"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.example.customsplashscreen"
        minSdk = 24
        targetSdk = 30
    }
}

dependencies {
    implementation("androidx.appcompat:appcompat:1.7.0")
}
