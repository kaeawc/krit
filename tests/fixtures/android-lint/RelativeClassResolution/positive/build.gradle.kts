plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.relativeclassresolution"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.example.relativeclassresolution"
        minSdk = 24
        targetSdk = 35
    }
}

dependencies {
    implementation("androidx.coordinatorlayout:coordinatorlayout:1.3.0")
    implementation("androidx.recyclerview:recyclerview:1.4.0")
}
