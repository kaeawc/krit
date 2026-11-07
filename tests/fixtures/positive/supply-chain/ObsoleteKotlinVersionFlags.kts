// Positive fixture: with Kotlin 2.4 these experimental flags are redundant (guards since
// 2.2.0, context parameters and explicit backing fields since 2.4.0) and context receivers
// are gone. ObsoleteKotlinVersionFlags reports each flag and deletes the redundant ones.
plugins {
    id("org.jetbrains.kotlin.jvm") version "2.4.20"
}

kotlin {
    compilerOptions {
        freeCompilerArgs.add("-Xwhen-guards")
        freeCompilerArgs.addAll("-Xcontext-parameters", "-Xexplicit-backing-fields")
        freeCompilerArgs.add("-Xcontext-receivers")
    }
}
