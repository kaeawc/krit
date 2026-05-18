plugins {
    `java-gradle-plugin`
    `kotlin-dsl`
    `maven-publish`
}

group = "dev.jasonpearson.krit"
version = (findProperty("kritVersion") as String?)
    ?.takeIf { it.isNotBlank() }
    ?: System.getenv("KRIT_VERSION")?.takeIf { it.isNotBlank() }
    ?: "0.0.0-SNAPSHOT"

repositories {
    mavenCentral()
}

// Intentionally no `jvmToolchain(...)` — the composite-built dependency on
// `krit-gradle-plugin` matches whichever JVM Gradle itself runs on, and a
// pinned consumer toolchain produces a no-matching-variant error.

gradlePlugin {
    plugins {
        create("kritSettings") {
            id = "dev.jasonpearson.krit.settings"
            implementationClass = "dev.jasonpearson.krit.settings.KritSettingsPlugin"
            displayName = "Krit Settings Plugin"
            description = "Apply Krit static analysis to every Kotlin / Java / " +
                "Android subproject from a single `settings.gradle.kts` block."
        }
    }
}

dependencies {
    // Depend on the host project plugin so the settings plugin can apply it by
    // type and read `KritExtension` directly. Resolved against the composite
    // build declared in `settings.gradle.kts`.
    implementation("dev.jasonpearson.krit:krit-gradle-plugin")

    testImplementation(gradleTestKit())
    testImplementation("org.junit.jupiter:junit-jupiter:6.0.3")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.test {
    useJUnitPlatform()
    // Surface the project-plugin location to TestKit so the functional test's
    // generated consumer build can `includeBuild(...)` the local source.
    systemProperty(
        "krit.gradle.plugin.dir",
        project.layout.projectDirectory.dir("../krit-gradle-plugin").asFile.absolutePath,
    )
}

publishing {
    publications {
        withType<MavenPublication>().configureEach {
            pom {
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
        maven {
            name = "stagingDir"
            url = layout.buildDirectory.dir("staging-deploy").get().asFile.toURI()
        }
    }
}
