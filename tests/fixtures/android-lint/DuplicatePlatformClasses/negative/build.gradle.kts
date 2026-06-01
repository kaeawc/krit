plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.platformdup"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.example.platformdup"
        minSdk = 24
        targetSdk = 34
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.12.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
}
