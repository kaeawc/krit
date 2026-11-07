// Negative fixture: KSP replaces kapt, a classpath-only declaration does not apply the
// plugin, and comments do not declare configurations.
plugins {
    id("com.android.application")
    id("com.google.devtools.ksp")
    id("org.jetbrains.kotlin.kapt") version "2.4.20" apply false
}

dependencies {
    ksp("androidx.room:room-compiler:2.8.0")
    // kapt("androidx.room:room-compiler:2.8.0")
}
