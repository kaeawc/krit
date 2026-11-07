// Negative fixture: jvmDefault replaces the deprecated -Xjvm-default flag.
plugins {
    id("org.jetbrains.kotlin.jvm") version "2.2.20"
}

// The old freeCompilerArgs.add("-Xjvm-default=all") is gone.
kotlin {
    compilerOptions {
        jvmDefault.set(org.jetbrains.kotlin.gradle.dsl.JvmDefaultMode.NO_COMPATIBILITY)
    }
}
