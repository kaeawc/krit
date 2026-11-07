plugins {
    id("org.jetbrains.kotlin.jvm") version "2.2.20"
}

kotlin {
    compilerOptions {
        freeCompilerArgs.addAll("-Xjvm-default=all", "-opt-in=kotlin.RequiresOptIn")
        freeCompilerArgs.add('-Xjvm-default=all-compatibility')
        freeCompilerArgs.add("-Xjvm-default=disable")
    }
}
