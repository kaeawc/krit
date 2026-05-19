plugins {
    kotlin("jvm") version "2.3.21"
    id("com.gradleup.shadow") version "9.4.1"
}

val kotlinVersion = "2.3.21"
extra["kotlinVersion"] = kotlinVersion

kotlin {
    jvmToolchain(21)
    compilerOptions {
        freeCompilerArgs.add("-Xcontext-parameters")
    }
}

dependencies {
    // Kotlin compiler bundled into the fat JAR — provides FIR checker API, K2JVMCompiler, and plugin infra
    implementation("org.jetbrains.kotlin:kotlin-compiler:$kotlinVersion")

    testImplementation("org.junit.jupiter:junit-jupiter:6.0.3")
    testImplementation(kotlin("test"))
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.shadowJar {
    archiveClassifier.set("")
    mergeServiceFiles()
    manifest {
        attributes(
            "Main-Class" to "dev.jasonpearson.krit.fir.MainKt",
            "Multi-Release" to "true",
        )
    }
    minimize {
        exclude(dependency("org.jetbrains.kotlin:kotlin-compiler:.*"))
    }
}

tasks.test {
    useJUnitPlatform()
    // AnalysisSessionAnalyzeTest drives the embedded K2 compiler and
    // needs the krit-fir plugin classes on the plugin classpath. Point
    // it at the plain `:jar` output (the compiler runtime is already
    // on the test classpath via the `kotlin-compiler` dep), so tests
    // don't need to wait for the slower shadow-jar build.
    dependsOn("jar")
    val pluginJar = tasks.named("jar", Jar::class.java).flatMap { it.archiveFile }
    inputs.file(pluginJar)
    systemProperty(
        "krit.fir.plugin.jar",
        pluginJar.get().asFile.absolutePath,
    )
}
