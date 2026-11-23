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
    testImplementation("org.junit.jupiter:junit-jupiter-api:6.1.3")
    testRuntimeOnly("org.junit.jupiter:junit-jupiter-engine:6.1.3")

    // Generator source set only needs kotlin stdlib
    "testGeneratorImplementation"(kotlin("stdlib"))
}

// Tests tagged go-fixtures (FixtureParityTest and a Go-fixture anchor check)
// read files outside this build: the Go rule fixtures and the generated config
// schema. They run in their own task, fixtureParityTest, so that only it
// depends on those files: a Go-only change (most rule PRs touch a fixture or
// the schema) reruns that small task and leaves `test` up to date or cached.
val goFixturesTag = "go-fixtures"

// Shared by `test` and fixtureParityTest.
tasks.withType<Test>().configureEach {
    useJUnitPlatform()
    // See testForks in the root build.
    maxParallelForks = rootProject.extra["testForks"] as Int
    dependsOn(":jar")
    val pluginJarPath = rootProject.tasks.named("jar", Jar::class.java).flatMap { it.archiveFile }
    inputs.file(pluginJarPath)
    // Test data (stubs and diagnostic sources) is read at runtime, not compiled,
    // so declare it as an input: a stub-only edit must rerun the suite instead
    // of reusing an up-to-date or cached result.
    inputs.dir(project.file("src/test/data")).withPropertyName("testData")
    systemProperty("krit.fir.plugin.jar", pluginJarPath.get().asFile.absolutePath)

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

    // The real kotlin-test jar, for tests of rules that match kotlin.test (a stub cannot declare that package).
    val kotlinTestJar = configurations.testRuntimeClasspath.get()
        .firstOrNull { it.name.matches(Regex("kotlin-test-\\d.*\\.jar")) }
    if (kotlinTestJar != null) {
        systemProperty("kotlin.test.jar", kotlinTestJar.absolutePath)
    }
}

tasks.test {
    useJUnitPlatform { excludeTags(goFixturesTag) }
    // CI splits this suite across jobs: -Pkrit.fir.goldens=only runs just the
    // classes generated from src/test/data/diagnostic (*DiagnosticTests), and
    // -Pkrit.fir.goldens=skip runs everything else. Unset runs both.
    val goldenClasses = "dev.jasonpearson.krit.fir.tests.*DiagnosticTests"
    when (val goldens = findProperty("krit.fir.goldens") as String?) {
        null, "" -> {}
        "only" -> filter.includeTestsMatching(goldenClasses)
        "skip" -> filter.excludeTestsMatching(goldenClasses)
        else -> throw GradleException("krit.fir.goldens must be 'only' or 'skip', not '$goldens'")
    }
}

val fixtureParityTest = tasks.register<Test>("fixtureParityTest") {
    description = "FIR fixture parity: built-in FIR rules against the Go rule fixtures and schema."
    group = "verification"
    useJUnitPlatform { includeTags(goFixturesTag) }
    testClassesDirs = sourceSets.test.get().output.classesDirs
    classpath = sourceSets.test.get().runtimeClasspath

    // The tagged tests read <repo>/tests/fixtures/{positive,negative}/<category>/<RuleId>.kt
    // and the schema's list of Go rule IDs from the krit repo root, two levels
    // above the krit-fir build. Declare exactly that shape (category
    // directories are only listed to find those files) so an edit reruns them.
    val repoRoot = rootProject.projectDir.resolve("../..").canonicalFile
    systemProperty("krit.repo.root", repoRoot.absolutePath)
    for (kind in listOf("positive", "negative")) {
        inputs.files(fileTree(repoRoot.resolve("tests/fixtures/$kind")) { include("*/*.kt") })
            .withPathSensitivity(PathSensitivity.RELATIVE)
            .withPropertyName("go${kind.replaceFirstChar { it.uppercaseChar() }}Fixtures")
    }
    inputs.file(repoRoot.resolve("schemas/krit-config.schema.json"))
        .withPathSensitivity(PathSensitivity.NONE)
        .withPropertyName("goRuleSchema")
}

// `./gradlew test` still runs the parity tests, as their own task with their
// own result: a finalizer runs whether `test` passed, failed or came from the
// cache, and adds no inputs to it.
tasks.test {
    finalizedBy(fixtureParityTest)
}
tasks.check {
    dependsOn(fixtureParityTest)
}
