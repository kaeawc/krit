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
    mainClass.set("dev.krit.fir.tests.GenerateTestsKt")
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
    testImplementation("org.junit.jupiter:junit-jupiter-api:5.11.4")
    testRuntimeOnly("org.junit.jupiter:junit-jupiter-engine:5.11.4")

    // Generator source set only needs kotlin stdlib
    "testGeneratorImplementation"(kotlin("stdlib"))
}

tasks.test {
    useJUnitPlatform()
    dependsOn(":jar")
    val pluginJarPath = rootProject.layout.buildDirectory.file("libs/krit-fir.jar")
    inputs.file(pluginJarPath)
    systemProperty("krit.fir.plugin.jar", pluginJarPath.get().asFile.absolutePath)

    // Pass kotlin-stdlib.jar path so the embedded compiler can resolve built-in declarations.
    val stdlibJar = configurations.testRuntimeClasspath.get()
        .firstOrNull { it.name.matches(Regex("kotlin-stdlib-\\d.*\\.jar")) }
    if (stdlibJar != null) {
        systemProperty("kotlin.stdlib.jar", stdlibJar.absolutePath)
    }
}
