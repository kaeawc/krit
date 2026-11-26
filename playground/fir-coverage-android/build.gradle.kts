plugins {
    // Puts KGP 2.4.21 on the build classpath; AGP's built-in Kotlin uses it.
    alias(libs.plugins.kotlin.jvm) apply false
    alias(libs.plugins.kotlin.serialization) apply false
    alias(libs.plugins.android.application) apply false
    alias(libs.plugins.android.test) apply false
    alias(libs.plugins.ksp) apply false
    alias(libs.plugins.wire) apply false
}
