// Positive fixture: AGP 9.0 has built-in Kotlin, so applying the Kotlin Android plugin
// fails with "The 'org.jetbrains.kotlin.android' plugin is no longer required".
plugins {
    id("com.android.application") version "9.0.0"
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.example.app"
    compileSdk = 36
}
