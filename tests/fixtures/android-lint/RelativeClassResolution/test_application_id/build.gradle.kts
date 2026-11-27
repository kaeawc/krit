plugins {
    id("com.android.library")
}

android {
    namespace = "com.example.relativeclassresolution"
    compileSdk = 36

    defaultConfig {
        minSdk = 24
        testApplicationId = "com.example.relativeclassresolution.test"
        /* applicationId = "com.example.commented" */
    }
}

dependencies {
    implementation("androidx.coordinatorlayout:coordinatorlayout:1.3.0")
    implementation("androidx.recyclerview:recyclerview:1.4.0")
}
