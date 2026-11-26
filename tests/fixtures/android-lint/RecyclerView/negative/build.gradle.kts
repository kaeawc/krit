plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.recyclerview"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.example.recyclerview"
        minSdk = 24
        targetSdk = 35
    }
}

dependencies {
    implementation("androidx.recyclerview:recyclerview:1.4.0")
}
