import org.gradle.api.publish.maven.tasks.PublishToMavenRepository

plugins {
    kotlin("jvm") version "2.4.21"
    id("com.gradleup.shadow") version "9.6.1"
    `maven-publish`
    signing
}

group = "dev.jasonpearson.krit"
version = (findProperty("kritVersion") as String?)
    ?.takeIf { it.isNotBlank() }
    ?: System.getenv("KRIT_VERSION")?.takeIf { it.isNotBlank() }
    ?: "0.0.0-SNAPSHOT"

val isSnapshot = version.toString().endsWith("-SNAPSHOT")

val kotlinVersion = "2.4.20"
extra["kotlinVersion"] = kotlinVersion

val bundledKotlinStdlib by configurations.creating {
    isCanBeConsumed = false
    isCanBeResolved = true
}

val bundledStdlibResources = layout.buildDirectory.dir("generated/kotlin-stdlib-resources")
val copyKotlinStdlib by tasks.registering(Copy::class) {
    from(bundledKotlinStdlib)
    into(bundledStdlibResources.map { it.dir("krit") })
    rename { "kotlin-stdlib.jar" }
}

sourceSets.main {
    resources.srcDir(bundledStdlibResources)
}
tasks.processResources {
    dependsOn(copyKotlinStdlib)
}

kotlin {
    jvmToolchain(21)
    compilerOptions {
        freeCompilerArgs.add("-Xcontext-parameters")
    }
}

dependencies {
    add(bundledKotlinStdlib.name, "org.jetbrains.kotlin:kotlin-stdlib:$kotlinVersion") { isTransitive = false }
    // Kotlin compiler bundled into the fat JAR — provides FIR checker API, K2JVMCompiler, and plugin infra
    implementation("org.jetbrains.kotlin:kotlin-compiler:$kotlinVersion")
    // krit-rule-api: KritRule + KritRuleInfo + Capability + RuleApiVersion.
    // The plugin loader inspects ServiceLoader-discovered impls of these
    // types against the daemon's compatible SDK version.
    implementation(project(":krit-rule-api"))

    testImplementation("org.junit.jupiter:junit-jupiter:6.1.3")
    testImplementation(kotlin("test"))
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.shadowJar {
    archiveClassifier.set("")
    // Keep the launcher path stable while project version drives publication coordinates.
    archiveFileName.set("krit-fir.jar")
    // Include Krit's license and the shared Apache-2.0 notices for bundled dependencies.
    from(layout.projectDirectory.file("../../LICENSE")) {
        into("META-INF")
        rename { "LICENSE-krit.txt" }
    }
    from(layout.projectDirectory.file("../THIRD_PARTY_NOTICES.txt")) {
        into("META-INF")
    }
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

val differentialTag = "incremental-differential"
val plannerTag = "incremental-planner"
val benchTag = "incremental-bench"

tasks.test {
    useJUnitPlatform {
        // The split differential compiles thousands of module slices; it runs in
        // its own task (incrementalDifferentialTest) and CI step.
        excludeTags(differentialTag, plannerTag, benchTag)
    }
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
    // Sessions that snapshot a module write under krit's cache dir: keep tests out of the user's.
    systemProperty("krit.fir.cache.dir", layout.buildDirectory.dir("test-krit-cache").get().asFile.absolutePath)
}

// FirIncrementalSplitDifferentialTest: a full-vs-slice compile of the compiler-test
// data, asserting that only tiered rules (FirRule.wholeCompilationReason) differ.
// Tuning: -Pkrit.fir.differential.sample=N (every Nth scenario), -Pkrit.fir.differential.only=Name.
tasks.register<Test>("incrementalDifferentialTest") {
    description = "Split differential: only whole-compilation-tiered rules may differ on a slice compile."
    group = "verification"
    useJUnitPlatform { includeTags(differentialTag) }
    testClassesDirs = sourceSets.test.get().output.classesDirs
    classpath = sourceSets.test.get().runtimeClasspath
    dependsOn("jar")
    val pluginJar = tasks.named("jar", Jar::class.java).flatMap { it.archiveFile }
    inputs.file(pluginJar)
    inputs.dir(project.file("compiler-tests/src/test/data")).withPropertyName("testData")
    systemProperty("krit.fir.plugin.jar", pluginJar.get().asFile.absolutePath)
    systemProperty("krit.fir.cache.dir", layout.buildDirectory.dir("test-krit-cache").get().asFile.absolutePath)
    for (name in listOf("krit.fir.differential.sample", "krit.fir.differential.only")) {
        findProperty(name)?.let { systemProperty(name, it.toString()) }
    }
    maxHeapSize = "2g"
    testLogging {
        events("failed")
        exceptionFormat = org.gradle.api.tasks.testing.logging.TestExceptionFormat.FULL
        showStandardStreams = true
    }
}

// IncrementalPlannerDifferentialTest: about 600 synthetic files and 40 mixed edits; after each one the planned
// check must equal a fresh full check. Tuning: -Pkrit.fir.planner.files=N, -Pkrit.fir.planner.edits=N.
tasks.register<Test>("incrementalPlannerTest") {
    description = "Planner differential: incremental checks equal fresh full checks over a mixed edit run."
    group = "verification"
    useJUnitPlatform { includeTags(plannerTag) }
    testClassesDirs = sourceSets.test.get().output.classesDirs
    classpath = sourceSets.test.get().runtimeClasspath
    dependsOn("jar")
    val pluginJar = tasks.named("jar", Jar::class.java).flatMap { it.archiveFile }
    inputs.file(pluginJar)
    systemProperty("krit.fir.plugin.jar", pluginJar.get().asFile.absolutePath)
    systemProperty("krit.fir.cache.dir", layout.buildDirectory.dir("test-krit-cache").get().asFile.absolutePath)
    for (name in listOf("krit.fir.planner.files", "krit.fir.planner.edits")) {
        findProperty(name)?.let { systemProperty(name, it.toString()) }
    }
    maxHeapSize = "3g"
    testLogging {
        events("failed")
        exceptionFormat = org.gradle.api.tasks.testing.logging.TestExceptionFormat.FULL
        showStandardStreams = true
    }
}

// IncrementalPlannerBench: p50/p95 per edit, planned vs full. Not a test; run by hand under a lock on a shared machine:
// -Pkrit.fir.bench.files=N (synthetic) or -Pkrit.fir.bench.real=dir1,dir2 (real sources), -Pkrit.fir.bench.edits=N,
// -Pkrit.fir.bench.policy=exclude|default.
tasks.register<Test>("incrementalBench") {
    description = "Timing of planned vs full checks (see IncrementalPlannerBench)."
    group = "verification"
    useJUnitPlatform { includeTags(benchTag) }
    testClassesDirs = sourceSets.test.get().output.classesDirs
    classpath = sourceSets.test.get().runtimeClasspath
    dependsOn("jar")
    outputs.upToDateWhen { false }
    val pluginJar = tasks.named("jar", Jar::class.java).flatMap { it.archiveFile }
    inputs.file(pluginJar)
    systemProperty("krit.fir.plugin.jar", pluginJar.get().asFile.absolutePath)
    systemProperty("krit.fir.cache.dir", layout.buildDirectory.dir("test-krit-cache").get().asFile.absolutePath)
    for (name in listOf("files", "real", "edits", "policy", "out", "classpath", "hashes")) {
        findProperty("krit.fir.bench.$name")?.let { systemProperty("krit.fir.bench.$name", it.toString()) }
    }
    maxHeapSize = (findProperty("krit.fir.bench.heap") as String?) ?: "6g"
    testLogging {
        events("failed")
        exceptionFormat = org.gradle.api.tasks.testing.logging.TestExceptionFormat.FULL
        showStandardStreams = true
    }
}

// Publish the self-contained shadow jar as the main artifact; its POM must not
// expose implementation dependencies that consumers cannot resolve centrally.
tasks.register<Jar>("sourcesJar") {
    dependsOn(copyKotlinStdlib)
    archiveClassifier.set("sources")
    from(sourceSets.main.get().allSource)
}

tasks.register<Jar>("javadocJar") {
    archiveClassifier.set("javadoc")
}

publishing {
    publications {
        create<MavenPublication>("maven") {
            from(components["shadow"])
            artifactId = "krit-fir"
            artifact(tasks.named("sourcesJar"))
            artifact(tasks.named("javadocJar"))
            pom {
                name.set("Krit FIR oracle and checkers")
                description.set("Krit FIR oracle and checkers — self-contained JVM helper launched by the krit CLI. The fat jar bundles the Apache-2.0 licensed Kotlin compiler.")
                url.set("https://github.com/kaeawc/krit")
                inceptionYear.set("2026")
                licenses {
                    license {
                        name.set("MIT License")
                        url.set("https://opensource.org/licenses/MIT")
                        distribution.set("repo")
                    }
                    license {
                        name.set("Apache License 2.0 (bundled Kotlin compiler / Analysis API)")
                        url.set("https://www.apache.org/licenses/LICENSE-2.0")
                    }
                }
                developers {
                    developer {
                        id.set("kaeawc")
                        name.set("Jason Pearson")
                        email.set("jason.d.pearson@gmail.com")
                        url.set("https://github.com/kaeawc")
                    }
                }
                scm {
                    connection.set("scm:git:https://github.com/kaeawc/krit.git")
                    developerConnection.set("scm:git:ssh://git@github.com/kaeawc/krit.git")
                    url.set("https://github.com/kaeawc/krit")
                }
                issueManagement {
                    system.set("GitHub")
                    url.set("https://github.com/kaeawc/krit/issues")
                }
            }
        }
    }
    repositories {
        maven {
            name = "stagingDir"
            url = layout.buildDirectory.dir("staging-deploy").get().asFile.toURI()
        }
        maven {
            name = "centralPortal"
            url = if (isSnapshot) uri("https://central.sonatype.com/repository/maven-snapshots/")
                else uri("https://ossrh-staging-api.central.sonatype.com/service/local/staging/deploy/maven2/")
            credentials {
                username = System.getenv("SONATYPE_USERNAME").orEmpty()
                password = System.getenv("SONATYPE_PASSWORD").orEmpty()
            }
        }
    }
}

signing {
    val signingKey = System.getenv("SIGNING_KEY")
    val signingPassword = System.getenv("SIGNING_PASSWORD")
    if (!signingKey.isNullOrBlank() && !signingPassword.isNullOrBlank()) {
        useInMemoryPgpKeys(signingKey, signingPassword)
        sign(publishing.publications["maven"])
    }
}

tasks.withType<Sign>().configureEach {
    onlyIf { !isSnapshot }
}

tasks.withType<PublishToMavenRepository>().configureEach {
    if (repository.name == "centralPortal") {
        doFirst {
            if (!isSnapshot) {
                val missing = listOf("SIGNING_KEY", "SIGNING_PASSWORD")
                    .filter { System.getenv(it).isNullOrBlank() }
                if (missing.isNotEmpty()) {
                    throw GradleException("Cannot publish to centralPortal without ${missing.joinToString(" and ")}")
                }
            }
        }
    }
}
