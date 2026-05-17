plugins {
    kotlin("jvm") version "2.3.21"
    `java-library`
    `maven-publish`
    signing
}

group = "dev.jasonpearson.krit"
// Default falls through to a local-dev SNAPSHOT so `publishToMavenLocal`
// works with no flags. CI always pins a real version via KRIT_VERSION.
version = (findProperty("kritVersion") as String?)
    ?.takeIf { it.isNotBlank() }
    ?: System.getenv("KRIT_VERSION")?.takeIf { it.isNotBlank() }
    ?: "0.0.0-SNAPSHOT"

val isSnapshot = version.toString().endsWith("-SNAPSHOT")

val kotlinVersion = "2.3.21"

dependencies {
    // PSI types are surfaced on the rule API but provided at runtime
    // by the krit-types daemon — keep them off the published rule jar.
    compileOnly("org.jetbrains.kotlin:kotlin-compiler:$kotlinVersion")
}

kotlin {
    jvmToolchain(21)
}

java {
    withSourcesJar()
    withJavadocJar()
}

// Codegen so RuleApiVersion.VERSION tracks `version` automatically rather
// than drifting from a hand-edited constant; the daemon reads it at
// jar-load time to compare against the consumer rule jar's Krit-SDK-Version.
val generateRuleApiVersion = tasks.register("generateRuleApiVersion") {
    val versionString = version.toString()
    val outputDir = layout.buildDirectory.dir("generated/source/ruleApiVersion/main")
    inputs.property("ruleApiVersion", versionString)
    outputs.dir(outputDir)
    doLast {
        val pkgDir = outputDir.get().asFile.resolve("dev/jasonpearson/krit/api")
        pkgDir.mkdirs()
        pkgDir.resolve("RuleApiVersion.kt").writeText(
            """
            |package dev.jasonpearson.krit.api
            |
            |/**
            | * Version of `dev.jasonpearson.krit:krit-rule-api` baked in at build time.
            | *
            | * The Krit daemon compares this against the `Krit-SDK-Version` manifest
            | * attribute stamped by `dev.jasonpearson.krit.custom` on each rule jar to
            | * emit a load-time compatibility diagnostic. See `docs/external-rules.md`.
            | */
            |public object RuleApiVersion {
            |    public const val VERSION: String = "$versionString"
            |}
            |
            """.trimMargin()
        )
    }
}

sourceSets {
    named("main") {
        java.srcDir(generateRuleApiVersion.map { layout.buildDirectory.dir("generated/source/ruleApiVersion/main") })
    }
}

tasks.named<Jar>("jar") {
    val versionString = version.toString()
    manifest {
        attributes(
            "Implementation-Title" to "krit-rule-api",
            "Implementation-Version" to versionString,
            "Implementation-Vendor" to "dev.jasonpearson",
        )
    }
}

publishing {
    publications {
        create<MavenPublication>("maven") {
            from(components["java"])
            artifactId = "krit-rule-api"
            pom {
                name.set("Krit Rule API")
                description.set(
                    "ServiceLoader SPI for authoring Kotlin-based custom Krit static-analysis rules.",
                )
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
    }
    repositories {
        // Local staging dir — handy for building a bundle to ship to the
        // Central Portal in CI, and for offline reproduction of the
        // exact set of files we'd upload.
        maven {
            name = "stagingDir"
            url = layout.buildDirectory.dir("staging-deploy").get().asFile.toURI()
        }
        // Sonatype Central Portal endpoints. Auth comes from env vars
        // SONATYPE_USERNAME / SONATYPE_PASSWORD (user token, not portal
        // login). Empty creds are tolerated so publishToMavenLocal and
        // publishAllPublicationsToStagingDirRepository work offline.
        maven {
            name = "centralPortal"
            url = if (isSnapshot) {
                uri("https://central.sonatype.com/repository/maven-snapshots/")
            } else {
                uri("https://ossrh-staging-api.central.sonatype.com/service/local/staging/deploy/maven2/")
            }
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

// Central Portal requires signed releases; SNAPSHOT publishing is
// unsigned so local + CI snapshot pushes don't need a key.
tasks.withType<Sign>().configureEach {
    onlyIf { !isSnapshot }
}
