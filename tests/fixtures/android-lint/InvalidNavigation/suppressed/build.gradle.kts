plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.invalidnavigation"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.example.invalidnavigation"
        minSdk = 24
        targetSdk = 35
    }
}

dependencies {
    implementation("androidx.navigation:navigation-fragment:2.9.6")
}
