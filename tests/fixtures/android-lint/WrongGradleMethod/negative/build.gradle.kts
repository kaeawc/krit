plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.wronggradlemethod"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.example.wronggradlemethod"
        minSdk = 24
        targetSdk = 35
    }
}

dependencies {
    debugImplementation("androidx.annotation:annotation:1.9.1")
}
