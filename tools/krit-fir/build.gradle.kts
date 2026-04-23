plugins {
    kotlin("jvm") version "2.3.20"
}

val kotlinVersion = "2.3.20-206"
extra["kotlinVersion"] = kotlinVersion

kotlin {
    jvmToolchain(21)
    compilerOptions {
        freeCompilerArgs.add("-Xcontext-parameters")
    }
}

dependencies {
    // Kotlin compiler (non-embeddable) — provides FIR checker API and plugin infra
    compileOnly("org.jetbrains.kotlin:kotlin-compiler:$kotlinVersion")
}

tasks.test {
    useJUnitPlatform()
}
