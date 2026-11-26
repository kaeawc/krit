plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.wearableactionduplicate"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.example.wearableactionduplicate"
        minSdk = 26
        targetSdk = 35
    }
}

dependencies {
    implementation("androidx.wear.watchface:watchface:1.2.1")
}
