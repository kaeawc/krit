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
    // krit-rule-api: KritRule + KritRuleInfo + Capability + RuleApiVersion.
    // The plugin loader inspects ServiceLoader-discovered impls of these
    // types against the daemon's compatible SDK version.
    implementation(project(":krit-rule-api"))

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
        // kotlin-stdlib carries runtime support classes (e.g.
        // `NoWhenBranchMatchedException`) that Kotlin codegen emits
        // references to without the minimizer seeing a direct call
        // site. Excluding stdlib from minimization keeps those
        // classes in the shadow jar so the launcher can initialize.
        exclude(dependency("org.jetbrains.kotlin:kotlin-stdlib:.*"))
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
