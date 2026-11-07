// Positive fixture: kapt plugin and kapt configurations. Room has a KSP variant.
plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.kapt")
}

dependencies {
    kapt("androidx.room:room-compiler:2.8.0")
}
