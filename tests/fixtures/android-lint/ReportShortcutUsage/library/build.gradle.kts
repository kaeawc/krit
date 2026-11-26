plugins {
    id("com.android.library")
}

android {
    namespace = "com.example.reportshortcutusage"
    compileSdk = 36

    defaultConfig {
        minSdk = 25
    }
}

dependencies {
    implementation("androidx.core:core:1.13.1")
}
