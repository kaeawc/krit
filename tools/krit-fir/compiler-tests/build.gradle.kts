val kotlinVersion: String by rootProject.extra

plugins {
    kotlin("jvm")
}

kotlin {
    jvmToolchain(21)
    compilerOptions {
        freeCompilerArgs.add("-Xcontext-parameters")
    }
}

val testGenerator by sourceSets.creating {
    compileClasspath += configurations["compileClasspath"]
    runtimeClasspath += configurations["runtimeClasspath"]
}

val generateTests by tasks.registering(JavaExec::class) {
    description = "Regenerates JUnit test methods from src/test/data/diagnostic/ files"
    mainClass.set("dev.jasonpearson.krit.fir.tests.GenerateTestsKt")
    classpath = sourceSets["testGenerator"].runtimeClasspath
    val dataDir = project.file("src/test/data/diagnostic")
    val outDir = layout.buildDirectory.dir("generated/source/tests").get().asFile
    args(dataDir.absolutePath, outDir.absolutePath)
    inputs.dir(dataDir)
    outputs.dir(outDir)
}

sourceSets["test"].kotlin.srcDir(layout.buildDirectory.dir("generated/source/tests"))

tasks.compileTestKotlin {
    dependsOn(generateTests)
}

dependencies {
    // Main plugin module — provides KritPluginRegistrar and checkers
    testImplementation(project(":"))

    // Kotlin compiler for running FIR analysis in tests
    testImplementation("org.jetbrains.kotlin:kotlin-compiler:$kotlinVersion")

    testImplementation(kotlin("test-junit5"))
    testImplementation("org.junit.jupiter:junit-jupiter-api:6.0.3")
    testRuntimeOnly("org.junit.jupiter:junit-jupiter-engine:6.0.3")

    // Generator source set only needs kotlin stdlib
    "testGeneratorImplementation"(kotlin("stdlib"))
}

tasks.test {
    useJUnitPlatform()
    dependsOn(":jar")
    val pluginJarPath = rootProject.layout.buildDirectory.file("libs/krit-fir.jar")
    inputs.file(pluginJarPath)
    // Test data (stubs and diagnostic sources) is read at runtime, not compiled,
    // so declare it as an input: a stub-only edit must rerun the suite instead
    // of reusing an up-to-date or cached result.
    inputs.dir(project.file("src/test/data")).withPropertyName("testData")
    systemProperty("krit.fir.plugin.jar", pluginJarPath.get().asFile.absolutePath)

    // FixtureParityTest reads the Go rule fixtures and the generated config
    // schema (its list of Go rule IDs) from the krit repo root, two levels
    // above the krit-fir build. Declare them as inputs so an edit reruns it.
    val repoRoot = rootProject.projectDir.resolve("../..").canonicalFile
    systemProperty("krit.repo.root", repoRoot.absolutePath)
    inputs.dir(repoRoot.resolve("tests/fixtures/positive")).withPropertyName("goPositiveFixtures")
    inputs.dir(repoRoot.resolve("tests/fixtures/negative")).withPropertyName("goNegativeFixtures")
    inputs.file(repoRoot.resolve("schemas/krit-config.schema.json")).withPropertyName("goRuleSchema")

    // Show why a test failed or was skipped (FixtureParityTest names the
    // unresolved symbols and the offending fixture) and its per-rule summary.
    testLogging {
        events("failed", "skipped")
        exceptionFormat = org.gradle.api.tasks.testing.logging.TestExceptionFormat.FULL
        showStandardStreams = true
    }

    // Pass kotlin-stdlib.jar path so the embedded compiler can resolve built-in declarations.
    val stdlibJar = configurations.testRuntimeClasspath.get()
        .firstOrNull { it.name.matches(Regex("kotlin-stdlib-\\d.*\\.jar")) }
    if (stdlibJar != null) {
        systemProperty("kotlin.stdlib.jar", stdlibJar.absolutePath)
    }
}
