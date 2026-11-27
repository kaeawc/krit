plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.annotateversioncheck"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.example.annotateversioncheck"
        minSdk = 21
        targetSdk = 35
    }
}

dependencies {
    implementation("androidx.annotation:annotation:1.9.1")
}
