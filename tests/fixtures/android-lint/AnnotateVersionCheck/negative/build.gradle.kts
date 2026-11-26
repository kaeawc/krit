plugins {
    id("com.android.library")
}

android {
    namespace = "com.example.annotateversioncheck"
    compileSdk = 36

    defaultConfig {
        minSdk = 21
    }
}

dependencies {
    implementation("androidx.annotation:annotation:1.9.1")
}
