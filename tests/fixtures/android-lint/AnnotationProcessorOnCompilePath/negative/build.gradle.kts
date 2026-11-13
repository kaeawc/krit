plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.processor"
    compileSdk = 34
    defaultConfig {
        applicationId = "com.example.processor"
        minSdk = 26
        targetSdk = 34
    }
}

dependencies {
    kapt("com.google.dagger:dagger-compiler:2.50")
    annotationProcessor("com.google.dagger:dagger-compiler:2.50")
    implementation("com.google.dagger:dagger:2.50")
}
