// Wire generates Kotlin message classes from src/main/proto.
plugins {
    alias(libs.plugins.kotlin.jvm)
    alias(libs.plugins.wire)
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

wire {
    kotlin {}
}
