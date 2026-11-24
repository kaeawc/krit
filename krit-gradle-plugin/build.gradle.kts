import org.gradle.api.publish.maven.tasks.PublishToMavenRepository

plugins {
    `java-gradle-plugin`
    `kotlin-dsl`
    `maven-publish`
    signing
    id("com.gradle.plugin-publish") version "2.2.1"
}

group = "dev.jasonpearson.krit"
// The plugin ships in lockstep with the krit binary: its version is the
// krit release it downloads by default (KritVersion.VERSION below). A
// release build pins it via -PkritVersion or KRIT_VERSION (same as
// krit-rule-api); the fallback is the latest stable release so local
// builds resolve a published binary. Bump it when a new release ships.
version = ((findProperty("kritVersion") as String?)
    ?.takeIf { it.isNotBlank() }
    ?: System.getenv("KRIT_VERSION")?.takeIf { it.isNotBlank() }
    ?: "0.4.0")
    .trim()
    .removePrefix("v")

val isSnapshot = version.toString().endsWith("-SNAPSHOT")

repositories {
    mavenCentral()
}

// com.gradle.plugin-publish adds the sources and javadoc jars Maven Central
// requires, and `publishPlugins` uploads the same publications to the Gradle
// Plugin Portal.
gradlePlugin {
    website = "https://github.com/kaeawc/krit"
    vcsUrl = "https://github.com/kaeawc/krit"
    plugins {
        create("krit") {
            id = "dev.jasonpearson.krit"
            implementationClass = "dev.jasonpearson.krit.gradle.KritPlugin"
            displayName = "Krit Kotlin Lint"
            description = "Static analysis for Kotlin using tree-sitter"
            tags = listOf("kotlin", "android", "lint", "static-analysis", "code-quality")
        }
    }
}

dependencies {
    testImplementation(gradleTestKit())
    testImplementation("org.junit.jupiter:junit-jupiter:6.1.3")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.withType<JavaCompile>().configureEach {
    options.release.set(17)
}

tasks.withType<org.jetbrains.kotlin.gradle.tasks.KotlinCompile>().configureEach {
    compilerOptions.jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
}

// Codegen so the default krit binary version tracks `version` rather than
// drifting from a hand-edited constant in KritPlugin.
val generateKritVersion = tasks.register("generateKritVersion") {
    val versionString = version.toString()
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

// Maven Central: `pluginMaven` is the plugin jar (krit-gradle-plugin), and each
// `<name>PluginMarkerMaven` publication is the `<id>.gradle.plugin` marker that
// resolves `plugins { id("dev.jasonpearson.krit") }` from mavenCentral().
publishing {
    publications.withType<MavenPublication>().configureEach {
        pom {
            if (name.orNull.isNullOrBlank()) name.set("Krit Gradle Plugin")
            if (description.orNull.isNullOrBlank()) {
                description.set("Gradle plugin that runs the krit static analyzer for Kotlin, Java, and Android.")
            }
            url.set("https://github.com/kaeawc/krit")
            inceptionYear.set("2026")
            licenses {
                license {
                    name.set("MIT License")
                    url.set("https://opensource.org/licenses/MIT")
                    distribution.set("repo")
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

val signingKey: String? = System.getenv("SIGNING_KEY")
val signingPassword: String? = System.getenv("SIGNING_PASSWORD")
val hasSigningKey = !signingKey.isNullOrBlank() && !signingPassword.isNullOrBlank()

signing {
    if (hasSigningKey) {
        useInMemoryPgpKeys(signingKey, signingPassword)
        sign(publishing.publications)
    }
}

// plugin-publish registers sign tasks for every publication whenever the
// signing plugin is applied; skip them for local builds without a key (the
// centralPortal guard below still refuses an unsigned release upload).
tasks.withType<Sign>().configureEach {
    onlyIf { !isSnapshot && hasSigningKey }
}

// The repository is not bound yet when java-gradle-plugin's marker publish
// tasks are configured, so check it when the task runs.
tasks.withType<PublishToMavenRepository>().configureEach {
    doFirst {
        if (repository.name == "centralPortal" && !isSnapshot) {
            val missing = listOf("SIGNING_KEY", "SIGNING_PASSWORD")
                .filter { System.getenv(it).isNullOrBlank() }
            if (missing.isNotEmpty()) {
                throw GradleException("Cannot publish to centralPortal without ${missing.joinToString(" and ")}")
            }
        }
    }
}
