plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.enqueuework"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.example.enqueuework"
        minSdk = 24
        targetSdk = 35
    }
}

dependencies {
    implementation("androidx.work:work-runtime-ktx:2.10.0")
}
