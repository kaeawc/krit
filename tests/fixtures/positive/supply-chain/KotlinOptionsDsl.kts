// Positive fixture: the kotlinOptions DSL is an error since Kotlin 2.2.0.
// KotlinOptionsDsl reports each kotlinOptions use and fixes the task block.
import org.jetbrains.kotlin.gradle.tasks.KotlinCompile

plugins {
    id("org.jetbrains.kotlin.jvm") version "2.2.20"
}

tasks.withType<KotlinCompile>().configureEach {
    kotlinOptions {
        jvmTarget = "17"
        freeCompilerArgs += listOf("-opt-in=kotlin.RequiresOptIn")
    }
}

android {
    kotlinOptions {
        jvmTarget = "17"
    }
}
