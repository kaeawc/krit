// Android application with view binding. AGP 9 compiles Kotlin itself
// (built-in Kotlin), so org.jetbrains.kotlin.android is not applied.
plugins {
    alias(libs.plugins.android.application)
}

android {
    namespace = "dev.krit.firsample.app"
    compileSdk = 36

    defaultConfig {
        applicationId = "dev.krit.firsample.app"
        minSdk = 26
        targetSdk = 36
        versionCode = 1
        versionName = "1.0"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    buildFeatures {
        viewBinding = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

dependencies {
    implementation(project(":data"))

    // src/test and src/androidTest use R (#1724).
    testImplementation(libs.junit)
    androidTestImplementation(libs.androidx.test.ext.junit)
    androidTestImplementation(libs.androidx.test.runner)
}
