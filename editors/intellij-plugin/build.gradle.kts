import org.jetbrains.intellij.platform.gradle.extensions.intellijPlatform

plugins {
    kotlin("jvm") version "2.3.21"
    id("org.jetbrains.intellij.platform") version "2.16.0"
}

group = "dev.jasonpearson.krit"
version = "0.1.0-SNAPSHOT"

repositories {
    mavenCentral()
    intellijPlatform { defaultRepositories() }
}

kotlin {
    jvmToolchain(21)
}

dependencies {
    implementation("com.google.code.gson:gson:2.13.2")

    testImplementation(kotlin("test-junit5"))
    testImplementation("org.junit.jupiter:junit-jupiter-api:6.0.3")
    testRuntimeOnly("org.junit.jupiter:junit-jupiter-engine:6.0.3")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")

    intellijPlatform {
        intellijIdea("2025.3")
        bundledPlugin("com.intellij.java")
        bundledPlugin("org.jetbrains.kotlin")
        pluginVerifier()
    }
}

intellijPlatform {
    pluginConfiguration {
        id.set("dev.jasonpearson.krit")
        name.set("Krit")
        version.set(project.version.toString())
        description.set(
            """
            Live IntelliJ and Android Studio integration for the Krit static analyzer.

            Krit is a Go-first analyzer for Kotlin, Java, and Android projects.
            This plugin renders Krit findings as editor annotations and
            inspections, offers a per-finding fix and a Suppress quick-fix,
            tracks scan state in the status bar, and reads the IDE's resolved
            classpath so type-aware (KAA / FIR) rules match CI behaviour.

            Configure the Krit binary, default fix level, and config file
            override under Settings → Tools → Krit.
            """.trimIndent(),
        )

        ideaVersion {
            sinceBuild.set("253")
            untilBuild.set("261.*")
        }

        vendor {
            name.set("Krit")
            url.set("https://github.com/kaeawc/krit")
        }
    }

    publishing {
        token.set(providers.environmentVariable("JETBRAINS_MARKETPLACE_TOKEN"))
        // "default" is the public stable channel; CI passes "beta" / "eap"
        // via -PpluginChannel for pre-release builds.
        val channel = (findProperty("pluginChannel") as String?) ?: "default"
        channels.set(listOf(channel))
    }

    pluginVerification { ides { recommended() } }
}

tasks.test {
    useJUnitPlatform()
}
