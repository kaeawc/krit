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
    implementation("commons-logging:commons-logging:1.2")
}
