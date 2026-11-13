plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.bom"
    compileSdk = 34
    defaultConfig {
        applicationId = "com.example.bom"
        minSdk = 26
        targetSdk = 34
    }
}

dependencies {
    implementation(platform("com.google.firebase:firebase-bom:32.0.0"))
    testImplementation("junit:junit:4.13.2")
}
