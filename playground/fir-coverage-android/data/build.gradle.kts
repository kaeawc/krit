// KSP: Moshi's codegen processor generates JSON adapters.
plugins {
    alias(libs.plugins.kotlin.jvm)
    alias(libs.plugins.ksp)
}

java {
    sourceCompatibility = JavaVersion.VERSION_17
    targetCompatibility = JavaVersion.VERSION_17
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

dependencies {
    api(project(":model"))
    api(project(":proto"))
    api(libs.moshi)
    ksp(libs.moshi.kotlin.codegen)
}
