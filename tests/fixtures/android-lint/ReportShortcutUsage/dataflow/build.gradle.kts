plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.reportshortcutusage"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.example.reportshortcutusage"
        minSdk = 25
        targetSdk = 35
    }
}

dependencies {
    implementation("androidx.core:core:1.13.1")
    implementation("androidx.core:core-ktx:1.13.1")
}
