plugins {
    kotlin("jvm") version "2.3.20"
    id("com.gradleup.shadow") version "9.4.1"
}

val kotlinVersion = "2.3.20"

dependencies {
    // Kotlin compiler (non-embeddable, full APIs)
    implementation("org.jetbrains.kotlin:kotlin-compiler:$kotlinVersion")

    // Kotlin Analysis API standalone (from JetBrains intellij-dependencies repo)
    implementation("org.jetbrains.kotlin:analysis-api-standalone-for-ide:$kotlinVersion") { isTransitive = false }
    implementation("org.jetbrains.kotlin:analysis-api-for-ide:$kotlinVersion") { isTransitive = false }
    implementation("org.jetbrains.kotlin:analysis-api-k2-for-ide:$kotlinVersion") { isTransitive = false }
    implementation("org.jetbrains.kotlin:analysis-api-impl-base-for-ide:$kotlinVersion") { isTransitive = false }
    implementation("org.jetbrains.kotlin:analysis-api-platform-interface-for-ide:$kotlinVersion") { isTransitive = false }
    implementation("org.jetbrains.kotlin:low-level-api-fir-for-ide:$kotlinVersion") { isTransitive = false }
    implementation("org.jetbrains.kotlin:symbol-light-classes-for-ide:$kotlinVersion") { isTransitive = false }

    // Required runtime deps
    implementation("com.github.ben-manes.caffeine:caffeine:3.2.3")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-core:1.11.0")
    runtimeOnly("org.jetbrains.intellij.deps.kotlinx:kotlinx-coroutines-core:1.10.2-intellij-1")
}

kotlin {
    jvmToolchain(21)
}

tasks.shadowJar {
    archiveClassifier.set("")
    mergeServiceFiles() // Required: Analysis API uses ServiceLoader extensively
    manifest {
        attributes(
            "Main-Class" to "dev.krit.types.MainKt",
            "Multi-Release" to "true",
        )
    }
    minimize {
        // Keep deps accessed via reflection/service loading
        exclude(dependency("org.jetbrains.kotlin:kotlin-compiler:.*"))
        exclude(dependency("org.jetbrains.kotlin:analysis-api.*"))
        exclude(dependency("org.jetbrains.kotlin:low-level-api.*"))
        exclude(dependency("org.jetbrains.kotlin:symbol-light-classes.*"))
        exclude(dependency("com.github.ben-manes.caffeine:caffeine:.*"))
        exclude(dependency("org.jetbrains.intellij.deps.kotlinx:kotlinx-coroutines-core:.*"))
        exclude(dependency("org.jetbrains.kotlinx:kotlinx-serialization-core:.*"))
    }
}
