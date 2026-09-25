import org.gradle.api.publish.maven.tasks.PublishToMavenRepository

plugins {
    kotlin("jvm") version "2.3.21"
    id("com.gradleup.shadow") version "9.4.2"
    `maven-publish`
    signing
}

group = "dev.jasonpearson.krit"
version = (findProperty("kritVersion") as String?)
    ?.takeIf { it.isNotBlank() }
    ?: System.getenv("KRIT_VERSION")?.takeIf { it.isNotBlank() }
    ?: "0.0.0-SNAPSHOT"

val isSnapshot = version.toString().endsWith("-SNAPSHOT")

val kotlinVersion = "2.3.21"

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

dependencies {
    add(bundledKotlinStdlib.name, "org.jetbrains.kotlin:kotlin-stdlib:$kotlinVersion") { isTransitive = false }
    implementation(project(":krit-rule-api"))

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
    implementation("com.github.ben-manes.caffeine:caffeine:3.2.4")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-core:1.11.0")
    runtimeOnly("org.jetbrains.intellij.deps.kotlinx:kotlinx-coroutines-core:1.10.2-intellij-1")

    testImplementation(kotlin("test-junit5"))
}

kotlin {
    jvmToolchain(21)
}

tasks.shadowJar {
    archiveClassifier.set("")
    // Keep the launcher path stable while project version drives publication coordinates.
    archiveFileName.set("krit-types.jar")
    // Include Krit's license and the shared Apache-2.0 notices for bundled dependencies.
    from(layout.projectDirectory.file("../../LICENSE")) {
        into("META-INF")
        rename { "LICENSE-krit.txt" }
    }
    from(layout.projectDirectory.file("../THIRD_PARTY_NOTICES.txt")) {
        into("META-INF")
    }
    mergeServiceFiles() // Required: Analysis API uses ServiceLoader extensively
    manifest {
        attributes(
            "Main-Class" to "dev.jasonpearson.krit.types.MainKt",
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

tasks.test {
    useJUnitPlatform()
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
            artifactId = "krit-types"
            artifact(tasks.named("sourcesJar"))
            artifact(tasks.named("javadocJar"))
            pom {
                name.set("Krit type oracle (Kotlin Analysis API)")
                description.set("Krit type oracle (Kotlin Analysis API) — self-contained JVM helper launched by the krit CLI. The fat jar bundles the Apache-2.0 licensed Kotlin compiler and Kotlin Analysis API.")
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
