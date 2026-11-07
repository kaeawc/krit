// Negative fixture: with AGP 9.0 the Kotlin Android plugin is not applied. A root script
// may still declare it with apply false, and comments do not apply plugins.
plugins {
    id("com.android.application") version "9.0.0"
    id("org.jetbrains.kotlin.android") version "2.4.0" apply false
}

// id("org.jetbrains.kotlin.android") was removed when moving to built-in Kotlin.
android {
    namespace = "com.example.app"
    compileSdk = 36
}
