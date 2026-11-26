// Kotlin compiler plugin (kotlinx-serialization) plus a module-wide opt-in.
plugins {
    alias(libs.plugins.kotlin.jvm)
    alias(libs.plugins.kotlin.serialization)
}

java {
    sourceCompatibility = JavaVersion.VERSION_17
    targetCompatibility = JavaVersion.VERSION_17
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
        optIn.add("dev.krit.firsample.model.ExperimentalSampleApi")
    }
}

dependencies {
    api(libs.kotlinx.serialization.json)
}
