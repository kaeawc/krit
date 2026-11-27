plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.returnthis"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.example.returnthis"
        minSdk = 24
        targetSdk = 35
    }
}

dependencies {
    implementation("androidx.annotation:annotation:1.9.1")
}
