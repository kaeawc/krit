plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.vectordrawablecompat"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.example.vectordrawablecompat"
        minSdk = 19
        targetSdk = 35
        // vectorDrawables.useSupportLibrary = true
    }
}

dependencies {
    implementation("androidx.appcompat:appcompat:1.6.1")
}
