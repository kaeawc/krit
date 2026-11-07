// Negative fixture: compilerOptions replaces kotlinOptions, and mentions of
// kotlinOptions in comments or strings are not code.
import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    id("org.jetbrains.kotlin.jvm") version "2.2.20"
}

// The old kotlinOptions { jvmTarget = "17" } block is gone.
val migrationNote = "kotlinOptions { } became compilerOptions { }"

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_17)
        freeCompilerArgs.add("-opt-in=kotlin.RequiresOptIn")
    }
}
