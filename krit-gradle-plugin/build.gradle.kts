plugins {
    `java-gradle-plugin`
    `kotlin-dsl`
    `maven-publish`
}

group = "dev.jasonpearson.krit"
// The plugin ships in lockstep with the krit binary: its version is the
// krit release it downloads by default (KritVersion.VERSION below). A
// release build pins it via -PkritVersion or KRIT_VERSION (same as
// krit-rule-api); the fallback is the latest stable release so local
// builds resolve a published binary. Bump it when a new release ships.
version = (findProperty("kritVersion") as String?)
    ?.takeIf { it.isNotBlank() }
    ?: System.getenv("KRIT_VERSION")?.takeIf { it.isNotBlank() }
    ?: "0.2.0"

repositories {
    mavenCentral()
}

gradlePlugin {
    plugins {
        create("krit") {
            id = "dev.jasonpearson.krit"
            implementationClass = "dev.jasonpearson.krit.gradle.KritPlugin"
            displayName = "Krit Kotlin Lint"
            description = "Static analysis for Kotlin using tree-sitter"
        }
    }
}

dependencies {
    testImplementation(gradleTestKit())
    testImplementation("org.junit.jupiter:junit-jupiter:6.1.0")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

// Codegen so the default krit binary version tracks `version` rather than
// drifting from a hand-edited constant in KritPlugin.
val generateKritVersion = tasks.register("generateKritVersion") {
    val versionString = version.toString().removePrefix("v")
    val outputDir = layout.buildDirectory.dir("generated/source/kritVersion/main")
    inputs.property("kritVersion", versionString)
    outputs.dir(outputDir)
    doLast {
        val pkgDir = outputDir.get().asFile.resolve("dev/jasonpearson/krit/gradle")
        pkgDir.mkdirs()
        pkgDir.resolve("KritVersion.kt").writeText(
            """
            |package dev.jasonpearson.krit.gradle
            |
            |/** krit release this plugin build downloads by default; baked in at build time. */
            |internal object KritVersion {
            |    const val VERSION: String = "$versionString"
            |}
            |
            """.trimMargin()
        )
    }
}

sourceSets {
    named("main") {
        kotlin.srcDir(generateKritVersion.map { layout.buildDirectory.dir("generated/source/kritVersion/main") })
    }
}

tasks.test {
    useJUnitPlatform()
}
