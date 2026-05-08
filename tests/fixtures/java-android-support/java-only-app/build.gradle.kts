plugins {
    id("com.android.application")
}

android {
    namespace = "com.example.javaonly"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.example.javaonly"
        minSdk = 16
        targetSdk = 16
    }
}

dependencies {
    implementation("androidx.annotation:annotation:1.8.2")
    implementation("androidx.fragment:fragment:1.8.5")
    implementation("androidx.recyclerview:recyclerview:1.3.2")
    implementation("androidx.room:room-runtime:2.6.1")
}
