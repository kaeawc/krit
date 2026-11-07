// Negative fixture: Kotlin 2.4 needs none of these flags, and mentions in comments or
// strings are not arguments.
plugins {
    id("org.jetbrains.kotlin.jvm") version "2.4.20"
}

// freeCompilerArgs.add("-Xwhen-guards") was needed before Kotlin 2.2.0.
val note = "-Xcontext-parameters is stable in 2.4.0"

kotlin {
    compilerOptions {
        freeCompilerArgs.add("-opt-in=kotlin.RequiresOptIn")
        freeCompilerArgs.add("-Xannotation-default-target=first-only")
    }
}
