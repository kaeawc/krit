plugins {
    id("com.android.application") version "8.5.0"
}

android {
    namespace = "com.example.wear"
    compileSdk = 34
}

dependencies {
    implementation("com.google.android.gms:play-services-wearable:18.1.0")
}
