// Positive fixture: -Xjvm-default is deprecated since Kotlin 2.2.0 in favor of -jvm-default.
plugins {
    id("org.jetbrains.kotlin.jvm") version "2.2.20"
}

kotlin {
    compilerOptions {
        freeCompilerArgs.add("-Xjvm-default=all")
    }
}
