plugins {
    id("com.android.application") version "8.5.0"
    kotlin("kapt")
    id("com.google.devtools.ksp") version "2.0.0-1.0.24"
}

dependencies {
    // Processors outside Lint's KSP table, other configurations, and KSP itself.
    kapt("com.google.dagger:dagger-compiler:2.50")
    kaptTest("androidx.room:room-compiler:2.6.1")
    ksp("androidx.room:room-compiler:2.6.1")
    // kapt("androidx.room:room-compiler:2.6.1")
}
